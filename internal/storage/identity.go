package storage

// 生命身份配置：名字（诞生时自我命名）+ 母语（诞生前用户选定，驱动全 prompt/输出语言）。
// 复用 config kv（schema_meta `config:` 前缀），集中键名避免散落 magic string。

const (
	keyLifeName = "life_name"
	keyLifeLang = "life_lang"
)

// GetLifeName 生命体名字；未命名返空。
func GetLifeName() string { return GetConfigString(keyLifeName, "") }

// SetLifeName 写生命体名字。
func SetLifeName(name string) error { return SetConfigString(keyLifeName, name) }

// GetLifeLang 生命母语；未设默认 zh（现有生命/兼容）。
func GetLifeLang() string { return GetConfigString(keyLifeLang, "zh") }

// SetLifeLang 写生命母语。
func SetLifeLang(lang string) error { return SetConfigString(keyLifeLang, lang) }
