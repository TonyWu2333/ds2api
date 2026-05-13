package promptcompat

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"ds2api/internal/toolcall"
)

const CurrentToolsContextFilename = "工具描述.txt"

const toolsTranscriptTitle = "# 工具描述.txt"
const toolsTranscriptSummary = "此请求可用的工具描述和参数架构。"

type toolPromptParts struct {
	Descriptions string
	Instructions string
	Names        []string
}

func injectToolPrompt(messages []map[string]any, tools []any, policy ToolChoicePolicy) ([]map[string]any, []string) {
	return injectToolPromptWithDescriptions(messages, tools, policy, true)
}

func injectToolPromptInstructionsOnly(messages []map[string]any, tools []any, policy ToolChoicePolicy) ([]map[string]any, []string) {
	return injectToolPromptWithDescriptions(messages, tools, policy, false)
}

func injectToolPromptWithDescriptions(messages []map[string]any, tools []any, policy ToolChoicePolicy, includeDescriptions bool) ([]map[string]any, []string) {
	if policy.IsNone() {
		return messages, nil
	}
	parts := buildToolPromptParts(tools, policy)
	if parts.Instructions == "" {
		return messages, parts.Names
	}
	toolPrompt := parts.Instructions
	// 只有 includeDescriptions = true 时才内联工具描述
	if includeDescriptions && parts.Descriptions != "" {
		toolPrompt = parts.Descriptions + "\n\n" + toolPrompt
	}

	for i := range messages {
		if messages[i]["role"] == "system" {
			old, _ := messages[i]["content"].(string)
			messages[i]["content"] = strings.TrimSpace(old + "\n\n" + toolPrompt)
			return messages, parts.Names
		}
	}
	messages = append([]map[string]any{{"role": "system", "content": toolPrompt}}, messages...)
	return messages, parts.Names
}

func buildToolPromptParts(tools []any, policy ToolChoicePolicy) toolPromptParts {
	toolSchemas := make([]string, 0, len(tools))
	names := make([]string, 0, len(tools))
	isAllowed := func(name string) bool {
		if strings.TrimSpace(name) == "" {
			return false
		}
		if len(policy.Allowed) == 0 {
			return true
		}
		_, ok := policy.Allowed[name]
		return ok
	}

	for _, t := range tools {
		tool, ok := t.(map[string]any)
		if !ok {
			continue
		}
		name, desc, schema := toolcall.ExtractToolMeta(tool)
		name = strings.TrimSpace(name)
		if !isAllowed(name) {
			continue
		}
		names = append(names, name)
		if desc == "" {
			desc = "没有可用描述"
		}
		b, _ := json.Marshal(schema)
		toolSchemas = append(toolSchemas, fmt.Sprintf("工具：%s\n描述：%s\n参数：%s", name, desc, string(b)))
	}
	if len(toolSchemas) == 0 {
		return toolPromptParts{Names: names}
	}
	descriptions := "你可以使用这些工具：\n\n" + strings.Join(toolSchemas, "\n\n")
	instructions := toolcall.BuildToolCallInstructions(names)
	if hasReadLikeTool(names) {
		instructions += "\n\n读取工具缓存保护：如果 Read/read_file 类工具的结果说文件未更改、已在历史记录中可用、应从先前上下文中引用，或者以其他方式没有提供文件体，请将该结果视为缺失内容。不要为该缺失体重复调用相同的读取请求。如果工具支持，请求完整内容读取，或者告诉用户需要重新提供文件内容。"
	}
	if policy.Mode == ToolChoiceRequired {
		instructions += "\n7) 对于此响应，你必须至少调用允许列表中的一个工具。"
	}
	if policy.Mode == ToolChoiceForced && strings.TrimSpace(policy.ForcedName) != "" {
		instructions += "\n7) 对于此响应，你必须恰好调用这个工具名称：" + strings.TrimSpace(policy.ForcedName)
		instructions += "\n8) 不要调用任何其他工具。"
	}
	return toolPromptParts{
		Descriptions: descriptions,
		Instructions: instructions,
		Names:        names,
	}
}

func BuildOpenAIToolsContextTranscript(toolsRaw any, policy ToolChoicePolicy) (string, []string) {
	if policy.IsNone() {
		return "", nil
	}
	tools, ok := toolsRaw.([]any)
	if !ok || len(tools) == 0 {
		return "", nil
	}
	parts := buildToolPromptParts(tools, policy)
	if strings.TrimSpace(parts.Descriptions) == "" {
		return "", parts.Names
	}
	var b strings.Builder
	b.WriteString(toolsTranscriptTitle)
	b.WriteString("\n")
	b.WriteString(toolsTranscriptSummary)
	b.WriteString("\n\n")
	b.WriteString(parts.Descriptions)
	b.WriteString("\n")
	return b.String(), parts.Names
}

func hasReadLikeTool(names []string) bool {
	for _, name := range names {
		switch normalizeToolNameForGuard(name) {
		case "read", "readfile":
			return true
		}
	}
	return false
}

func normalizeToolNameForGuard(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
