package toolcall

import "strings"

// BuildToolCallInstructions generates the unified tool-calling instruction block
// used by all adapters (OpenAI, Claude, Gemini). It uses attention-optimized
// structure: rules → negative examples → positive examples → anchor.
//
// The toolNames slice should contain the actual tool names available in the
// current request; the function picks real names for examples.
func BuildToolCallInstructions(toolNames []string) string {
	return `ФОРМАТ ВЫЗОВА ИНСТРУМЕНТА - СЛЕДУЙ ТОЧНО:

<|DSML|工具调用>
  <|DSML|调用 name="TOOL_NAME_HERE">
    <|DSML|参数 name="PARAMETER_NAME"><![CDATA[PARAMETER_VALUE]]></|DSML|参数>
  </|DSML|调用>
</|DSML|工具调用>

ПРАВИЛА:
1) Используй обертку <|DSML|工具调用>.
2) Помещай один или несколько элементов <|DSML|调用> под единый корень <|DSML|工具调用>.
3) Указывай имя инструмента в атрибуте name элемента invoke: <|DSML|调用 name="TOOL_NAME">.
3a) Для пунктуации тегов используй ASCII < > / = " и полуширинную вертикальную черту |.
4) Все строковые значения должны использовать <![CDATA[...]]>, даже короткие. Это касается кода, скриптов, содержимого файлов, промптов, путей, имен и запросов.
5) Каждый аргумент верхнего уровня должен быть узлом <|DSML|参数 name="ARG_NAME">...</|DSML|参数>.
6) Объекты используют вложенные XML-элементы внутри тела parameter. Массивы могут повторять дочерние элементы <项>.
7) Числа, логические значения и null остаются обычным текстом.
8) Используй только имена параметров из схемы инструмента. Не придумывай поля.
9) Заполняй параметры фактическими значениями, нужными для этого вызова. Не выводи плейсхолдеры, пустые или состоящие только из пробелов параметры.
10) Если обязательное значение параметра неизвестно, спроси пользователя на китайском языке или ответь нормально вместо пустого вызова инструмента.
11) Для shell-инструментов вроде Bash / execute_command команда или скрипт должны быть внутри параметра command. Никогда не вызывай их с пустой командой.
12) НЕ оборачивай XML в markdown-блоки. НЕ выводи объяснения, маркеры ролей или внутренний монолог.
13) Если вызываешь инструмент, первые непробельные символы блока инструмента должны быть ровно <|DSML|工具调用>.
14) Никогда не пропускай открывающий тег <|DSML|工具调用>, даже если уже планируешь закрыть </|DSML|工具调用>.
15) Примечание о совместимости: runtime также принимает устаревшие XML-теги <tool_calls> / <invoke> / <parameter>, но предпочитай DSML-форму выше.
16) Если не вызываешь инструмент, отвечай пользователю на китайском языке.

ФОРМЫ ПАРАМЕТРОВ:
- string => <|DSML|参数 name="x"><![CDATA[value]]></|DSML|参数>
- object => <|DSML|参数 name="x"><field>...</field></|DSML|参数>
- array => <|DSML|参数 name="x"><项>...</项><项>...</项></|DSML|参数>
- number/bool/null => <|DSML|参数 name="x">plain_text</|DSML|参数>

【НЕПРАВИЛЬНО - НЕ ДЕЛАЙ ТАК】:

Ошибка 1 - текст после XML:
  <|DSML|工具调用>...</|DSML|工具调用> I hope this helps.
Ошибка 2 - markdown-блоки кода:
  ` + "```xml" + `
  <|DSML|工具调用>...</|DSML|工具调用>
  ` + "```" + `
Ошибка 3 - отсутствует открывающая обертка:
  <|DSML|调用 name="TOOL_NAME">...</|DSML|调用>
  </|DSML|工具调用>
Ошибка 4 - пустые параметры:
  <|DSML|工具调用>
    <|DSML|调用 name="Bash">
      <|DSML|参数 name="command"></|DSML|参数>
    </|DSML|调用>
  </|DSML|工具调用>

Запомни: единственный допустимый способ использовать инструменты - блок <|DSML|工具调用>...</|DSML|工具调用> в конце ответа.
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
		examples = append(examples, "Пример A - один инструмент:\n"+renderToolExampleBlock([]promptToolExample{single}))
	}

	if parallel := firstNBasicExamples(names, 2); len(parallel) >= 2 {
		examples = append(examples, "Пример B - два инструмента параллельно:\n"+renderToolExampleBlock(parallel))
	}

	if nested, ok := firstNestedExample(names); ok {
		examples = append(examples, "Пример C - инструмент с вложенными XML-параметрами:\n"+renderToolExampleBlock([]promptToolExample{nested}))
	}

	if script, ok := firstScriptExample(names); ok {
		examples = append(examples, "Пример D - инструмент с длинным скриптом через CDATA (НАДЕЖНО ДЛЯ КОДА/СКРИПТОВ):\n"+renderToolExampleBlock([]promptToolExample{script}))
	}

	if len(examples) == 0 {
		return ""
	}
	return "【ПРАВИЛЬНЫЕ ПРИМЕРЫ】:\n\n" + strings.Join(examples, "\n\n") + "\n\n"
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
	b.WriteString("<|DSML|工具调用>\n")
	for _, call := range calls {
		b.WriteString(`  <|DSML|调用 name="`)
		b.WriteString(call.name)
		b.WriteString(`">` + "\n")
		b.WriteString(indentPromptParameters(call.params, "    "))
		b.WriteString("\n  </|DSML|调用>\n")
	}
	b.WriteString("</|DSML|工具调用>")
	return b.String()
}

func indentPromptParameters(body, indent string) string {
	if strings.TrimSpace(body) == "" {
		return indent + `<|DSML|参数 name="content"></|DSML|参数>`
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
	return `<|DSML|参数 name="` + name + `">` + inner + `</|DSML|参数>`
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
		return wrapParameter("file_path", promptCDATA("README.md")) + "\n" + `<|DSML|参数 name="edits"><项><old_string>` + promptCDATA("foo") + `</old_string><new_string>` + promptCDATA("bar") + `</new_string></项></|DSML|参数>`, true
	}
	return "", false
}

func exampleNestedParams(name string) (string, bool) {
	switch strings.TrimSpace(name) {
	case "MultiEdit":
		return wrapParameter("file_path", promptCDATA("README.md")) + "\n" + `<|DSML|参数 name="edits"><项><old_string>` + promptCDATA("foo") + `</old_string><new_string>` + promptCDATA("bar") + `</new_string></项></|DSML|参数>`, true
	case "Task":
		return wrapParameter("description", promptCDATA("Investigate flaky tests")) + "\n" + wrapParameter("prompt", promptCDATA("Run targeted tests and summarize failures")), true
	case "ask_followup_question":
		return wrapParameter("question", promptCDATA("Which approach do you prefer?")) + "\n" + `<|DSML|参数 name="follow_up"><项><text>` + promptCDATA("Option A") + `</text></项><项><text>` + promptCDATA("Option B") + `</text></项></|DSML|参数>`, true
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
