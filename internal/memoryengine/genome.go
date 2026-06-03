package memoryengine

import (
	"database/sql"
	"errors"
	"mindverse/internal/core"
)

// LoadGenome 读取生命体 Genome。返回 (nil, sql.ErrNoRows) 表示尚未出生。
func (s *Store) LoadGenome() (*core.Genome, error) {
	row := s.db.QueryRow(`
		SELECT life_id, curiosity, sociability, creativity, persistence, risk_taking, empathy, born_at, genome_version
		FROM genome LIMIT 1`)
	var g core.Genome
	err := row.Scan(&g.LifeID, &g.Curiosity, &g.Sociability, &g.Creativity,
		&g.Persistence, &g.RiskTaking, &g.Empathy, &g.BornAt, &g.GenomeVersion)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// InsertGenome 仅在出生时调用。重复写入会因 PRIMARY KEY 冲突失败。
func (s *Store) InsertGenome(g *core.Genome) error {
	if g == nil {
		return errors.New("nil genome")
	}
	_, err := s.db.Exec(`
		INSERT INTO genome (life_id, curiosity, sociability, creativity, persistence, risk_taking, empathy, born_at, genome_version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.LifeID, g.Curiosity, g.Sociability, g.Creativity,
		g.Persistence, g.RiskTaking, g.Empathy, g.BornAt, g.GenomeVersion,
	)
	return err
}

// HasGenome 检查是否已有出生记录。
func (s *Store) HasGenome() (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM genome`).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// 显式 re-export 以便调用方判 sql.ErrNoRows
var ErrNoRows = sql.ErrNoRows
