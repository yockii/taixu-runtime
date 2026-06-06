package embed

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- codec 往返 ---

func TestEncodeDecodeRoundTrip(t *testing.T) {
	in := []float32{0, 1, -1, 3.14159, 1e-7, -2.5e3}
	b := Encode(in)
	if len(b) != 4*len(in) {
		t.Fatalf("encoded len = %d, want %d", len(b), 4*len(in))
	}
	out, err := Decode(b)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("decoded len = %d, want %d", len(out), len(in))
	}
	for i := range in {
		if in[i] != out[i] {
			t.Errorf("round trip mismatch at %d: %v != %v", i, in[i], out[i])
		}
	}
}

func TestEncodeNilEmpty(t *testing.T) {
	if Encode(nil) != nil {
		t.Error("Encode(nil) should be nil")
	}
	if Encode([]float32{}) != nil {
		t.Error("Encode(empty) should be nil")
	}
	v, err := Decode(nil)
	if err != nil || v != nil {
		t.Errorf("Decode(nil) = %v, %v; want nil, nil", v, err)
	}
}

func TestDecodeCorrupt(t *testing.T) {
	if _, err := Decode([]byte{1, 2, 3}); err == nil {
		t.Error("expected error on len%4 != 0")
	}
}

// --- cosine + top-k ---

func TestCosine(t *testing.T) {
	a := []float32{1, 0, 0}
	if got := Cosine(a, a); math.Abs(got-1) > 1e-6 {
		t.Errorf("self cosine = %v, want 1", got)
	}
	if got := Cosine([]float32{1, 0}, []float32{0, 1}); math.Abs(got) > 1e-6 {
		t.Errorf("orthogonal cosine = %v, want 0", got)
	}
	if got := Cosine([]float32{1, 1}, []float32{-1, -1}); math.Abs(got+1) > 1e-6 {
		t.Errorf("opposite cosine = %v, want -1", got)
	}
	// 维度不等 / 零向量 → 0
	if Cosine([]float32{1}, []float32{1, 2}) != 0 {
		t.Error("mismatched dim should be 0")
	}
	if Cosine([]float32{0, 0}, []float32{1, 1}) != 0 {
		t.Error("zero vector should be 0")
	}
}

func TestTopKOrderingAndSkips(t *testing.T) {
	q := []float32{1, 0}
	cands := []struct {
		ID   int64
		Blob []byte
	}{
		{1, Encode([]float32{1, 0})},     // cos 1.0
		{2, Encode([]float32{0.7, 0.7})}, // cos ~0.707
		{3, Encode([]float32{0, 1})},     // cos 0
		{4, nil},                         // 空 blob → 跳过
		{5, []byte{1, 2, 3}},             // 损坏 → 跳过
		{6, Encode([]float32{1, 0, 0})},  // 维度不符 → 跳过
	}
	top := TopK(q, cands, 2)
	if len(top) != 2 {
		t.Fatalf("top len = %d, want 2", len(top))
	}
	if top[0].ID != 1 || top[1].ID != 2 {
		t.Errorf("top ids = %d,%d; want 1,2", top[0].ID, top[1].ID)
	}
	if top[0].Score < top[1].Score {
		t.Error("not sorted descending")
	}
}

// --- Embed：未配置返错 + query 前缀 + httptest stub ---

func TestEmbedNotConfigured(t *testing.T) {
	Init(Config{BaseURL: ""})
	if Configured() {
		t.Fatal("should not be configured with empty BaseURL")
	}
	if _, err := Embed(context.Background(), []string{"hi"}, false); err == nil {
		t.Error("expected error when not configured")
	}
}

// stubServer 返回每条 input 的固定向量，并把收到的 input 暴露给测试断言前缀逻辑。
func stubServer(t *testing.T, captured *[]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v1/embeddings") {
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		var req embedReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		*captured = append(*captured, req.Input...)
		var resp embedResp
		for i := range req.Input {
			vec := make([]float32, Dim)
			vec[0] = float32(i + 1) // 每条给个可区分的非零向量
			resp.Data = append(resp.Data, struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{Embedding: vec, Index: i})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestEmbedQueryPrefix(t *testing.T) {
	var captured []string
	srv := stubServer(t, &captured)
	defer srv.Close()
	Init(Config{BaseURL: srv.URL, Timeout: 5 * time.Second})
	if !Configured() {
		t.Fatal("should be configured")
	}

	// doc 端：无前缀
	captured = nil
	if _, err := Embed(context.Background(), []string{"hello"}, false); err != nil {
		t.Fatalf("doc embed: %v", err)
	}
	if len(captured) != 1 || captured[0] != "hello" {
		t.Errorf("doc input = %v; want [hello] (no prefix)", captured)
	}

	// query 端：加 Instruct 前缀
	captured = nil
	if _, err := Embed(context.Background(), []string{"hello"}, true); err != nil {
		t.Fatalf("query embed: %v", err)
	}
	if len(captured) != 1 || !strings.HasPrefix(captured[0], "Instruct: ") || !strings.Contains(captured[0], "Query: hello") {
		t.Errorf("query input = %q; want Instruct/Query prefix", captured[0])
	}
}

func TestEmbedServerDown(t *testing.T) {
	srv := stubServer(t, &[]string{})
	url := srv.URL
	srv.Close() // 立刻关停 → 不可达
	Init(Config{BaseURL: url, Timeout: 1 * time.Second})
	if _, err := Embed(context.Background(), []string{"x"}, true); err == nil {
		t.Error("expected error when server unreachable")
	}
	// DocBlobBestEffort 在 server 挂时应优雅返回 nil（不 panic）
	if b := DocBlobBestEffort("x"); b != nil {
		t.Errorf("DocBlobBestEffort on down server = %v; want nil", b)
	}
}

func TestDimMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := embedResp{Data: []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		}{{Embedding: []float32{1, 2, 3}, Index: 0}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	Init(Config{BaseURL: srv.URL, Timeout: 5 * time.Second})
	if _, err := Embed(context.Background(), []string{"x"}, false); err == nil {
		t.Error("expected dim mismatch error")
	}
}
