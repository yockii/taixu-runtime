package memoryengine

import (
	"database/sql"
	"errors"
)

// GetMeta 读 schema_meta 单 key。第二返回值为 ok。
func (s *Store) GetMeta(key string) (string, bool, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM schema_meta WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

// SetMeta 写 schema_meta 单 key。INSERT OR REPLACE。
func (s *Store) SetMeta(key, value string) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO schema_meta (key, value, updated_at)
		VALUES (?, ?, strftime('%s','now'))`, key, value)
	return err
}
