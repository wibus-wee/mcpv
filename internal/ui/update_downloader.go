package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"mcpv/internal/ui/types"
)

const (
	updateDownloadStatusIdle        = "idle"
	updateDownloadStatusResolving   = "resolving"
	updateDownloadStatusDownloading = "downloading"
	updateDownloadStatusCompleted   = "completed"
	updateDownloadStatusFailed      = "failed"
)

const updateDownloadTimeout = 30 * time.Minute

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type githubReleaseWithAssets struct {
	TagName string               `json:"tag_name"`
	Assets  []githubReleaseAsset `json:"assets"`
}

type UpdateDownloader struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	client   *http.Client
	progress types.UpdateDownloadProgress
	inFlight bool
	cancel   context.CancelFunc
}

func NewUpdateDownloader(logger *zap.Logger) *UpdateDownloader {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &UpdateDownloader{
		logger: logger.Named("update-downloader"),
		client: &http.Client{},
		progress: types.UpdateDownloadProgress{
			Status: updateDownloadStatusIdle,
		},
	}
}

func (d *UpdateDownloader) Progress() types.UpdateDownloadProgress {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.progress
}

func (d *UpdateDownloader) Start(_ context.Context, req types.UpdateDownloadRequest) (types.UpdateDownloadProgress, error) {
	if d == nil {
		return types.UpdateDownloadProgress{}, errors.New("update downloader not initialized")
	}

	releaseURL := strings.TrimSpace(req.ReleaseURL)
	if releaseURL == "" {
		return types.UpdateDownloadProgress{}, errors.New("release url is required")
	}

	d.mu.Lock()
	if d.inFlight {
		progress := d.progress
		d.mu.Unlock()
		return progress, errors.New("download already in progress")
	}
	d.inFlight = true
	d.progress = types.UpdateDownloadProgress{
		Status: updateDownloadStatusResolving,
	}
	d.mu.Unlock()

	go d.run(releaseURL, req.AssetName)

	return d.Progress(), nil
}

func (d *UpdateDownloader) run(releaseURL, assetName string) {
	ctx, cancel := context.WithTimeout(context.Background(), updateDownloadTimeout)
	d.setCancel(cancel)
	defer func() {
		cancel()
		d.clearCancel()
	}()

	assetURL, assetFileName, assetSize, err := d.resolveAsset(ctx, releaseURL, assetName)
	if err != nil {
		d.fail(err)
		return
	}

	d.setProgress(func(progress *types.UpdateDownloadProgress) {
		progress.Status = updateDownloadStatusDownloading
		progress.FileName = assetFileName
		progress.Total = assetSize
		progress.Percent = 0
		progress.Message = ""
	})

	filePath, err := d.downloadFile(ctx, assetURL, assetFileName)
	if err != nil {
		d.fail(err)
		return
	}

	d.setProgress(func(progress *types.UpdateDownloadProgress) {
		progress.Status = updateDownloadStatusCompleted
		progress.FilePath = filePath
		progress.Message = ""
		if progress.Total == 0 {
			progress.Total = progress.Bytes
		}
		if progress.Total > 0 {
			progress.Percent = 100
		}
	})

	d.markDone()
}

func (d *UpdateDownloader) downloadFile(ctx context.Context, assetURL, assetFileName string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent())

	resp, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	if resp.ContentLength > 0 {
		d.setProgress(func(progress *types.UpdateDownloadProgress) {
			progress.Total = resp.ContentLength
		})
	}

	filePath, file, err := createTempUpdateFile(assetFileName)
	if err != nil {
		return "", err
	}
	defer file.Close()

	reader := &progressReader{
		reader: resp.Body,
		onRead: d.addBytes,
	}

	if _, err := io.CopyBuffer(file, reader, make([]byte, 32*1024)); err != nil {
		_ = os.Remove(filePath)
		return "", err
	}

	return filePath, nil
}

func (d *UpdateDownloader) resolveAsset(ctx context.Context, releaseURL, assetName string) (string, string, int64, error) {
	if assetURL, name, ok := resolveDirectAssetURL(releaseURL); ok {
		return assetURL, name, 0, nil
	}

	owner, repo, tag, latest, ok := parseGitHubReleaseURL(releaseURL)
	if !ok {
		return "", "", 0, errors.New("unsupported release url")
	}

	endpoint := ""
	if latest {
		endpoint = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	} else {
		endpoint = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", owner, repo, tag)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", "", 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent())

	resp, err := d.client.Do(req)
	if err != nil {
		return "", "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", "", 0, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var release githubReleaseWithAssets
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", 0, err
	}

	asset, ok := selectReleaseAsset(release.Assets, assetName, runtime.GOARCH)
	if !ok {
		return "", "", 0, errors.New("no suitable asset found")
	}

	return asset.BrowserDownloadURL, asset.Name, asset.Size, nil
}

func (d *UpdateDownloader) addBytes(read int64) {
	if read <= 0 {
		return
	}
	d.setProgress(func(progress *types.UpdateDownloadProgress) {
		progress.Bytes += read
		if progress.Total > 0 {
			percent := (float64(progress.Bytes) / float64(progress.Total)) * 100
			progress.Percent = math.Min(100, percent)
		}
	})
}

func (d *UpdateDownloader) setCancel(cancel context.CancelFunc) {
	d.mu.Lock()
	d.cancel = cancel
	d.mu.Unlock()
}

func (d *UpdateDownloader) clearCancel() {
	d.mu.Lock()
	d.cancel = nil
	d.mu.Unlock()
}

func (d *UpdateDownloader) markDone() {
	d.mu.Lock()
	d.inFlight = false
	d.mu.Unlock()
}

func (d *UpdateDownloader) fail(err error) {
	d.setProgress(func(progress *types.UpdateDownloadProgress) {
		progress.Status = updateDownloadStatusFailed
		progress.Message = err.Error()
	})
	d.markDone()
}

func (d *UpdateDownloader) setProgress(update func(progress *types.UpdateDownloadProgress)) {
	d.mu.Lock()
	progress := d.progress
	update(&progress)
	d.progress = progress
	d.mu.Unlock()
}

type progressReader struct {
	reader io.Reader
	onRead func(read int64)
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.reader.Read(buf)
	if n > 0 && p.onRead != nil {
		p.onRead(int64(n))
	}
	return n, err
}

func createTempUpdateFile(assetName string) (string, *os.File, error) {
	ext := filepath.Ext(assetName)
	base := strings.TrimSuffix(assetName, ext)
	if base == "" {
		base = "mcpv-update"
	}
	pattern := fmt.Sprintf("%s-*%s", base, ext)
	return createTempFileWithPattern(pattern)
}

func createTempFileWithPattern(pattern string) (string, *os.File, error) {
	file, err := os.CreateTemp(os.TempDir(), pattern)
	if err != nil {
		return "", nil, err
	}
	return file.Name(), file, nil
}

func resolveDirectAssetURL(raw string) (string, string, bool) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" {
		return "", "", false
	}
	name := path.Base(parsed.Path)
	if name == "" || name == "." || name == "/" {
		return "", "", false
	}
	if !isSupportedAssetName(name) {
		return "", "", false
	}
	return raw, name, true
}

func isSupportedAssetName(name string) bool {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "universal") {
		return false
	}
	return strings.HasSuffix(lower, ".dmg") || strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".pkg")
}

func selectReleaseAsset(assets []githubReleaseAsset, preferredName string, arch string) (githubReleaseAsset, bool) {
	if len(assets) == 0 {
		return githubReleaseAsset{}, false
	}
	preferredName = strings.TrimSpace(preferredName)
	if preferredName != "" {
		for _, asset := range assets {
			if asset.Name != preferredName {
				continue
			}
			if isSupportedAssetName(asset.Name) {
				return asset, true
			}
			break
		}
	}

	targetArch := normalizeArchLabel(arch)
	if targetArch != "" {
		if asset, ok := selectAssetByArch(assets, targetArch); ok {
			return asset, true
		}
	}

	if asset, ok := selectNonUniversalAsset(assets); ok {
		return asset, true
	}

	return githubReleaseAsset{}, false
}

func parseGitHubReleaseURL(raw string) (owner, repo, tag string, latest bool, ok bool) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", "", "", false, false
	}
	if parsed.Host != "github.com" && parsed.Host != "www.github.com" {
		return "", "", "", false, false
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 4 {
		return "", "", "", false, false
	}
	owner = parts[0]
	repo = parts[1]
	if parts[2] != "releases" {
		return "", "", "", false, false
	}
	if parts[3] == "latest" {
		return owner, repo, "", true, true
	}
	if parts[3] == "tag" && len(parts) >= 5 {
		tag = parts[4]
		return owner, repo, tag, false, true
	}
	return "", "", "", false, false
}

func normalizeArchLabel(arch string) string {
	switch strings.ToLower(strings.TrimSpace(arch)) {
	case "arm64":
		return "arm64"
	case "amd64":
		return "amd64"
	default:
		return ""
	}
}

func selectAssetByArch(assets []githubReleaseAsset, arch string) (githubReleaseAsset, bool) {
	extOrder := []string{".dmg", ".zip", ".pkg"}
	for _, ext := range extOrder {
		for _, asset := range assets {
			name := strings.ToLower(asset.Name)
			if !isSupportedAssetName(name) {
				continue
			}
			if !strings.Contains(name, "macos-"+arch) {
				continue
			}
			if strings.HasSuffix(name, ext) {
				return asset, true
			}
		}
	}
	return githubReleaseAsset{}, false
}

func selectNonUniversalAsset(assets []githubReleaseAsset) (githubReleaseAsset, bool) {
	extOrder := []string{".dmg", ".zip", ".pkg"}
	for _, ext := range extOrder {
		for _, asset := range assets {
			name := strings.ToLower(asset.Name)
			if !isSupportedAssetName(name) {
				continue
			}
			if strings.HasSuffix(name, ext) {
				return asset, true
			}
		}
	}
	return githubReleaseAsset{}, false
}
