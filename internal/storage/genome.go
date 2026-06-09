package storage

import (
	"errors"

	"taixu.icu/runtime/internal/core"
)

// LoadGenome 读取（首个）生命体 Genome。Phase 0 单生命；返回 (nil, ErrNoRows) 表示尚未出生。
func LoadGenome() (*core.Genome, error) {
	row := db.QueryRow(`
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
func InsertGenome(g *core.Genome) error {
	if g == nil {
		return errors.New("nil genome")
	}
	_, err := db.Exec(`
		INSERT INTO genome (life_id, curiosity, sociability, creativity, persistence, risk_taking, empathy, born_at, genome_version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.LifeID, g.Curiosity, g.Sociability, g.Creativity,
		g.Persistence, g.RiskTaking, g.Empathy, g.BornAt, g.GenomeVersion)
	return err
}

// HasGenome 是否已有出生记录。
func HasGenome() (bool, error) {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM genome`).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}
