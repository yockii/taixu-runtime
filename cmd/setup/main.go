// Mindverse Setup CLI · Phase 0.1 脚手架。
//
// 子命令：
//   mindverse-setup help
//   mindverse-setup feishu   # Phase 0.3 完整接入；当前仅打印步骤
//   mindverse-setup llm      # Phase 0.3 完整接入；当前仅打印步骤
//   mindverse-setup env      # 写 .env 文件（交互式）
//
// 完整向导逻辑在 Phase 0.3 实施。
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}
	switch os.Args[1] {
	case "help", "-h", "--help":
		usage()
	case "feishu":
		feishuStub()
	case "llm":
		llmStub()
	case "env":
		writeEnv()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println(`Mindverse Setup CLI (Phase 0.1 stub)

Usage:
  mindverse-setup <command>

Commands:
  help    Show this message
  env     Interactively write .env from prompts
  feishu  Show Feishu setup steps (full wizard in Phase 0.3)
  llm     Show LLM setup steps (full wizard in Phase 0.3)`)
}

func feishuStub() {
	fmt.Println(`Feishu setup steps (Phase 0.3 will automate):

  1. Visit https://open.feishu.cn → create "企业自建应用"
  2. Permissions: im:message / im:message.group_at_msg / im:resource
  3. Mode: 长连接 (LongConnection / WebSocket)
  4. Copy App ID + App Secret to .env:
       FEISHU_APP_ID=cli_xxxxxxxx
       FEISHU_APP_SECRET=xxxxxxxx
  5. docker compose restart`)
}

func llmStub() {
	fmt.Println(`LLM setup steps (Phase 0.3 will automate):

  Supported (OpenAI-compatible):
    - OpenAI / Anthropic / DeepSeek / Zhipu GLM / Qwen / Moonshot / Ollama / vLLM / llama.cpp

  Required .env fields:
    LLM_BASE_URL=https://...
    LLM_API_KEY=sk-...
    LLM_MODEL=...
    LLM_TEMPERATURE=0.7`)
}

func writeEnv() {
	path := ".env"
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("%s exists. Overwrite? [y/N] ", path)
		if !readYN() {
			fmt.Println("aborted.")
			return
		}
	}

	r := bufio.NewReader(os.Stdin)
	fields := map[string]string{
		"LLM_BASE_URL":      prompt(r, "LLM_BASE_URL", ""),
		"LLM_API_KEY":       prompt(r, "LLM_API_KEY", ""),
		"LLM_MODEL":         prompt(r, "LLM_MODEL", ""),
		"LLM_TEMPERATURE":   prompt(r, "LLM_TEMPERATURE", "0.7"),
		"FEISHU_APP_ID":     prompt(r, "FEISHU_APP_ID", ""),
		"FEISHU_APP_SECRET": prompt(r, "FEISHU_APP_SECRET", ""),
	}

	order := []string{
		"LLM_BASE_URL", "LLM_API_KEY", "LLM_MODEL", "LLM_TEMPERATURE",
		"FEISHU_APP_ID", "FEISHU_APP_SECRET",
	}

	var b strings.Builder
	b.WriteString("# Mindverse Phase 0 .env\n# DO NOT commit this file.\n\n")
	for _, k := range order {
		b.WriteString(fmt.Sprintf("%s=%s\n", k, fields[k]))
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", path, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (0600)\n", path)
}

func prompt(r *bufio.Reader, key, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", key, def)
	} else {
		fmt.Printf("%s: ", key)
	}
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func readYN() bool {
	r := bufio.NewReader(os.Stdin)
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}
