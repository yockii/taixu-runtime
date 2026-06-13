package main

import (
	"embed"
	"io/fs"
)

// version 构建版本，CI 经 -ldflags "-X main.version=<tag>" 注入；本地默认 dev。
var version = "dev"

// SvelteKit build 产物（//go:embed 必须在与 .go 文件同包；空目录占位 .gitkeep 让构建成功）。
//
//go:embed all:webbuild
var webBuild embed.FS

// webStaticFS 暴露 webbuild/ 下的子文件系统作为根。
// 若空（仅含 .gitkeep）则返回 nil，httpapi 回退到占位提示。
func webStaticFS() fs.FS {
	sub, err := fs.Sub(webBuild, "webbuild")
	if err != nil {
		return nil
	}
	// 简易检测：尝试打开 index.html；不存在则返回 nil。
	if _, err := sub.Open("index.html"); err != nil {
		return nil
	}
	return sub
}
