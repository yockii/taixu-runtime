package memory

import "testing"

// TestBigramJaccard 验 C6 内容闸相似度：同串=1、无overlap=0、重复探索类摘要≥阈值、不同主题<阈值。
func TestBigramJaccard(t *testing.T) {
	if j := bigramJaccard("交流协议探索", "交流协议探索"); j != 1.0 {
		t.Fatalf("同串应=1, 得 %.3f", j)
	}
	if j := bigramJaccard("abcdef", "xyzuvw"); j != 0.0 {
		t.Fatalf("无 overlap 应=0, 得 %.3f", j)
	}
	if j := bigramJaccard("", "x"); j != 0.0 {
		t.Fatalf("空串应=0, 得 %.3f", j)
	}
	// 近乎逐字重复（真卡死复读，summarizer 对同一活动产近同文本）→ 判重复降权（≥阈值）。
	a := "探索数字生命间的交流协议，查阅资料并记录要点"
	b := "探索数字生命间的交流协议，查阅资料并记录要点。"
	if j := bigramJaccard(a, b); j < episodeDupJaccard {
		t.Fatalf("近逐字重复应≥%.2f, 得 %.3f", episodeDupJaccard, j)
	}
	// 同主题但 reworded（有新角度，合法深化）→ 应低于阈值，不误判降权。
	c := "探索数字生命间的交流协议，查阅资料并记录要点"
	d := "探索数字生命间的交流协议，又换个角度查了资料、跑脚本验证了一遍"
	if j := bigramJaccard(c, d); j >= episodeDupJaccard {
		t.Fatalf("reworded 深化不应误判重复, 得 %.3f", j)
	}
	// 不同主题 → 远低于阈值。
	if j := bigramJaccard("探索交流协议，查阅资料", "创作一首关于星空与孤独的短诗"); j >= episodeDupJaccard {
		t.Fatalf("不同主题不应判重复, 得 %.3f", j)
	}
}
