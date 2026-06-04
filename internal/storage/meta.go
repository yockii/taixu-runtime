package storage

import "errors"

func GetMeta(key string) (string, bool, error) {
	var v string
	err := db.QueryRow(`SELECT value FROM schema_meta WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func SetMeta(key, value string) error {
	_, err := db.Exec(`
		INSERT OR REPLACE INTO schema_meta (key, value, updated_at)
		VALUES (?, ?, strftime('%s','now'))`, key, value)
	return err
}
