package uiconfig

import (
	"encoding/binary"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

const (
	schemaVersion = 1

	rootBucketName       = "ui_settings"
	metaBucketName       = "meta"
	scopesBucketName     = "scopes"
	globalBucketName     = "global"
	workspacesBucketName = "workspaces"
	versionKey           = "version"
)

func ensureSchema(db *bolt.DB) error {
	return db.Update(func(tx *bolt.Tx) error {
		root, err := tx.CreateBucketIfNotExists([]byte(rootBucketName))
		if err != nil {
			return fmt.Errorf("create root bucket: %w", err)
		}
		meta, err := root.CreateBucketIfNotExists([]byte(metaBucketName))
		if err != nil {
			return fmt.Errorf("create meta bucket: %w", err)
		}
		scopes, err := root.CreateBucketIfNotExists([]byte(scopesBucketName))
		if err != nil {
			return fmt.Errorf("create scopes bucket: %w", err)
		}
		if _, err := scopes.CreateBucketIfNotExists([]byte(globalBucketName)); err != nil {
			return fmt.Errorf("create global bucket: %w", err)
		}
		if _, err := scopes.CreateBucketIfNotExists([]byte(workspacesBucketName)); err != nil {
			return fmt.Errorf("create workspaces bucket: %w", err)
		}

		currentVersion := readSchemaVersion(meta)
		switch {
		case currentVersion == 0:
			return writeSchemaVersion(meta, schemaVersion)
		case currentVersion > schemaVersion:
			return fmt.Errorf("unsupported ui settings schema version %d", currentVersion)
		case currentVersion < schemaVersion:
			if err := migrateSchema(tx, currentVersion, schemaVersion); err != nil {
				return err
			}
			return writeSchemaVersion(meta, schemaVersion)
		default:
			return nil
		}
	})
}

func migrateSchema(_ *bolt.Tx, fromVersion, toVersion int) error {
	if fromVersion == toVersion {
		return nil
	}
	return fmt.Errorf("missing migration path from %d to %d", fromVersion, toVersion)
}

func readSchemaVersion(meta *bolt.Bucket) int {
	if meta == nil {
		return 0
	}
	raw := meta.Get([]byte(versionKey))
	if len(raw) != 8 {
		return 0
	}
	return int(binary.BigEndian.Uint64(raw))
}

func writeSchemaVersion(meta *bolt.Bucket, version int) error {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(version))
	return meta.Put([]byte(versionKey), buf)
}
