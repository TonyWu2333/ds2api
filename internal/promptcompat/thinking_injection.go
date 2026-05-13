package promptcompat

import "strings"

const (
	ThinkingInjectionMarker        = "推理努力：不允许任何捷径，必须全力以赴。"
	DefaultThinkingInjectionPrompt = ThinkingInjectionMarker + "\n" +
		"你必须进行非常深入的思考，全面分解问题以解决根本原因，针对所有可能的路径、边界情况和对抗性场景严格检验你的逻辑。\n" +
		"明确写出你的整个思考过程，记录每一个中间步骤、考虑过的替代方案和被拒绝的假设，确保绝对没有未被检验的假设。"
)

func AppendThinkingInjectionToLatestUser(messages []any) ([]any, bool) {
	return AppendThinkingInjectionPromptToLatestUser(messages, "")
}

func AppendThinkingInjectionPromptToLatestUser(messages []any, injectionPrompt string) ([]any, bool) {
	if len(messages) == 0 {
		return messages, false
	}
	injectionPrompt = strings.TrimSpace(injectionPrompt)
	if injectionPrompt == "" {
		injectionPrompt = DefaultThinkingInjectionPrompt
	}
	for i := len(messages) - 1; i >= 0; i-- {
		msg, ok := messages[i].(map[string]any)
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(asString(msg["role"]))) != "user" {
			continue
		}
		content := msg["content"]
		normalizedContent := NormalizeOpenAIContentForPrompt(content)
		if strings.Contains(normalizedContent, ThinkingInjectionMarker) || strings.Contains(normalizedContent, injectionPrompt) {
			return messages, false
		}
		updatedContent := appendThinkingInjectionToContent(content, injectionPrompt)
		out := append([]any(nil), messages...)
		cloned := make(map[string]any, len(msg))
		for k, v := range msg {
			cloned[k] = v
		}
		cloned["content"] = updatedContent
		out[i] = cloned
		return out, true
	}
	return messages, false
}

func appendThinkingInjectionToContent(content any, injectionPrompt string) any {
	switch x := content.(type) {
	case string:
		return appendTextBlock(x, injectionPrompt)
	case []any:
		out := append([]any(nil), x...)
		out = append(out, map[string]any{
			"type": "text",
			"text": injectionPrompt,
		})
		return out
	default:
		text := NormalizeOpenAIContentForPrompt(content)
		return appendTextBlock(text, injectionPrompt)
	}
}

func appendTextBlock(base, addition string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return addition
	}
	return base + "\n\n" + addition
}
