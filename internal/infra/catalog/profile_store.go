package catalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"mcpd/internal/domain"
	"mcpd/internal/infra/fsutil"
)

const (
	profilesDirName    = "profiles"
	callersFileName    = "callers.yaml"
	defaultProfileFile = "default.yaml"
	defaultProfileAlt  = "default.yml"
	runtimeFileName    = "runtime.yaml"
	runtimeFileAlt     = "runtime.yml"
)

type ProfileStoreOptions struct {
	AllowCreate bool
}

type ProfileStoreLoader struct {
	logger *zap.Logger
	loader *Loader
}

func NewProfileStoreLoader(logger *zap.Logger) *ProfileStoreLoader {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ProfileStoreLoader{
		logger: logger.Named("profiles"),
		loader: NewLoader(logger),
	}
}

func (l *ProfileStoreLoader) Load(ctx context.Context, path string, opts ProfileStoreOptions) (domain.ProfileStore, error) {
	if path == "" {
		return domain.ProfileStore{}, errors.New("profile store path is required")
	}

	info, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return domain.ProfileStore{}, fmt.Errorf("stat profile store: %w", err)
		}
		if isYAMLPath(path) {
			return domain.ProfileStore{}, fmt.Errorf("profile store path must be a directory: %s", path)
		}
		if !opts.AllowCreate {
			return domain.ProfileStore{}, fmt.Errorf("profile store not found: %s", path)
		}
		if err := os.MkdirAll(path, fsutil.DefaultDirMode); err != nil {
			return domain.ProfileStore{}, fmt.Errorf("create profile store: %w", err)
		}
		info, err = os.Stat(path)
		if err != nil {
			return domain.ProfileStore{}, fmt.Errorf("stat profile store: %w", err)
		}
	}

	if !info.IsDir() {
		return domain.ProfileStore{}, fmt.Errorf("profile store path must be a directory: %s", path)
	}
	return l.loadFromDir(ctx, path, opts)
}

func (l *ProfileStoreLoader) loadFromDir(ctx context.Context, path string, opts ProfileStoreOptions) (domain.ProfileStore, error) {
	profilesDir := filepath.Join(path, profilesDirName)
	if err := ensureDir(profilesDir, opts.AllowCreate); err != nil {
		return domain.ProfileStore{}, err
	}

	callersPath := filepath.Join(path, callersFileName)
	if err := ensureCallersFile(callersPath, opts.AllowCreate); err != nil {
		return domain.ProfileStore{}, err
	}

	runtimeConfig, err := l.loadRuntimeConfig(ctx, path)
	if err != nil {
		return domain.ProfileStore{}, err
	}

	defaultProfilePath := filepath.Join(profilesDir, defaultProfileFile)
	defaultAltPath := filepath.Join(profilesDir, defaultProfileAlt)
	createdDefault, err := ensureDefaultProfile(defaultProfilePath, defaultAltPath, opts.AllowCreate)
	if err != nil {
		return domain.ProfileStore{}, err
	}

	profiles, err := l.loadProfiles(ctx, profilesDir, runtimeConfig)
	if err != nil {
		if createdDefault {
			return domain.ProfileStore{}, fmt.Errorf("default profile created at %s; update it with servers to continue: %w", defaultProfilePath, err)
		}
		return domain.ProfileStore{}, err
	}

	if _, ok := profiles[domain.DefaultProfileName]; !ok {
		return domain.ProfileStore{}, fmt.Errorf("default profile %q not found in %s", domain.DefaultProfileName, profilesDir)
	}

	callers, err := loadCallers(callersPath)
	if err != nil {
		return domain.ProfileStore{}, err
	}
	if err := validateCallers(callers, profiles); err != nil {
		return domain.ProfileStore{}, err
	}

	return domain.ProfileStore{
		Profiles: profiles,
		Callers:  callers,
	}, nil
}

func (l *ProfileStoreLoader) loadRuntimeConfig(ctx context.Context, path string) (*domain.RuntimeConfig, error) {
	runtimePath := filepath.Join(path, runtimeFileName)
	altPath := filepath.Join(path, runtimeFileAlt)

	if _, err := os.Stat(runtimePath); err != nil {
		if os.IsNotExist(err) {
			if _, altErr := os.Stat(altPath); altErr != nil {
				if os.IsNotExist(altErr) {
					return nil, nil
				}
				return nil, fmt.Errorf("stat %s: %w", altPath, altErr)
			}
			runtimePath = altPath
		} else {
			return nil, fmt.Errorf("stat %s: %w", runtimePath, err)
		}
	}

	cfg, err := l.loader.LoadRuntimeConfig(ctx, runtimePath)
	if err != nil {
		return nil, fmt.Errorf("load runtime config: %w", err)
	}
	return &cfg, nil
}

func (l *ProfileStoreLoader) loadProfiles(ctx context.Context, dir string, runtimeOverride *domain.RuntimeConfig) (map[string]domain.Profile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read profiles dir: %w", err)
	}

	profiles := make(map[string]domain.Profile)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		profileName := strings.TrimSuffix(name, ext)
		if profileName == "" {
			continue
		}
		if _, exists := profiles[profileName]; exists {
			return nil, fmt.Errorf("duplicate profile %q", profileName)
		}
		path := filepath.Join(dir, name)

		catalogData, err := l.loader.Load(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("load profile %q: %w", profileName, err)
		}
		if runtimeOverride != nil {
			catalogData.Runtime = *runtimeOverride
		}
		profiles[profileName] = domain.Profile{
			Name:    profileName,
			Catalog: catalogData,
		}
	}

	if len(profiles) == 0 {
		return nil, errors.New("no profiles found")
	}
	return profiles, nil
}

func ensureDir(path string, allowCreate bool) error {
	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", path)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if !allowCreate {
		return fmt.Errorf("missing directory %s", path)
	}
	if err := os.MkdirAll(path, fsutil.DefaultDirMode); err != nil {
		return fmt.Errorf("create directory %s: %w", path, err)
	}
	return nil
}

func ensureCallersFile(path string, allowCreate bool) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	if !allowCreate {
		return nil
	}
	if err := os.WriteFile(path, []byte("callers: {}\n"), fsutil.DefaultFileMode); err != nil {
		return fmt.Errorf("write callers file: %w", err)
	}
	return nil
}

func isYAMLPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

func ensureDefaultProfile(path string, altPath string, allowCreate bool) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat %s: %w", path, err)
	}
	if _, err := os.Stat(altPath); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat %s: %w", altPath, err)
	}
	if !allowCreate {
		return false, nil
	}
	if err := os.WriteFile(path, []byte("servers: []\n"), fsutil.DefaultFileMode); err != nil {
		return false, fmt.Errorf("write default profile: %w", err)
	}
	return true, nil
}

type rawCallers struct {
	Callers map[string]string `yaml:"callers"`
}

func loadCallers(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("read callers file: %w", err)
	}

	var raw rawCallers
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse callers file: %w", err)
	}

	callers := make(map[string]string, len(raw.Callers))
	for caller, profile := range raw.Callers {
		caller = strings.TrimSpace(caller)
		profile = strings.TrimSpace(profile)
		if caller == "" || profile == "" {
			return nil, errors.New("callers file contains empty caller or profile name")
		}
		callers[caller] = profile
	}
	return callers, nil
}

func validateCallers(callers map[string]string, profiles map[string]domain.Profile) error {
	for caller, profile := range callers {
		if _, ok := profiles[profile]; !ok {
			return fmt.Errorf("caller %q references unknown profile %q", caller, profile)
		}
	}
	return nil
}
