package uiconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	updatedAtKey   = "__updated_at"
	reservedPrefix = "__"
)

var (
	ErrStoreClosed       = errors.New("ui settings store is closed")
	ErrMissingWorkspace  = errors.New("workspace id is required")
	ErrInvalidScope      = errors.New("invalid scope")
	ErrEmptyUpdates      = errors.New("updates or removes required")
	ErrInvalidSectionKey = errors.New("invalid section key")
)

type Scope string

const (
	ScopeGlobal    Scope = "global"
	ScopeWorkspace Scope = "workspace"
	ScopeEffective Scope = "effective"
)

type Snapshot struct {
	Scope       Scope
	WorkspaceID string
	Version     int
	UpdatedAt   string
	Sections    map[string]json.RawMessage
}

type Store struct {
	mu     sync.RWMutex
	db     *bolt.DB
	path   string
	closed bool
}

func OpenStore(path string) (*Store, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, fmt.Errorf("settings path is required")
	}
	if err := os.MkdirAll(filepath.Dir(trimmed), 0o755); err != nil {
		return nil, fmt.Errorf("ensure settings dir: %w", err)
	}
	options := &bolt.Options{Timeout: time.Second}
	base, err := bolt.Open(trimmed, 0o600, options)
	if err != nil {
		return nil, fmt.Errorf("open settings db: %w", err)
	}
	if err := ensureSchema(base); err != nil {
		_ = base.Close()
		return nil, err
	}
	return &Store{db: base, path: trimmed}, nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.db.Close()
}

func (s *Store) Get(scope Scope, workspaceID string) (Snapshot, error) {
	if err := validateScope(scope, workspaceID); err != nil {
		return Snapshot{}, err
	}
	var snapshot Snapshot
	err := s.view(func(tx *bolt.Tx) error {
		version, err := readVersion(tx)
		if err != nil {
			return err
		}
		snapshot = Snapshot{
			Scope:       scope,
			WorkspaceID: workspaceID,
			Version:     version,
			Sections:    map[string]json.RawMessage{},
		}
		bucket, err := scopeBucket(tx, scope, workspaceID, false)
		if err != nil {
			return err
		}
		if bucket == nil {
			return nil
		}
		snapshot.UpdatedAt = readUpdatedAt(bucket)
		return bucket.ForEach(func(key, value []byte) error {
			if value == nil || isReservedKey(key) {
				return nil
			}
			snapshot.Sections[string(key)] = append([]byte(nil), value...)
			return nil
		})
	})
	return snapshot, err
}

func (s *Store) GetEffective(workspaceID string) (Snapshot, error) {
	globalSnapshot, err := s.Get(ScopeGlobal, "")
	if err != nil {
		return Snapshot{}, err
	}
	if strings.TrimSpace(workspaceID) == "" {
		globalSnapshot.Scope = ScopeEffective
		return globalSnapshot, nil
	}
	workspaceSnapshot, err := s.Get(ScopeWorkspace, workspaceID)
	if err != nil {
		return Snapshot{}, err
	}
	merged := Snapshot{
		Scope:       ScopeEffective,
		WorkspaceID: workspaceID,
		Version:     globalSnapshot.Version,
		Sections:    map[string]json.RawMessage{},
		UpdatedAt:   globalSnapshot.UpdatedAt,
	}
	for key, value := range globalSnapshot.Sections {
		merged.Sections[key] = value
	}
	for key, value := range workspaceSnapshot.Sections {
		merged.Sections[key] = value
	}
	if workspaceSnapshot.UpdatedAt != "" {
		merged.UpdatedAt = workspaceSnapshot.UpdatedAt
	}
	return merged, nil
}

func (s *Store) Update(scope Scope, workspaceID string, updates map[string]json.RawMessage, removes []string) (Snapshot, error) {
	if err := validateScope(scope, workspaceID); err != nil {
		return Snapshot{}, err
	}
	if len(updates) == 0 && len(removes) == 0 {
		return Snapshot{}, ErrEmptyUpdates
	}
	for key := range updates {
		if err := validateSectionKey(key); err != nil {
			return Snapshot{}, err
		}
	}
	for _, key := range removes {
		if err := validateSectionKey(key); err != nil {
			return Snapshot{}, err
		}
	}
	if err := s.update(func(tx *bolt.Tx) error {
		bucket, err := scopeBucket(tx, scope, workspaceID, true)
		if err != nil {
			return err
		}
		for key, value := range updates {
			if value == nil {
				return fmt.Errorf("section value is nil for %s", key)
			}
			if err := bucket.Put([]byte(key), value); err != nil {
				return fmt.Errorf("write section %s: %w", key, err)
			}
		}
		for _, key := range removes {
			if err := bucket.Delete([]byte(key)); err != nil {
				return fmt.Errorf("delete section %s: %w", key, err)
			}
		}
		return writeUpdatedAt(bucket)
	}); err != nil {
		return Snapshot{}, err
	}
	return s.Get(scope, workspaceID)
}

func (s *Store) Reset(scope Scope, workspaceID string) (Snapshot, error) {
	if err := validateScope(scope, workspaceID); err != nil {
		return Snapshot{}, err
	}
	if err := s.update(func(tx *bolt.Tx) error {
		switch scope {
		case ScopeGlobal:
			bucket, err := scopeBucket(tx, scope, workspaceID, false)
			if err != nil || bucket == nil {
				return err
			}
			return clearBucket(bucket)
		case ScopeWorkspace:
			root := tx.Bucket([]byte(rootBucketName))
			if root == nil {
				return nil
			}
			scopes := root.Bucket([]byte(scopesBucketName))
			if scopes == nil {
				return nil
			}
			workspaces := scopes.Bucket([]byte(workspacesBucketName))
			if workspaces == nil {
				return nil
			}
			if workspaces.Bucket([]byte(workspaceID)) == nil {
				return nil
			}
			return workspaces.DeleteBucket([]byte(workspaceID))
		default:
			return ErrInvalidScope
		}
	}); err != nil {
		return Snapshot{}, err
	}
	return s.Get(scope, workspaceID)
}

func (s *Store) view(fn func(*bolt.Tx) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return ErrStoreClosed
	}
	return s.db.View(fn)
}

func (s *Store) update(fn func(*bolt.Tx) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return ErrStoreClosed
	}
	return s.db.Update(fn)
}

func validateScope(scope Scope, workspaceID string) error {
	switch scope {
	case ScopeGlobal:
		return nil
	case ScopeWorkspace:
		if strings.TrimSpace(workspaceID) == "" {
			return ErrMissingWorkspace
		}
		return nil
	default:
		return ErrInvalidScope
	}
}

func validateSectionKey(key string) error {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" || strings.HasPrefix(trimmed, reservedPrefix) {
		return ErrInvalidSectionKey
	}
	return nil
}

func readVersion(tx *bolt.Tx) (int, error) {
	root := tx.Bucket([]byte(rootBucketName))
	if root == nil {
		return 0, fmt.Errorf("missing root bucket")
	}
	meta := root.Bucket([]byte(metaBucketName))
	if meta == nil {
		return 0, fmt.Errorf("missing meta bucket")
	}
	version := readSchemaVersion(meta)
	if version == 0 {
		return 0, fmt.Errorf("schema version not set")
	}
	return version, nil
}

func scopeBucket(tx *bolt.Tx, scope Scope, workspaceID string, create bool) (*bolt.Bucket, error) {
	root := tx.Bucket([]byte(rootBucketName))
	if root == nil {
		return nil, fmt.Errorf("missing root bucket")
	}
	scopes := root.Bucket([]byte(scopesBucketName))
	if scopes == nil {
		return nil, fmt.Errorf("missing scopes bucket")
	}
	switch scope {
	case ScopeGlobal:
		bucket := scopes.Bucket([]byte(globalBucketName))
		if bucket == nil {
			return nil, fmt.Errorf("missing global bucket")
		}
		return bucket, nil
	case ScopeWorkspace:
		workspaces := scopes.Bucket([]byte(workspacesBucketName))
		if workspaces == nil {
			return nil, fmt.Errorf("missing workspaces bucket")
		}
		key := []byte(workspaceID)
		if create {
			bucket, err := workspaces.CreateBucketIfNotExists(key)
			if err != nil {
				return nil, fmt.Errorf("create workspace bucket: %w", err)
			}
			return bucket, nil
		}
		return workspaces.Bucket(key), nil
	default:
		return nil, ErrInvalidScope
	}
}

func readUpdatedAt(bucket *bolt.Bucket) string {
	if bucket == nil {
		return ""
	}
	value := bucket.Get([]byte(updatedAtKey))
	if len(value) == 0 {
		return ""
	}
	return string(value)
}

func writeUpdatedAt(bucket *bolt.Bucket) error {
	if bucket == nil {
		return nil
	}
	value := time.Now().UTC().Format(time.RFC3339Nano)
	return bucket.Put([]byte(updatedAtKey), []byte(value))
}

func isReservedKey(key []byte) bool {
	return strings.HasPrefix(string(key), reservedPrefix)
}

func clearBucket(bucket *bolt.Bucket) error {
	var keys [][]byte
	if err := bucket.ForEach(func(key, _ []byte) error {
		keys = append(keys, append([]byte(nil), key...))
		return nil
	}); err != nil {
		return err
	}
	for _, key := range keys {
		if err := bucket.Delete(key); err != nil {
			return err
		}
	}
	return nil
}
