package promptcompat

import (
	"strings"
	"testing"
)

func TestBuildOpenAIFinalPrompt_HandlerPathIncludesToolRoundtripSemantics(t *testing.T) {
	messages := []any{
		map[string]any{"role": "user", "content": "查北京天气"},
		map[string]any{
			"role": "assistant",
			"tool_calls": []any{
				map[string]any{
					"id": "call_1",
					"function": map[string]any{
						"name":      "get_weather",
						"arguments": "{\"city\":\"beijing\"}",
					},
				},
			},
		},
		map[string]any{
			"role":         "tool",
			"tool_call_id": "call_1",
			"name":         "get_weather",
			"content":      map[string]any{"temp": 18, "condition": "sunny"},
		},
	}
	tools := []any{
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "get_weather",
				"description": "Get weather",
				"parameters": map[string]any{
					"type": "object",
				},
			},
		},
	}

	finalPrompt, toolNames := buildOpenAIFinalPrompt(messages, tools, "", false)
	if len(toolNames) != 1 || toolNames[0] != "get_weather" {
		t.Fatalf("unexpected tool names: %#v", toolNames)
	}
	if !strings.Contains(finalPrompt, `"condition":"sunny"`) {
		t.Fatalf("handler finalPrompt should preserve tool output content: %q", finalPrompt)
	}
	if !strings.Contains(finalPrompt, "<|DSML|工具调用>") {
		t.Fatalf("handler finalPrompt should preserve assistant tool history: %q", finalPrompt)
	}
	if !strings.Contains(finalPrompt, `<|DSML|调用 name="get_weather">`) {
		t.Fatalf("handler finalPrompt should include tool name history: %q", finalPrompt)
	}
}

func TestBuildOpenAIFinalPrompt_VercelPreparePathKeepsFinalAnswerInstruction(t *testing.T) {
	messages := []any{
		map[string]any{"role": "system", "content": "You are helpful"},
		map[string]any{"role": "user", "content": "请调用工具"},
	}
	tools := []any{
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "search",
				"description": "search docs",
				"parameters": map[string]any{
					"type": "object",
				},
			},
		},
	}

	finalPrompt, _ := buildOpenAIFinalPrompt(messages, tools, "", false)
	if !strings.Contains(finalPrompt, "Запомни: единственный допустимый способ использовать инструменты") {
		t.Fatalf("vercel prepare finalPrompt missing final tool-call anchor instruction: %q", finalPrompt)
	}
	if !strings.Contains(finalPrompt, "ФОРМАТ ВЫЗОВА ИНСТРУМЕНТА") {
		t.Fatalf("vercel prepare finalPrompt missing xml format instruction: %q", finalPrompt)
	}
	if !strings.Contains(finalPrompt, "НЕ оборачивай XML в markdown-блоки") {
		t.Fatalf("vercel prepare finalPrompt missing no-fence xml instruction: %q", finalPrompt)
	}
	if strings.Contains(finalPrompt, "```json") {
		t.Fatalf("vercel prepare finalPrompt should not require fenced tool calls: %q", finalPrompt)
	}
}

func TestBuildOpenAIPromptWithToolInstructionsOnlyOmitsSchemas(t *testing.T) {
	messages := []any{
		map[string]any{"role": "system", "content": "You are helpful"},
		map[string]any{"role": "user", "content": "请调用工具"},
	}
	tools := []any{
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "search",
				"description": "search docs",
				"parameters": map[string]any{
					"type": "object",
				},
			},
		},
	}

	finalPrompt, toolNames := BuildOpenAIPromptWithToolInstructionsOnly(messages, tools, "", DefaultToolChoicePolicy(), false)
	if len(toolNames) != 1 || toolNames[0] != "search" {
		t.Fatalf("unexpected tool names: %#v", toolNames)
	}
	if strings.Contains(finalPrompt, "Тебе доступны эти инструменты") || strings.Contains(finalPrompt, "Описание: search docs") || strings.Contains(finalPrompt, "Параметры:") {
		t.Fatalf("tool descriptions should be externalized, got: %q", finalPrompt)
	}
	if !strings.Contains(finalPrompt, "Считай DS2API_TOOLS.txt авторитетным списком") {
		t.Fatalf("expected instructions-only prompt to point model at tools file, got: %q", finalPrompt)
	}
	if !strings.Contains(finalPrompt, "ФОРМАТ ВЫЗОВА ИНСТРУМЕНТА") || !strings.Contains(finalPrompt, "Запомни: единственный допустимый способ использовать инструменты") {
		t.Fatalf("expected tool format instructions to remain in live prompt, got: %q", finalPrompt)
	}
}

func TestBuildOpenAIToolsContextTranscriptContainsOnlyDescriptions(t *testing.T) {
	tools := []any{
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "search",
				"description": "search docs",
				"parameters": map[string]any{
					"type": "object",
				},
			},
		},
	}

	transcript, toolNames := BuildOpenAIToolsContextTranscript(tools, DefaultToolChoicePolicy())
	if len(toolNames) != 1 || toolNames[0] != "search" {
		t.Fatalf("unexpected tool names: %#v", toolNames)
	}
	for _, want := range []string{"# DS2API_TOOLS.txt", "Тебе доступны эти инструменты", "Инструмент: search", "Описание: search docs", `Параметры: {"type":"object"}`} {
		if !strings.Contains(transcript, want) {
			t.Fatalf("expected tools transcript to contain %q, got: %q", want, transcript)
		}
	}
	if strings.Contains(transcript, "ФОРМАТ ВЫЗОВА ИНСТРУМЕНТА") || strings.Contains(transcript, "<|DSML|工具调用>") {
		t.Fatalf("tools transcript should not duplicate format instructions, got: %q", transcript)
	}
}

func TestBuildOpenAIFinalPromptPrependsOutputIntegrityGuard(t *testing.T) {
	messages := []any{
		map[string]any{"role": "system", "content": "You are helpful"},
		map[string]any{"role": "user", "content": "请调用工具"},
	}
	tools := []any{
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "search",
				"description": "search docs",
				"parameters": map[string]any{
					"type": "object",
				},
			},
		},
	}

	finalPrompt, _ := buildOpenAIFinalPrompt(messages, tools, "", false)
	guardIdx := strings.Index(finalPrompt, "Защита целостности вывода")
	toolIdx := strings.Index(finalPrompt, "ФОРМАТ ВЫЗОВА ИНСТРУМЕНТА")
	if guardIdx < 0 {
		t.Fatalf("expected output integrity guard in final prompt, got: %q", finalPrompt)
	}
	if toolIdx < 0 {
		t.Fatalf("expected tool instructions in final prompt, got: %q", finalPrompt)
	}
	if guardIdx > toolIdx {
		t.Fatalf("expected output integrity guard to precede tool instructions, got: %q", finalPrompt)
	}
}

func TestBuildOpenAIFinalPromptReadLikeToolIncludesCacheGuard(t *testing.T) {
	messages := []any{
		map[string]any{"role": "user", "content": "请读取文件"},
	}
	tools := []any{
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "read_file",
				"description": "Read a file",
				"parameters": map[string]any{
					"type": "object",
				},
			},
		},
	}

	finalPrompt, _ := buildOpenAIFinalPrompt(messages, tools, "", false)
	if !strings.Contains(finalPrompt, "Защита кэша инструмента чтения") {
		t.Fatalf("read-like tool prompt missing cache guard: %q", finalPrompt)
	}
	if !strings.Contains(finalPrompt, "не содержит тело файла") {
		t.Fatalf("read-like tool prompt missing no-body handling: %q", finalPrompt)
	}
	if !strings.Contains(finalPrompt, "Не вызывай повторно тот же запрос чтения") {
		t.Fatalf("read-like tool prompt missing loop guard: %q", finalPrompt)
	}
}

func TestBuildOpenAIFinalPromptNonReadToolOmitsCacheGuard(t *testing.T) {
	messages := []any{
		map[string]any{"role": "user", "content": "搜索一下"},
	}
	tools := []any{
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "search",
				"description": "Search docs",
				"parameters": map[string]any{
					"type": "object",
				},
			},
		},
	}

	finalPrompt, _ := buildOpenAIFinalPrompt(messages, tools, "", false)
	if strings.Contains(finalPrompt, "Защита кэша инструмента чтения") {
		t.Fatalf("non-read tool prompt should not include read cache guard: %q", finalPrompt)
	}
}

func TestBuildOpenAIFinalPromptWithThinkingKeepsPromptUnchanged(t *testing.T) {
	messages := []any{
		map[string]any{"role": "user", "content": "继续回答上一个问题"},
	}

	finalPromptThinking, _ := buildOpenAIFinalPrompt(messages, nil, "", true)
	finalPromptPlain, _ := buildOpenAIFinalPrompt(messages, nil, "", false)
	if finalPromptThinking != finalPromptPlain {
		t.Fatalf("expected thinking flag not to prepend continuation contract, thinking=%q plain=%q", finalPromptThinking, finalPromptPlain)
	}
}
