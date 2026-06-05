package action

import (
	"path/filepath"
	"strings"
	"testing"

	"mindverse/internal/core"
	"mindverse/internal/storage"
)

// TestSedimentToSemantic 验证引擎权威把学透的知识沉淀进语义候选（digest 已有路径，不触发 LLM）。
// 修 sem_confirmed 恒 0：不再依赖 LLM 自觉调 record_learning。
func TestSedimentToSemantic(t *testing.T) {
	if err := storage.Init(filepath.Join(t.TempDir(), "a.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = storage.Close() }()

	const life = "life-sed"
	if err := storage.InsertGenome(&core.Genome{LifeID: life, BornAt: 1, GenomeVersion: "1"}); err != nil {
		t.Fatalf("genome: %v", err)
	}
	lifeID = life // 包内直接置，避开 Init 副作用

	seed := &storage.InterestSeed{
		ID:      1,
		Content: "进程间通信",
		Kind:    "knowledge",
		Digest:  "共享内存是最快的 IPC：内核映射同一物理页到多进程地址空间，省去内核态拷贝；需自管同步（信号量/futex）防竞态。",
		Mastery: 0.85,
	}
	sedimentToSemantic(seed, 100)

	above, err := storage.ListCandidatesAboveConfidence(life, 0.75, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(above) != 1 {
		t.Fatalf("want 1 semantic candidate >=0.75, got %d", len(above))
	}
	c := above[0]
	if !strings.Contains(c.Content, "进程间通信") || !strings.Contains(c.Content, "共享内存") {
		t.Errorf("candidate content missing topic/digest: %q", c.Content)
	}
	if c.Confidence < 0.84 { // 置信=mastery
		t.Errorf("candidate confidence=%.2f want ~0.85 (mastery)", c.Confidence)
	}
}
