package prompt

import "testing"

func TestStringifyToolCallArgumentsPreservesConcatenatedJSON(t *testing.T) {
	got := StringifyToolCallArguments(`{}{"query":"测试工具调用"}`)
	if got != `{}{"query":"测试工具调用"}` {
		t.Fatalf("expected raw concatenated JSON to be preserved, got %q", got)
	}
}

func TestFormatToolCallsForPromptDSML(t *testing.T) {
	got := FormatToolCallsForPrompt([]any{
		map[string]any{
			"id": "call_1",
			"function": map[string]any{
				"name":      "search_web",
				"arguments": map[string]any{"query": "latest"},
			},
		},
	})
	if got == "" {
		t.Fatal("expected non-empty formatted tool calls")
	}
	if got != "<|DSML|工具调用>\n  <|DSML|调用 name=\"search_web\">\n    <|DSML|参数 name=\"query\"><![CDATA[latest]]></|DSML|参数>\n  </|DSML|调用>\n</|DSML|工具调用>" {
		t.Fatalf("unexpected formatted tool call DSML: %q", got)
	}
}

func TestFormatToolCallsForPromptEscapesXMLEntities(t *testing.T) {
	got := FormatToolCallsForPrompt([]any{
		map[string]any{
			"name":      "search<&>",
			"arguments": `{"q":"a < b && c > d"}`,
		},
	})
	want := "<|DSML|工具调用>\n  <|DSML|调用 name=\"search&lt;&amp;&gt;\">\n    <|DSML|参数 name=\"q\"><![CDATA[a < b && c > d]]></|DSML|参数>\n  </|DSML|调用>\n</|DSML|工具调用>"
	if got != want {
		t.Fatalf("unexpected escaped tool call XML: %q", got)
	}
}

func TestFormatToolCallsForPromptUsesCDATAForMultilineContent(t *testing.T) {
	got := FormatToolCallsForPrompt([]any{
		map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"path":    "script.sh",
				"content": "#!/bin/bash\nprintf \"hello\"\n",
			},
		},
	})
	want := "<|DSML|工具调用>\n  <|DSML|调用 name=\"write_file\">\n    <|DSML|参数 name=\"content\"><![CDATA[#!/bin/bash\nprintf \"hello\"\n]]></|DSML|参数>\n    <|DSML|参数 name=\"path\"><![CDATA[script.sh]]></|DSML|参数>\n  </|DSML|调用>\n</|DSML|工具调用>"
	if got != want {
		t.Fatalf("unexpected multiline cdata tool call XML: %q", got)
	}
}
