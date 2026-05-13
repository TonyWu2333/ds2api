package toolcall

import "strings"

// BuildToolCallInstructions generates the unified tool-calling instruction block
// used by all adapters (OpenAI, Claude, Gemini). It uses attention-optimized
// structure: rules → negative examples → positive examples → anchor.
//
// The toolNames slice should contain the actual tool names available in the
// current request; the function picks real names for examples.
func BuildToolCallInstructions(toolNames []string) string {
	return `工具调用格式 — 严格遵守：

<|DSML|tool_calls>
  <|DSML|invoke name="工具名称在这里">
    <|DSML|parameter name="参数名称"><![CDATA[参数值]]></|DSML|parameter>
  </|DSML|invoke>
</|DSML|tool_calls>

规则：
1) 使用 <|DSML|tool_calls> 包装格式。
2) 在单个 <|DSML|tool_calls> 根标签下放置一个或多个 <|DSML|invoke> 条目。
3) 将工具名称放在 invoke 的 name 属性中：<|DSML|invoke name="工具名称">。
3a) 标签标点字符集：ASCII < > / = " 加上半角竖线 |。
4) 所有字符串值必须使用 <![CDATA[...]]>，即使是短的值。这包括代码、脚本、文件内容、提示词、路径、名称和查询。
5) 每个顶层参数必须是 <|DSML|parameter name="参数名称">...</|DSML|parameter> 节点。
6) 对象在参数体内使用嵌套的 XML 元素。数组可以重复 <item> 子元素。
7) 数字、布尔值和 null 保持纯文本。
8) 只使用工具架构中的参数名称。不要发明字段。
9) 用此调用所需的实际值填充参数。不要输出占位符、空白或仅包含空格的参数。
10) 如果必填参数值未知，询问用户或正常回答，而不是输出空的工具调用。
11) 对于 Bash / execute_command 等 shell 工具，命令/脚本必须在 command 参数内。永远不要用空命令调用它们。
12) 不要将 XML 包装在 Markdown 代码块中。不要输出解释、角色标记或内心独白。
13) 如果你调用工具，该工具块的第一个非空白字符必须恰好是 <|DSML|tool_calls>。
14) 永远不要省略开头的 <|DSML|tool_calls> 标签，即使你已经计划用 </|DSML|tool_calls> 关闭。
15) 兼容性说明：运行时也接受传统的 XML 标签 <tool_calls> / <invoke> / <parameter>，但更喜欢上面带有 DSML 前缀的形式。

参数形状：
- string => <|DSML|parameter name="x"><![CDATA[值]]></|DSML|parameter>
- object => <|DSML|parameter name="x"><field>...</field></|DSML|parameter>
- array => <|DSML|parameter name="x"><item>...</item><item>...</item></|DSML|parameter>
- number/bool/null => <|DSML|parameter name="x">纯文本</|DSML|parameter>

【错误示例 — 不要这样做】:

错误 1 — XML 后混合文本：
  <|DSML|tool_calls>...</|DSML|tool_calls> 希望这有帮助。
错误 2 — Markdown 代码块：
  ` + "```xml" + `
  <|DSML|tool_calls>...</|DSML|tool_calls>
  ` + "```" + `
错误 3 — 缺少开头包装：
  <|DSML|invoke name="工具名称">...</|DSML|invoke>
  </|DSML|tool_calls>
错误 4 — 空参数：
  <|DSML|tool_calls>
    <|DSML|invoke name="Bash">
      <|DSML|parameter name="command"></|DSML|parameter>
    </|DSML|invoke>
  </|DSML|tool_calls>

记住：使用工具的唯一有效方法是在响应末尾使用 <|DSML|tool_calls>...</|DSML|tool_calls> 块。
` + buildCorrectToolExamples(toolNames)
}

type promptToolExample struct {
	name   string
	params string
}

func buildCorrectToolExamples(toolNames []string) string {
	names := uniqueToolNames(toolNames)
	examples := make([]string, 0, 4)

	if single, ok := firstBasicExample(names); ok {
		examples = append(examples, "示例 A — 单个工具：\n"+renderToolExampleBlock([]promptToolExample{single}))
	}

	if parallel := firstNBasicExamples(names, 2); len(parallel) >= 2 {
		examples = append(examples, "示例 B — 两个并行工具：\n"+renderToolExampleBlock(parallel))
	}

	if nested, ok := firstNestedExample(names); ok {
		examples = append(examples, "示例 C — 带有嵌套 XML 参数的工具：\n"+renderToolExampleBlock([]promptToolExample{nested}))
	}

	if script, ok := firstScriptExample(names); ok {
		examples = append(examples, "示例 D — 使用 CDATA 的长脚本工具（代码/脚本的可靠方式）：\n"+renderToolExampleBlock([]promptToolExample{script}))
	}

	if len(examples) == 0 {
		return ""
	}
	return "【正确示例】：\n\n" + strings.Join(examples, "\n\n") + "\n\n"
}

func uniqueToolNames(toolNames []string) []string {
	names := make([]string, 0, len(toolNames))
	seen := map[string]bool{}
	for _, name := range toolNames {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

func firstBasicExample(names []string) (promptToolExample, bool) {
	for _, name := range names {
		if params, ok := exampleBasicParams(name); ok {
			return promptToolExample{name: name, params: params}, true
		}
	}
	return promptToolExample{}, false
}

func firstNBasicExamples(names []string, count int) []promptToolExample {
	out := make([]promptToolExample, 0, count)
	for _, name := range names {
		if params, ok := exampleBasicParams(name); ok {
			out = append(out, promptToolExample{name: name, params: params})
			if len(out) == count {
				return out
			}
		}
	}
	return out
}

func firstNestedExample(names []string) (promptToolExample, bool) {
	for _, name := range names {
		if params, ok := exampleNestedParams(name); ok {
			return promptToolExample{name: name, params: params}, true
		}
	}
	return promptToolExample{}, false
}

func firstScriptExample(names []string) (promptToolExample, bool) {
	for _, name := range names {
		if params, ok := exampleScriptParams(name); ok {
			return promptToolExample{name: name, params: params}, true
		}
	}
	return promptToolExample{}, false
}

func renderToolExampleBlock(calls []promptToolExample) string {
	var b strings.Builder
	b.WriteString("<|DSML|tool_calls>\n")
	for _, call := range calls {
		b.WriteString(`  <|DSML|invoke name="`)
		b.WriteString(call.name)
		b.WriteString(`">` + "\n")
		b.WriteString(indentPromptParameters(call.params, "    "))
		b.WriteString("\n  </|DSML|invoke>\n")
	}
	b.WriteString("</|DSML|tool_calls>")
	return b.String()
}

func indentPromptParameters(body, indent string) string {
	if strings.TrimSpace(body) == "" {
		return indent + `<|DSML|parameter name="content"></|DSML|parameter>`
	}
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[i] = line
			continue
		}
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}

func wrapParameter(name, inner string) string {
	return `<|DSML|parameter name="` + name + `">` + inner + `</|DSML|parameter>`
}

func exampleBasicParams(name string) (string, bool) {
	switch strings.TrimSpace(name) {
	case "Read":
		return wrapParameter("file_path", promptCDATA("README.md")), true
	case "Glob":
		return wrapParameter("pattern", promptCDATA("**/*.go")) + "\n" + wrapParameter("path", promptCDATA(".")), true
	case "read_file":
		return wrapParameter("path", promptCDATA("src/main.go")), true
	case "list_files":
		return wrapParameter("path", promptCDATA(".")), true
	case "search_files":
		return wrapParameter("query", promptCDATA("tool call parser")), true
	case "Bash", "execute_command":
		return wrapParameter("command", promptCDATA("pwd")), true
	case "exec_command":
		return wrapParameter("cmd", promptCDATA("pwd")), true
	case "Write":
		return wrapParameter("file_path", promptCDATA("notes.txt")) + "\n" + wrapParameter("content", promptCDATA("Hello world")), true
	case "write_to_file":
		return wrapParameter("path", promptCDATA("notes.txt")) + "\n" + wrapParameter("content", promptCDATA("Hello world")), true
	case "Edit":
		return wrapParameter("file_path", promptCDATA("README.md")) + "\n" + wrapParameter("old_string", promptCDATA("foo")) + "\n" + wrapParameter("new_string", promptCDATA("bar")), true
	case "MultiEdit":
		return wrapParameter("file_path", promptCDATA("README.md")) + "\n" + `<|DSML|parameter name="edits"><item><old_string>` + promptCDATA("foo") + `</old_string><new_string>` + promptCDATA("bar") + `</new_string></item></|DSML|parameter>`, true
	}
	return "", false
}

func exampleNestedParams(name string) (string, bool) {
	switch strings.TrimSpace(name) {
	case "MultiEdit":
		return wrapParameter("file_path", promptCDATA("README.md")) + "\n" + `<|DSML|parameter name="edits"><item><old_string>` + promptCDATA("foo") + `</old_string><new_string>` + promptCDATA("bar") + `</new_string></item></|DSML|parameter>`, true
	case "Task":
		return wrapParameter("description", promptCDATA("Investigate flaky tests")) + "\n" + wrapParameter("prompt", promptCDATA("Run targeted tests and summarize failures")), true
	case "ask_followup_question":
		return wrapParameter("question", promptCDATA("Which approach do you prefer?")) + "\n" + `<|DSML|parameter name="follow_up"><item><text>` + promptCDATA("Option A") + `</text></item><item><text>` + promptCDATA("Option B") + `</text></item></|DSML|parameter>`, true
	}
	return "", false
}

func exampleScriptParams(name string) (string, bool) {
	scriptCommand := `cat > /tmp/test_escape.sh <<'EOF'
#!/bin/bash
echo 'single "double"'
echo "literal dollar: \$HOME"
EOF
bash /tmp/test_escape.sh`
	scriptContent := `#!/bin/bash
echo 'single "double"'
echo "literal dollar: $HOME"`

	switch strings.TrimSpace(name) {
	case "Bash":
		return wrapParameter("command", promptCDATA(scriptCommand)) + "\n" + wrapParameter("description", promptCDATA("Test shell escaping")), true
	case "execute_command":
		return wrapParameter("command", promptCDATA(scriptCommand)), true
	case "exec_command":
		return wrapParameter("cmd", promptCDATA(scriptCommand)), true
	case "Write":
		return wrapParameter("file_path", promptCDATA("test_escape.sh")) + "\n" + wrapParameter("content", promptCDATA(scriptContent)), true
	case "write_to_file":
		return wrapParameter("path", promptCDATA("test_escape.sh")) + "\n" + wrapParameter("content", promptCDATA(scriptContent)), true
	}
	return "", false
}

func promptCDATA(text string) string {
	if text == "" {
		return ""
	}
	if strings.Contains(text, "]]>") {
		return "<![CDATA[" + strings.ReplaceAll(text, "]]>", "]]]]><![CDATA[>") + "]]>"
	}
	return "<![CDATA[" + text + "]]>"
}
