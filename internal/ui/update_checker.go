package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/zap"
	"golang.org/x/mod/semver"

	"mcpv/internal/buildinfo"
	"mcpv/internal/ui/events"
	"mcpv/internal/ui/types"
)

const (
	defaultUpdateInterval = 24 * time.Hour
	updateRequestTimeout  = 10 * time.Second
)

const (
	githubRepoOwner = "wibus-wee"
	githubRepoName  = "mcpd"
)

type githubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	HTMLURL     string `json:"html_url"`
	Prerelease  bool   `json:"prerelease"`
	Draft       bool   `json:"draft"`
	PublishedAt string `json:"published_at"`
}

// UpdateChecker periodically checks GitHub releases and emits update events.
type UpdateChecker struct {
	mu sync.RWMutex

	logger *zap.Logger
	wails  *application.App
	client *http.Client

	options types.UpdateCheckOptions

	ticker *time.Ticker
	stop   chan struct{}
	done   chan struct{}

	etag           string
	cachedRelease  *githubRelease
	lastNotified   string
	lastCheckedAt  time.Time
	checkInFlight  bool
	checkInFlightC *sync.Cond
}

// NewUpdateChecker creates a new UpdateChecker with options.
func NewUpdateChecker(logger *zap.Logger, opts types.UpdateCheckOptions) *UpdateChecker {
	if logger == nil {
		logger = zap.NewNop()
	}
	checker := &UpdateChecker{
		logger:  logger.Named("update-checker"),
		client:  &http.Client{Timeout: updateRequestTimeout},
		options: normalizeUpdateOptions(opts),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	checker.checkInFlightC = sync.NewCond(&checker.mu)
	return checker
}

// SetWailsApp updates the Wails application reference.
func (c *UpdateChecker) SetWailsApp(wails *application.App) {
	c.mu.Lock()
	c.wails = wails
	c.mu.Unlock()
}

// Options returns the current update checker options.
func (c *UpdateChecker) Options() types.UpdateCheckOptions {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.options
}

// SetOptions updates the checker options and returns the normalized value.
func (c *UpdateChecker) SetOptions(opts types.UpdateCheckOptions) types.UpdateCheckOptions {
	normalized := normalizeUpdateOptions(opts)

	c.mu.Lock()
	previous := c.options
	c.options = normalized
	if previous.IncludePrerelease != normalized.IncludePrerelease {
		c.etag = ""
		c.cachedRelease = nil
		c.lastNotified = ""
	}
	intervalChanged := previous.IntervalHours != normalized.IntervalHours
	ticker := c.ticker
	c.mu.Unlock()

	if intervalChanged && ticker != nil {
		c.restartTicker(normalized)
	}

	return normalized
}

// Start begins the periodic update check loop.
func (c *UpdateChecker) Start() {
	c.mu.Lock()
	if c.ticker != nil {
		c.mu.Unlock()
		return
	}
	interval := updateInterval(c.options)
	c.ticker = time.NewTicker(interval)
	c.stop = make(chan struct{})
	c.done = make(chan struct{})
	c.mu.Unlock()

	go c.run()
}

// Stop halts the update checker loop.
func (c *UpdateChecker) Stop() {
	c.mu.Lock()
	ticker := c.ticker
	if ticker == nil {
		c.mu.Unlock()
		return
	}
	stop := c.stop
	done := c.done
	c.ticker = nil
	c.stop = nil
	c.done = nil
	c.mu.Unlock()

	ticker.Stop()
	close(stop)
	<-done
}

// CheckNow performs an immediate update check.
func (c *UpdateChecker) CheckNow(ctx context.Context) (types.UpdateCheckResult, error) {
	return c.checkOnce(ctx, true)
}

func (c *UpdateChecker) run() {
	defer func() {
		c.mu.Lock()
		done := c.done
		c.mu.Unlock()
		if done != nil {
			close(done)
		}
	}()

	if _, err := c.checkOnce(context.Background(), true); err != nil {
		c.logger.Warn("initial update check failed", zap.Error(err))
	}

	for {
		c.mu.RLock()
		ticker := c.ticker
		stop := c.stop
		c.mu.RUnlock()

		if ticker == nil || stop == nil {
			return
		}

		select {
		case <-ticker.C:
			if _, err := c.checkOnce(context.Background(), true); err != nil {
				c.logger.Warn("periodic update check failed", zap.Error(err))
			}
		case <-stop:
			return
		}
	}
}

func (c *UpdateChecker) restartTicker(opts types.UpdateCheckOptions) {
	c.mu.Lock()
	ticker := c.ticker
	stop := c.stop
	c.ticker = nil
	c.stop = nil
	c.options = opts
	c.mu.Unlock()

	if ticker != nil {
		ticker.Stop()
	}
	if stop != nil {
		close(stop)
	}

	c.Start()
}

func (c *UpdateChecker) checkOnce(ctx context.Context, notify bool) (types.UpdateCheckResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	ctx, cancel := context.WithTimeout(ctx, updateRequestTimeout)
	defer cancel()

	currentVersion := strings.TrimSpace(buildinfo.Version)
	if isDevelopmentBuild() {
		return types.UpdateCheckResult{
			CurrentVersion:  currentVersion,
			UpdateAvailable: false,
		}, nil
	}
	if !isVersionComparable(currentVersion) {
		return types.UpdateCheckResult{
			CurrentVersion:  currentVersion,
			UpdateAvailable: false,
		}, nil
	}

	c.mu.Lock()
	for c.checkInFlight {
		c.checkInFlightC.Wait()
	}
	c.checkInFlight = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.checkInFlight = false
		c.checkInFlightC.Broadcast()
		c.mu.Unlock()
	}()

	opts := c.Options()

	latest, err := c.fetchLatestRelease(ctx, opts.IncludePrerelease)
	if err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			c.logger.Warn("update check failed", zap.Error(err))
		}
		return types.UpdateCheckResult{CurrentVersion: currentVersion}, err
	}
	if latest.TagName == "" || latest.HTMLURL == "" {
		return types.UpdateCheckResult{CurrentVersion: currentVersion}, nil
	}

	updateAvailable, compareErr := isUpdateAvailable(currentVersion, latest.TagName)
	if compareErr != nil {
		c.logger.Debug("version compare failed", zap.String("current", currentVersion), zap.String("latest", latest.TagName), zap.Error(compareErr))
		return types.UpdateCheckResult{CurrentVersion: currentVersion}, nil
	}

	release := types.UpdateRelease{
		Version:     latest.TagName,
		Name:        latest.Name,
		URL:         latest.HTMLURL,
		PublishedAt: latest.PublishedAt,
		Prerelease:  latest.Prerelease,
	}

	result := types.UpdateCheckResult{
		CurrentVersion:  currentVersion,
		UpdateAvailable: updateAvailable,
	}
	if updateAvailable {
		result.Latest = &release
	}

	if updateAvailable && notify {
		c.mu.Lock()
		alreadyNotified := c.lastNotified == latest.TagName
		if !alreadyNotified {
			c.lastNotified = latest.TagName
		}
		c.lastCheckedAt = time.Now()
		wails := c.wails
		c.mu.Unlock()

		if !alreadyNotified {
			events.EmitUpdateAvailable(wails, events.UpdateAvailableEvent{
				CurrentVersion: currentVersion,
				Latest:         release,
			})
		}
	}

	return result, nil
}

func (c *UpdateChecker) fetchLatestRelease(ctx context.Context, includePrerelease bool) (githubRelease, error) {
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=10", githubRepoOwner, githubRepoName)

	c.mu.RLock()
	etag := c.etag
	c.mu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return githubRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent())
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return githubRelease{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		c.mu.RLock()
		cached := c.cachedRelease
		c.mu.RUnlock()
		if cached != nil {
			return *cached, nil
		}
		return githubRelease{}, errors.New("update cache miss for 304 response")
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return githubRelease{}, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return githubRelease{}, err
	}

	selected, ok := selectRelease(releases, includePrerelease)
	if !ok {
		return githubRelease{}, nil
	}

	c.mu.Lock()
	c.etag = resp.Header.Get("ETag")
	c.cachedRelease = &selected
	c.mu.Unlock()

	return selected, nil
}

func selectRelease(releases []githubRelease, includePrerelease bool) (githubRelease, bool) {
	for _, release := range releases {
		if release.Draft {
			continue
		}
		if !includePrerelease && release.Prerelease {
			continue
		}
		return release, true
	}
	return githubRelease{}, false
}

func isUpdateAvailable(current, latest string) (bool, error) {
	currentSemver, ok := normalizeSemver(current)
	if !ok {
		return false, fmt.Errorf("invalid current version: %s", current)
	}
	latestSemver, ok := normalizeSemver(latest)
	if !ok {
		return false, fmt.Errorf("invalid latest version: %s", latest)
	}
	return semver.Compare(latestSemver, currentSemver) > 0, nil
}

func normalizeSemver(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", false
	}
	if !strings.HasPrefix(value, "v") {
		value = "v" + value
	}
	normalized := semver.Canonical(value)
	if normalized == "" {
		return "", false
	}
	return normalized, true
}

func isVersionComparable(raw string) bool {
	value := strings.TrimSpace(raw)
	if value == "" || value == "dev" {
		return false
	}
	_, ok := normalizeSemver(value)
	return ok
}

func isDevelopmentBuild() bool {
	version := strings.TrimSpace(buildinfo.Version)
	if version == "" || version == "dev" {
		return true
	}
	build := strings.TrimSpace(buildinfo.Build)
	return build == "" || build == "unknown"
}

func userAgent() string {
	version := strings.TrimSpace(buildinfo.Version)
	if version == "" {
		version = "dev"
	}
	return fmt.Sprintf("mcpv/%s", version)
}

func normalizeUpdateOptions(opts types.UpdateCheckOptions) types.UpdateCheckOptions {
	if opts.IntervalHours <= 0 {
		opts.IntervalHours = int(defaultUpdateInterval / time.Hour)
	}
	return opts
}

func updateInterval(opts types.UpdateCheckOptions) time.Duration {
	if opts.IntervalHours <= 0 {
		return defaultUpdateInterval
	}
	return time.Duration(opts.IntervalHours) * time.Hour
}
