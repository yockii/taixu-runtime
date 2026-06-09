package llm

import "testing"

// TestModelRoutingFallback 验 C1 多模型路由：default 必配、strong 可选、未配 strong 时 resolveModel 回退 default。
// 纯注册/解析逻辑，不发真 API 请求。
func TestModelRoutingFallback(t *testing.T) {
	// 隔离：清空包级注册（测试间不串）。
	mu.Lock()
	models = map[string]*modelClient{}
	mu.Unlock()

	if Configured() {
		t.Fatal("空注册不应 Configured")
	}
	if HasModel(ModelStrong) {
		t.Fatal("未配 strong 不应 HasModel")
	}

	if err := Init(Config{BaseURL: "http://x", APIKey: "k", Model: "m-default"}); err != nil {
		t.Fatalf("Init default: %v", err)
	}
	if !Configured() {
		t.Fatal("配了 default 应 Configured")
	}
	if HasModel(ModelStrong) {
		t.Fatal("只配 default 时 strong 不应存在")
	}
	// 未配 strong → resolveModel(strong) 回退 default。
	if mc, ok := resolveModel(ModelStrong); !ok || mc.cfg.Model != "m-default" {
		t.Fatalf("未配 strong 时应回退 default, 得 ok=%v model=%q", ok, mc.cfg.Model)
	}

	if err := InitModel(ModelStrong, Config{BaseURL: "http://y", APIKey: "k2", Model: "m-strong"}); err != nil {
		t.Fatalf("InitModel strong: %v", err)
	}
	if !HasModel(ModelStrong) {
		t.Fatal("配了 strong 应 HasModel")
	}
	if mc, ok := resolveModel(ModelStrong); !ok || mc.cfg.Model != "m-strong" {
		t.Fatalf("配了 strong 时应得 strong, 得 ok=%v model=%q", ok, mc.cfg.Model)
	}
	if mc, ok := resolveModel(ModelDefault); !ok || mc.cfg.Model != "m-default" {
		t.Fatalf("default 仍应在, 得 ok=%v model=%q", ok, mc.cfg.Model)
	}

	// 不完整配置应报错。
	if err := InitModel("bad", Config{BaseURL: "http://z"}); err == nil {
		t.Fatal("缺 APIKey/Model 应报错")
	}

	// 复位 default 配置缺失校验。
	if err := Init(Config{}); err == nil {
		t.Fatal("default 缺字段应报错")
	}
}
