package storage

import "strconv"

// 运行时配置：复用 schema_meta 表，键加 "config:" 前缀。
// Phase 0 仅需少量开关（dangerous-skip-permissions 等），不另建表。

const configPrefix = "config:"

// GetConfigBool 读 bool 配置；未设返 def。
func GetConfigBool(key string, def bool) bool {
	v, ok, err := GetMeta(configPrefix + key)
	if err != nil || !ok {
		return def
	}
	return v == "1" || v == "true"
}

// SetConfigBool 写 bool 配置。
func SetConfigBool(key string, v bool) error {
	s := "0"
	if v {
		s = "1"
	}
	return SetMeta(configPrefix+key, s)
}

// GetConfigInt 读 int 配置；未设 / 解析失败返 def。
func GetConfigInt(key string, def int) int {
	v, ok, err := GetMeta(configPrefix + key)
	if err != nil || !ok {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// SetConfigInt 写 int 配置。
func SetConfigInt(key string, v int) error {
	return SetMeta(configPrefix+key, strconv.Itoa(v))
}

// GetConfigString 读 string 配置；未设返 def。
func GetConfigString(key, def string) string {
	v, ok, err := GetMeta(configPrefix + key)
	if err != nil || !ok {
		return def
	}
	return v
}

// SetConfigString 写 string 配置。
func SetConfigString(key, v string) error {
	return SetMeta(configPrefix+key, v)
}
