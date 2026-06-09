package storage

import (
	"path/filepath"
	"testing"

	"taixu.icu/runtime/internal/core"
)

// TestConfirmedSemanticDecayRetract 验 C3：固化知识衰减 → 复confirm强化(不造重复行) → 反复衰减跌破地板撤回。
func TestConfirmedSemanticDecayRetract(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "m.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = Close() }()
	const life = "life-c3"
	if err := InsertGenome(&core.Genome{LifeID: life, BornAt: 1, GenomeVersion: "1"}); err != nil {
		t.Fatalf("genome: %v", err)
	}

	// 候选 → 固化一条知识。
	if err := UpsertSemanticCandidateConf(life, "fact-A", "test", 100, 0.9); err != nil {
		t.Fatalf("cand: %v", err)
	}
	cands, _ := ListCandidatesAboveConfidence(life, 0.75, 10)
	if len(cands) != 1 {
		t.Fatalf("应1候选, 得%d", len(cands))
	}
	if err := PromoteToConfirmed(life, cands[0].ID, "fact-A", 0.9, 200); err != nil {
		t.Fatalf("promote: %v", err)
	}
	conf, _ := ListSemanticConfirmed(life, "", 10)
	if len(conf) != 1 || conf[0].Confidence < 0.89 {
		t.Fatalf("固化应1条conf~0.9, 得%+v", conf)
	}

	// 衰减一次(0.97)：降信不撤回。
	if d, r, err := DecayConfirmedSemantic(life, 0.97, 0.3); err != nil || d != 1 || r != 0 {
		t.Fatalf("衰减1次应降1撤0, 得d=%d r=%d err=%v", d, r, err)
	}

	// 复confirm强化：再 promote 同 content → 不造重复行（升置信+刷新）。
	if err := UpsertSemanticCandidateConf(life, "fact-A", "test", 300, 0.9); err != nil {
		t.Fatalf("re-cand: %v", err)
	}
	cands, _ = ListCandidatesAboveConfidence(life, 0.75, 10)
	if err := PromoteToConfirmed(life, cands[0].ID, "fact-A", 0.9, 300); err != nil {
		t.Fatalf("re-promote: %v", err)
	}
	if conf, _ = ListSemanticConfirmed(life, "", 10); len(conf) != 1 {
		t.Fatalf("复confirm不应造重复行, 得%d条", len(conf))
	}

	// downgrade 主动反驳。
	if err := DowngradeConfirmedSemantic(life, "fact-A", 0.2); err != nil {
		t.Fatalf("downgrade: %v", err)
	}

	// 快衰减反复直到跌破地板撤回。
	retracted := false
	for i := 0; i < 100; i++ {
		if _, r, _ := DecayConfirmedSemantic(life, 0.5, 0.3); r > 0 {
			retracted = true
			break
		}
	}
	if !retracted {
		t.Fatal("反复衰减应最终撤回")
	}
	if conf, _ = ListSemanticConfirmed(life, "", 10); len(conf) != 0 {
		t.Fatalf("撤回后应0条, 得%d", len(conf))
	}
}
