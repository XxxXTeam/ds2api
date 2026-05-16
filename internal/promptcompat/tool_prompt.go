package promptcompat

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"ds2api/internal/toolcall"
)

const CurrentToolsContextFilename = "DS2API_TOOLS.txt"

const toolsTranscriptTitle = "# DS2API_TOOLS.txt"
const toolsTranscriptSummary = "Доступные описания инструментов и схемы параметров для этого запроса."

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
	if includeDescriptions && parts.Descriptions != "" {
		toolPrompt = parts.Descriptions + "\n\n" + toolPrompt
	} else if !includeDescriptions && parts.Descriptions != "" {
		toolPrompt = "Доступные описания инструментов и схемы параметров приложены в DS2API_TOOLS.txt. Считай DS2API_TOOLS.txt авторитетным списком вызываемых инструментов и схем; используй только перечисленные там инструменты и параметры. Если отвечаешь пользователю без вызова инструмента, отвечай на китайском языке.\n\n" + toolPrompt
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
			desc = "Описание недоступно"
		}
		b, _ := json.Marshal(schema)
		toolSchemas = append(toolSchemas, fmt.Sprintf("Инструмент: %s\nОписание: %s\nПараметры: %s", name, desc, string(b)))
	}
	if len(toolSchemas) == 0 {
		return toolPromptParts{Names: names}
	}
	descriptions := "Тебе доступны эти инструменты:\n\n" + strings.Join(toolSchemas, "\n\n")
	instructions := toolcall.BuildToolCallInstructions(names)
	if hasReadLikeTool(names) {
		instructions += "\n\nЗащита кэша инструмента чтения: если результат инструмента типа Read/read_file сообщает, что файл не изменился, уже доступен в истории, должен быть взят из предыдущего контекста или иначе не содержит тело файла, считай такой результат отсутствующим содержимым. Не вызывай повторно тот же запрос чтения ради отсутствующего тела. Запроси чтение полного содержимого, если инструмент это поддерживает, или сообщи пользователю на китайском языке, что содержимое файла нужно предоставить снова."
	}
	if policy.Mode == ToolChoiceRequired {
		instructions += "\n7) В этом ответе ты ОБЯЗАН вызвать хотя бы один инструмент из разрешенного списка."
	}
	if policy.Mode == ToolChoiceForced && strings.TrimSpace(policy.ForcedName) != "" {
		instructions += "\n7) В этом ответе ты ОБЯЗАН вызвать ровно этот инструмент: " + strings.TrimSpace(policy.ForcedName)
		instructions += "\n8) Не вызывай никакие другие инструменты."
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
