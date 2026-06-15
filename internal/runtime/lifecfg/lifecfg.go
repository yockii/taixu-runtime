// Package lifecfg 生命运行配置的单一解析/装配点：sqlite config(权威) 优先、env 兜底。
// main 启动装配 + httpapi 界面改配置都经此，避免 LLM 配置读取逻辑散落。
package lifecfg

import (
	"context"
	"errors"
	"os"
	"strconv"
	"time"

	"taixu.icu/runtime/internal/io/llm"
	"taixu.icu/runtime/internal/storage"
)

// 配置键（存 sqlite，复用 storage config: 前缀）。诞生 / 界面换 LLM 写这些。
const (
	KeyLLMBase  = "llm_base_url"
	KeyLLMKey   = "llm_api_key"
	KeyLLMModel = "llm_model"
	KeyLLMTemp  = "llm_temperature"

	KeyLLMStrongBase  = "llm_strong_base_url"
	KeyLLMStrongKey   = "llm_strong_api_key"
	KeyLLMStrongModel = "llm_strong_model"
	KeyLLMStrongTemp  = "llm_strong_temperature"

	// KeyControlToken 生命本地控制令牌：守卫 httpapi 写端点（改配置/审批/注入/对话）。
	// 诞生时强制设（随机预填可改）。复用 httpapi 既有 accessToken 机制（config 优先、env TAIXU_ACCESS_TOKEN 兜底）。
	KeyControlToken = "control_token"

	// 飞书凭据：界面手填 / 一键扫码创建后落库（config 优先、env FEISHU_* 兜底）。改后重启生效。
	KeyFeishuAppID  = "feishu_app_id"
	KeyFeishuSecret = "feishu_app_secret"

	// 微信 iLink bot_token：扫码登录后落库（config 优先、env WECHAT_BOT_TOKEN 兜底）。持久长效。
	KeyWechatBotToken = "wechat_bot_token"

	// KeyAutoUpgrade 自动升级开关：'1'=自动应用平台新版 runtime；其他/空=只通知、等用户确认。默认关。
	KeyAutoUpgrade = "auto_upgrade"
)

// AutoUpgrade 是否开了自动升级（sqlite config，默认关=通知模式）。
func AutoUpgrade() bool {
	return cfgOrEnv(KeyAutoUpgrade, "TAIXU_AUTO_UPGRADE", "") == "1"
}

// SetAutoUpgrade 设自动升级开关（界面切）。
func SetAutoUpgrade(on bool) error {
	v := "0"
	if on {
		v = "1"
	}
	return storage.SetConfigString(KeyAutoUpgrade, v)
}

// WechatBotToken 微信 iLink bot_token：sqlite config 优先、env 兜底。空=未登录。
func WechatBotToken() string {
	return cfgOrEnv(KeyWechatBotToken, "WECHAT_BOT_TOKEN", "")
}

// SetWechatBotToken 落库微信 bot_token（扫码成功后写）。重启生效。
func SetWechatBotToken(token string) error {
	return storage.SetConfigString(KeyWechatBotToken, token)
}

// FeishuConfig 飞书 app_id/secret：sqlite config 优先、env 兜底。两者皆非空才算配齐。
func FeishuConfig() (appID, secret string) {
	return cfgOrEnv(KeyFeishuAppID, "FEISHU_APP_ID", ""),
		cfgOrEnv(KeyFeishuSecret, "FEISHU_APP_SECRET", "")
}

// SetFeishuConfig 落库飞书凭据（手填 / 一键创建成功后写）。重启生效。
func SetFeishuConfig(appID, secret string) error {
	if err := storage.SetConfigString(KeyFeishuAppID, appID); err != nil {
		return err
	}
	return storage.SetConfigString(KeyFeishuSecret, secret)
}

// ControlToken 守卫令牌：sqlite config 优先、env TAIXU_ACCESS_TOKEN 兜底。空=未设(不鉴权)。
func ControlToken() string {
	return cfgOrEnv(KeyControlToken, "TAIXU_ACCESS_TOKEN", "")
}

// EffectiveLLM 当前生效的 default LLM 展示信息（base/model/temp，**不含密钥**）。面板回显用。
func EffectiveLLM() (base, model, temp string) {
	return cfgOrEnv(KeyLLMBase, "LLM_BASE_URL", ""),
		cfgOrEnv(KeyLLMModel, "LLM_MODEL", ""),
		cfgOrEnv(KeyLLMTemp, "LLM_TEMPERATURE", "")
}

// cfgOrEnv sqlite config 优先、env 兜底、都无返 def。
func cfgOrEnv(key, env, def string) string {
	if v := storage.GetConfigString(key, ""); v != "" {
		return v
	}
	if v := os.Getenv(env); v != "" {
		return v
	}
	return def
}

// LLMConfigured default LLM 三要素是否齐（sqlite 或 env 任一来源）。boot 门控据此判是否进诞生模式。
func LLMConfigured() bool {
	return cfgOrEnv(KeyLLMBase, "LLM_BASE_URL", "") != "" &&
		cfgOrEnv(KeyLLMKey, "LLM_API_KEY", "") != "" &&
		cfgOrEnv(KeyLLMModel, "LLM_MODEL", "") != ""
}

func parseTemp(s string, def float32) float32 {
	if s == "" {
		return def
	}
	if f, err := strconv.ParseFloat(s, 32); err == nil {
		return float32(f)
	}
	return def
}

// defaultLLM 读 default 模型配置（sqlite 优先 env 兜底）。
func defaultLLM() (llm.Config, bool) {
	base := cfgOrEnv(KeyLLMBase, "LLM_BASE_URL", "")
	key := cfgOrEnv(KeyLLMKey, "LLM_API_KEY", "")
	model := cfgOrEnv(KeyLLMModel, "LLM_MODEL", "")
	if base == "" || key == "" || model == "" {
		return llm.Config{}, false
	}
	return llm.Config{
		BaseURL:     base,
		APIKey:      key,
		Model:       model,
		Temperature: parseTemp(cfgOrEnv(KeyLLMTemp, "LLM_TEMPERATURE", ""), 0.7),
		Timeout:     90 * time.Second,
	}, true
}

// BuildLLM 据当前配置装配 default(+可选 strong)。无 default 配置返 err。
// 可重复调（llm.Init 支持替换）——界面换 LLM 后重装即生效。
func BuildLLM() error {
	c, ok := defaultLLM()
	if !ok {
		return errors.New("missing LLM config (base/key/model)")
	}
	if err := llm.Init(c); err != nil {
		return err
	}
	if sbase := cfgOrEnv(KeyLLMStrongBase, "LLM_STRONG_BASE_URL", ""); sbase != "" {
		smodel := cfgOrEnv(KeyLLMStrongModel, "LLM_STRONG_MODEL", "")
		if smodel == "" {
			smodel = c.Model
		}
		skey := cfgOrEnv(KeyLLMStrongKey, "LLM_STRONG_API_KEY", "")
		if skey == "" {
			skey = c.APIKey
		}
		_ = llm.InitModel(llm.ModelStrong, llm.Config{
			BaseURL:     sbase,
			APIKey:      skey,
			Model:       smodel,
			Temperature: parseTemp(cfgOrEnv(KeyLLMStrongTemp, "LLM_STRONG_TEMPERATURE", ""), c.Temperature),
			Timeout:     120 * time.Second,
		})
	}
	return nil
}

// TestLLM 测候选 default LLM 配置连通（不改在用模型）。诞生页 / 换 LLM 的「测试连通」按钮用。
// key 留空=沿用现有（面板回显时密钥被掩码，测试不必重输）。
func TestLLM(ctx context.Context, base, key, model string) error {
	if key == "" {
		key = cfgOrEnv(KeyLLMKey, "LLM_API_KEY", "")
	}
	return llm.Probe(ctx, llm.Config{BaseURL: base, APIKey: key, Model: model, Timeout: 30 * time.Second})
}

// Commit 诞生提交：测通 LLM → 写全套配置(llm + 母语 + 控制令牌) → 装配 LLM。
// 任一步失败不留半套坏配置（先 probe 再落库）。诞生页 /api/genesis/commit 调。
func Commit(ctx context.Context, base, key, model, temp, lang, token string) error {
	if err := llm.Probe(ctx, llm.Config{BaseURL: base, APIKey: key, Model: model, Timeout: 30 * time.Second}); err != nil {
		return err
	}
	if err := storage.SetConfigString(KeyLLMBase, base); err != nil {
		return err
	}
	if err := storage.SetConfigString(KeyLLMKey, key); err != nil {
		return err
	}
	if err := storage.SetConfigString(KeyLLMModel, model); err != nil {
		return err
	}
	if temp != "" {
		_ = storage.SetConfigString(KeyLLMTemp, temp)
	}
	if lang != "" {
		_ = storage.SetLifeLang(lang)
	}
	if token != "" {
		_ = storage.SetConfigString(KeyControlToken, token)
	}
	return BuildLLM()
}

// ApplyLLM 界面换 default LLM：先测通、再写 sqlite、再热重装。任一步失败不留半套坏配置。
// key 留空 = 沿用现有（用户只改 base/model 不重输密钥的场景）。
func ApplyLLM(ctx context.Context, base, key, model, temp string) error {
	if key == "" {
		key = cfgOrEnv(KeyLLMKey, "LLM_API_KEY", "")
	}
	if err := llm.Probe(ctx, llm.Config{BaseURL: base, APIKey: key, Model: model, Timeout: 30 * time.Second}); err != nil {
		return err
	}
	if err := storage.SetConfigString(KeyLLMBase, base); err != nil {
		return err
	}
	if err := storage.SetConfigString(KeyLLMKey, key); err != nil {
		return err
	}
	if err := storage.SetConfigString(KeyLLMModel, model); err != nil {
		return err
	}
	if temp != "" {
		_ = storage.SetConfigString(KeyLLMTemp, temp)
	}
	return BuildLLM()
}
