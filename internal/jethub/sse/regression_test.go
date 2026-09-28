package sse

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRegressionCodeBuddyReasoningFrames drives a realistic CodeBuddy reasoning
// stream through the framing helper every passthrough executor uses, and asserts
// the property the AI SDK depends on: no forwarded frame may carry an empty
// `tool_calls` array.
//
// The fixture reproduces the real wire format byte for byte (see the captured
// `key1.sse` capture in the analysis): `content` is always `""`, `tool_calls` is
// always `[]`, and each frame holds one short reasoning fragment. Before the
// sanitizer landed, feeding this through `@ai-sdk/openai-compatible` produced one
// `reasoning-start` per frame — 107 segments for 177 characters — which is what
// rendered as one word per row in ZCode.
func TestRegressionCodeBuddyReasoningFrames(t *testing.T) {
	fragments := []string{"我们", "需要", "回答", "中文", "。", "哈希", "表是", "一种", "数据", "结构"}
	var upstream strings.Builder
	for _, fragment := range fragments {
		encoded, errMarshal := json.Marshal(map[string]any{
			"id":      "cmb-9f3d2f46bae211f1bbbbc2eea00ba509",
			"model":   "deepseek-v4.1-flash",
			"object":  "chat.completion.chunk",
			"created": 1790561792,
			"choices": []any{map[string]any{
				"index":         0,
				"logprobs":      nil,
				"finish_reason": "",
				"delta": map[string]any{
					"content":           "",
					"reasoning_content": fragment,
					"function_call":     nil,
					"refusal":           "",
					"tool_calls":        []any{},
					"extra_fields":      nil,
				},
			}},
			"usage": nil,
		})
		if errMarshal != nil {
			t.Fatalf("marshal fixture: %v", errMarshal)
		}
		upstream.WriteString("data: " + string(encoded) + "\n\n")
	}
	upstream.WriteString("data: [DONE]\n\n")

	scanner := &Scanner{}
	seen := 0
	reassembled := strings.Builder{}
	for _, payload := range scanner.Feed([]byte(upstream.String())) {
		if payload == Done {
			break
		}
		forwarded := string(Payload(payload))
		seen++
		if strings.Contains(forwarded, `"tool_calls"`) {
			t.Fatalf("frame %d still carries tool_calls: %s", seen, forwarded)
		}
		// The reasoning text itself must be untouched, and the large `created`
		// integer must not have been re-rendered by a round trip through `any`.
		var decoded struct {
			Created int64 `json:"created"`
			Choices []struct {
				Delta struct {
					ReasoningContent string `json:"reasoning_content"`
					Content          string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if errUnmarshal := json.Unmarshal([]byte(forwarded), &decoded); errUnmarshal != nil {
			t.Fatalf("forwarded frame %d is not JSON: %v (%s)", seen, errUnmarshal, forwarded)
		}
		if decoded.Created != 1790561792 {
			t.Errorf("created = %d, want 1790561792 (payload was re-marshalled?)", decoded.Created)
		}
		if len(decoded.Choices) != 1 {
			t.Fatalf("choices changed shape: %s", forwarded)
		}
		if decoded.Choices[0].Delta.Content != "" {
			t.Errorf("content changed: %q", decoded.Choices[0].Delta.Content)
		}
		reassembled.WriteString(decoded.Choices[0].Delta.ReasoningContent)
	}
	if seen != len(fragments) {
		t.Fatalf("forwarded %d frames, want %d", seen, len(fragments))
	}
	if got := reassembled.String(); got != strings.Join(fragments, "") {
		t.Fatalf("reasoning text = %q, want %q", got, strings.Join(fragments, ""))
	}
}

// Tool-call streams must still work: the sanitizer must not swallow the frames
// that carry real tool calls, or function calling would break on every plugin.
func TestRegressionToolCallFramesSurvive(t *testing.T) {
	chunks := []string{
		`{"id":"c1","choices":[{"index":0,"delta":{"role":"assistant","content":"","tool_calls":[]},"finish_reason":""}]}`,
		`{"id":"c1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":""}}]},"finish_reason":""}]}`,
		`{"id":"c1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"SF\"}"}}]},"finish_reason":""}]}`,
		`{"id":"c1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
	}
	var body strings.Builder
	for _, chunk := range chunks {
		body.WriteString("data: " + chunk + "\n\n")
	}
	scanner := &Scanner{}
	var forwarded []string
	for _, payload := range scanner.Feed([]byte(body.String())) {
		if payload == Done {
			break
		}
		forwarded = append(forwarded, string(Payload(payload)))
	}
	if len(forwarded) != len(chunks) {
		t.Fatalf("forwarded %d frames, want %d", len(forwarded), len(chunks))
	}
	if strings.Contains(forwarded[0], `"tool_calls"`) {
		t.Errorf("empty array survived in frame 0: %s", forwarded[0])
	}
	// Real tool-call content must be preserved exactly, including the nested
	// escaped JSON in `function.arguments`.
	joined := strings.Join(forwarded, "\n")
	for _, fragment := range []string{`"name":"lookup"`, `"arguments":""`, `city`, `SF`, `"finish_reason":"tool_calls"`} {
		if !strings.Contains(joined, fragment) {
			t.Errorf("tool-call fragment %q was lost", fragment)
		}
	}
}
