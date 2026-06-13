package llm

import "context"

// ReasonForceTool 强制 LLM 调用指定单工具（tool_choice=function）。
// 用于命名等需确定性结构化输出的内省调用：只给一个工具 + 强制选它，
// 防 LLM 跑偏（自由文本 / 调别的工具 / 拒答）。调用方从返回的 ToolCalls[0].ArgsJSON 取结构化结果。
func ReasonForceTool(ctx context.Context, msgs []Message, tool Tool) (ReasonResult, error) {
	return reasonInternal(ctx, ModelDefault, msgs, []Tool{tool}, tool.Name)
}
