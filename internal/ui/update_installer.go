package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"mcpv/internal/ui/types"
)

const (
	updateInstallStatusIdle       = "idle"
	updateInstallStatusPreparing  = "preparing"
	updateInstallStatusExtracting = "extracting"
	updateInstallStatusValidating = "validating"
	updateInstallStatusReplacing  = "replacing"
	updateInstallStatusCleaning   = "cleaning"
	updateInstallStatusRestarting = "restarting"
	updateInstallStatusCompleted  = "completed"
	updateInstallStatusFailed     = "failed"
)

const (
	updateInstallTimeout     = 30 * time.Minute
	updateInstallBackupDelay = 24 * time.Hour
	updateAppExecutableName  = "mcpvui"
)

type UpdateInstaller struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	progress types.UpdateInstallProgress
	inFlight bool
	shutdown func()
	exit     func(code int)
}

func NewUpdateInstaller(logger *zap.Logger, shutdown func()) *UpdateInstaller {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &UpdateInstaller{
		logger: logger.Named("update-installer"),
		progress: types.UpdateInstallProgress{
			Status: updateInstallStatusIdle,
		},
		shutdown: shutdown,
		exit:     os.Exit,
	}
}

func (i *UpdateInstaller) Progress() types.UpdateInstallProgress {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.progress
}

func (i *UpdateInstaller) Start(_ context.Context, req types.UpdateInstallRequest) (types.UpdateInstallProgress, error) {
	if i == nil {
		return types.UpdateInstallProgress{}, errors.New("update installer not initialized")
	}
	filePath := strings.TrimSpace(req.FilePath)
	if filePath == "" {
		return types.UpdateInstallProgress{}, errors.New("file path is required")
	}

	i.mu.Lock()
	if i.inFlight {
		progress := i.progress
		i.mu.Unlock()
		return progress, errors.New("install already in progress")
	}
	i.inFlight = true
	i.progress = types.UpdateInstallProgress{
		Status:   updateInstallStatusPreparing,
		Percent:  5,
		FilePath: filePath,
	}
	i.mu.Unlock()

	go i.run(filePath)

	return i.Progress(), nil
}

func (i *UpdateInstaller) run(filePath string) {
	ctx, cancel := context.WithTimeout(context.Background(), updateInstallTimeout)
	defer cancel()

	if err := i.install(ctx, filePath); err != nil {
		i.fail(err)
		return
	}

	i.setProgress(func(progress *types.UpdateInstallProgress) {
		progress.Status = updateInstallStatusCompleted
		progress.Percent = 100
	})

	i.markDone()
}

func (i *UpdateInstaller) install(ctx context.Context, filePath string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to access update file: %w", err)
	}
	if info.IsDir() {
		return errors.New("update file must be a DMG or ZIP")
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	sourceRoot := ""
	var cleanup func()

	switch ext {
	case ".dmg":
		i.setProgress(func(progress *types.UpdateInstallProgress) {
			progress.Status = updateInstallStatusExtracting
			progress.Percent = 15
		})
		mountPoint, detach, err := mountDMG(ctx, filePath)
		if err != nil {
			return err
		}
		sourceRoot = mountPoint
		cleanup = detach
	case ".zip":
		i.setProgress(func(progress *types.UpdateInstallProgress) {
			progress.Status = updateInstallStatusExtracting
			progress.Percent = 15
		})
		extractRoot, cleanupZip, err := extractZip(ctx, filePath)
		if err != nil {
			return err
		}
		sourceRoot = extractRoot
		cleanup = cleanupZip
	default:
		return fmt.Errorf("unsupported update file: %s", ext)
	}
	if cleanup != nil {
		defer cleanup()
	}

	i.setProgress(func(progress *types.UpdateInstallProgress) {
		progress.Status = updateInstallStatusValidating
		progress.Percent = 30
	})

	appPath, err := findAppBundle(sourceRoot, updateAppExecutableName)
	if err != nil {
		return err
	}

	currentAppPath, err := findCurrentAppBundle()
	if err != nil {
		return err
	}

	i.setProgress(func(progress *types.UpdateInstallProgress) {
		progress.Status = updateInstallStatusReplacing
		progress.Percent = 45
		progress.AppPath = currentAppPath
	})

	backupPath := fmt.Sprintf("%s.backup-%s", currentAppPath, time.Now().UTC().Format("20060102-150405"))
	if err := os.Rename(currentAppPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup app: %w", err)
	}

	i.setProgress(func(progress *types.UpdateInstallProgress) {
		progress.BackupPath = backupPath
		progress.Percent = 55
	})

	if err := copyAppBundle(appPath, currentAppPath); err != nil {
		_ = os.RemoveAll(currentAppPath)
		if rollbackErr := os.Rename(backupPath, currentAppPath); rollbackErr != nil {
			return fmt.Errorf("install failed: %w (rollback failed: %v)", err, rollbackErr)
		}
		return fmt.Errorf("install failed: %w", err)
	}

	i.setProgress(func(progress *types.UpdateInstallProgress) {
		progress.Percent = 70
	})

	if err := removeQuarantine(currentAppPath); err != nil {
		i.logger.Warn("failed to remove quarantine attribute", zap.String("path", currentAppPath), zap.Error(err))
	}

	if err := removeQuarantine(appPath); err != nil {
		i.logger.Warn("failed to remove quarantine attribute on source", zap.String("path", appPath), zap.Error(err))
	}

	i.setProgress(func(progress *types.UpdateInstallProgress) {
		progress.Status = updateInstallStatusCleaning
		progress.Percent = 80
	})

	if err := scheduleBackupCleanup(backupPath, updateInstallBackupDelay); err != nil {
		i.logger.Warn("failed to schedule backup cleanup", zap.String("path", backupPath), zap.Error(err))
	}

	i.setProgress(func(progress *types.UpdateInstallProgress) {
		progress.Status = updateInstallStatusRestarting
		progress.Percent = 95
	})

	if err := launchRestartHelper(currentAppPath); err != nil {
		return fmt.Errorf("restart helper failed: %w", err)
	}

	if i.shutdown != nil {
		i.shutdown()
	}

	i.exit(0)
	return nil
}

func (i *UpdateInstaller) markDone() {
	i.mu.Lock()
	i.inFlight = false
	i.mu.Unlock()
}

func (i *UpdateInstaller) fail(err error) {
	i.setProgress(func(progress *types.UpdateInstallProgress) {
		progress.Status = updateInstallStatusFailed
		progress.Message = err.Error()
	})
	i.markDone()
}

func (i *UpdateInstaller) setProgress(update func(progress *types.UpdateInstallProgress)) {
	i.mu.Lock()
	progress := i.progress
	update(&progress)
	i.progress = progress
	i.mu.Unlock()
}

func mountDMG(ctx context.Context, dmgPath string) (string, func(), error) {
	mountPoint, err := os.MkdirTemp(os.TempDir(), "mcpv-update-")
	if err != nil {
		return "", nil, err
	}

	cmd := exec.CommandContext(ctx, "hdiutil", "attach", "-mountpoint", mountPoint, "-nobrowse", "-readonly", dmgPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(mountPoint)
		return "", nil, fmt.Errorf("hdiutil attach failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	detach := func() {
		_ = exec.CommandContext(context.Background(), "hdiutil", "detach", "-force", mountPoint).Run()
		_ = os.RemoveAll(mountPoint)
	}

	return mountPoint, detach, nil
}

func extractZip(ctx context.Context, zipPath string) (string, func(), error) {
	extractRoot, err := os.MkdirTemp(os.TempDir(), "mcpv-extract-")
	if err != nil {
		return "", nil, err
	}

	cmd := exec.CommandContext(ctx, "ditto", "-x", "-k", zipPath, extractRoot)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(extractRoot)
		return "", nil, fmt.Errorf("zip extract failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	cleanup := func() {
		_ = os.RemoveAll(extractRoot)
	}

	return extractRoot, cleanup, nil
}

func findAppBundle(root, executable string) (string, error) {
	candidates := make([]string, 0, 4)
	candidates = append(candidates, root)

	entries, err := os.ReadDir(root)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() && !strings.HasSuffix(entry.Name(), ".app") {
				candidates = append(candidates, filepath.Join(root, entry.Name()))
			}
		}
	}

	for _, base := range candidates {
		items, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, item := range items {
			if !item.IsDir() || !strings.HasSuffix(item.Name(), ".app") {
				continue
			}
			path := filepath.Join(base, item.Name())
			if validateAppBundle(path, executable) {
				return path, nil
			}
		}
	}

	return "", errors.New("no valid app bundle found")
}

func validateAppBundle(appPath, executable string) bool {
	binaryPath := filepath.Join(appPath, "Contents", "MacOS", executable)
	info, err := os.Stat(binaryPath)
	return err == nil && !info.IsDir()
}

func findCurrentAppBundle() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return "", err
	}

	dir := filepath.Dir(exePath)
	for {
		if strings.HasSuffix(dir, ".app") {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("unable to locate current app bundle")
}

func copyAppBundle(source, target string) error {
	cmd := exec.CommandContext(context.Background(), "ditto", source, target)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("copy failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func removeQuarantine(path string) error {
	cmd := exec.CommandContext(context.Background(), "xattr", "-rd", "com.apple.quarantine", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("xattr failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func scheduleBackupCleanup(backupPath string, delay time.Duration) error {
	scriptPath, err := writeCleanupScript(backupPath, delay)
	if err != nil {
		return err
	}
	return launchHelperScript(scriptPath)
}

func writeCleanupScript(backupPath string, delay time.Duration) (string, error) {
	seconds := int(delay.Seconds())
	content := fmt.Sprintf("#!/bin/bash\nsleep %d\nrm -rf %s\nrm -f \"$0\"\n", seconds, shellQuote(backupPath))
	return writeTempScript("mcpv-cleanup-", content)
}

func launchRestartHelper(appPath string) error {
	content := fmt.Sprintf("#!/bin/bash\nsleep 2\nopen %s\nrm -f \"$0\"\n", shellQuote(appPath))
	scriptPath, err := writeTempScript("mcpv-restart-", content)
	if err != nil {
		return err
	}
	return launchHelperScript(scriptPath)
}

func writeTempScript(prefix, content string) (string, error) {
	file, err := os.CreateTemp(os.TempDir(), prefix+"*.sh")
	if err != nil {
		return "", err
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	if err := os.Chmod(file.Name(), 0o755); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

func launchHelperScript(scriptPath string) error {
	cmd := exec.CommandContext(context.Background(), "sh", "-c", fmt.Sprintf("nohup %s >/dev/null 2>&1 &", shellQuote(scriptPath)))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("helper launch failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.Contains(value, "'") {
		return "'" + value + "'"
	}
	return "'" + strings.ReplaceAll(value, "'", `'\"'\"'`) + "'"
}
