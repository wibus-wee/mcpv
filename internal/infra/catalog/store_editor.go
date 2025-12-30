package catalog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"mcpd/internal/domain"
	"mcpd/internal/infra/fsutil"
)

func CreateProfile(storePath string, name string) (string, error) {
	storePath = strings.TrimSpace(storePath)
	if storePath == "" {
		return "", errors.New("profile store path is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("profile name is required")
	}
	if name == domain.DefaultProfileName {
		return "", errors.New("default profile cannot be created")
	}

	profilesDir := filepath.Join(storePath, profilesDirName)
	if err := ensureDir(profilesDir, true); err != nil {
		return "", err
	}

	candidate := filepath.Join(profilesDir, name+".yaml")
	altCandidate := filepath.Join(profilesDir, name+".yml")
	exists, err := fileExists(candidate)
	if err != nil {
		return "", err
	}
	if exists {
		return "", fmt.Errorf("profile %q already exists", name)
	}
	exists, err = fileExists(altCandidate)
	if err != nil {
		return "", err
	}
	if exists {
		return "", fmt.Errorf("profile %q already exists", name)
	}

	if err := os.WriteFile(candidate, []byte("servers: []\n"), fsutil.DefaultFileMode); err != nil {
		return "", fmt.Errorf("write profile file: %w", err)
	}
	return candidate, nil
}

func DeleteProfile(storePath string, name string) error {
	storePath = strings.TrimSpace(storePath)
	if storePath == "" {
		return errors.New("profile store path is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("profile name is required")
	}
	if name == domain.DefaultProfileName {
		return errors.New("default profile cannot be deleted")
	}

	path, err := ResolveProfilePath(storePath, name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove profile file: %w", err)
	}
	return nil
}

func SetCallerMapping(storePath string, caller string, profile string, profiles map[string]domain.Profile) (ProfileUpdate, error) {
	storePath = strings.TrimSpace(storePath)
	if storePath == "" {
		return ProfileUpdate{}, errors.New("profile store path is required")
	}
	caller = strings.TrimSpace(caller)
	if caller == "" {
		return ProfileUpdate{}, errors.New("caller is required")
	}
	profile = strings.TrimSpace(profile)
	if profile == "" {
		return ProfileUpdate{}, errors.New("profile is required")
	}
	if _, ok := profiles[profile]; !ok {
		return ProfileUpdate{}, fmt.Errorf("profile %q not found", profile)
	}

	callersPath := filepath.Join(storePath, callersFileName)
	if err := ensureCallersFile(callersPath, true); err != nil {
		return ProfileUpdate{}, err
	}
	callers, err := loadCallers(callersPath)
	if err != nil {
		return ProfileUpdate{}, err
	}
	callers[caller] = profile
	if err := validateCallers(callers, profiles); err != nil {
		return ProfileUpdate{}, err
	}

	update, err := writeCallersFile(callersPath, callers)
	if err != nil {
		return ProfileUpdate{}, err
	}
	return update, nil
}

func RemoveCallerMapping(storePath string, caller string, profiles map[string]domain.Profile) (ProfileUpdate, error) {
	storePath = strings.TrimSpace(storePath)
	if storePath == "" {
		return ProfileUpdate{}, errors.New("profile store path is required")
	}
	caller = strings.TrimSpace(caller)
	if caller == "" {
		return ProfileUpdate{}, errors.New("caller is required")
	}

	callersPath := filepath.Join(storePath, callersFileName)
	if err := ensureCallersFile(callersPath, true); err != nil {
		return ProfileUpdate{}, err
	}
	callers, err := loadCallers(callersPath)
	if err != nil {
		return ProfileUpdate{}, err
	}
	if _, ok := callers[caller]; !ok {
		return ProfileUpdate{}, fmt.Errorf("caller %q not found", caller)
	}
	delete(callers, caller)
	if err := validateCallers(callers, profiles); err != nil {
		return ProfileUpdate{}, err
	}

	update, err := writeCallersFile(callersPath, callers)
	if err != nil {
		return ProfileUpdate{}, err
	}
	return update, nil
}

func writeCallersFile(path string, callers map[string]string) (ProfileUpdate, error) {
	payload := rawCallers{Callers: callers}
	data, err := yaml.Marshal(payload)
	if err != nil {
		return ProfileUpdate{}, fmt.Errorf("render callers file: %w", err)
	}
	return ProfileUpdate{
		Path: path,
		Data: data,
	}, nil
}
