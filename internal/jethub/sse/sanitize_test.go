package sse

import (
	"encoding/json"
	"strings"
	"testing"
)

// Empty `tool_calls` arrays are the CodeBuddy convention that split a reasoning
// stream into one segment per token in segment-based clients. These tests pin
// both directions: the member must go, and nothing else in the frame may move.

func TestSanitizePayloadDropsEmptyToolCalls(t *testing.T) {
	// The exact shape CodeBuddy sends on every reasoning frame.
	frame := `{"id":"cmb-9f3d","model":"deepseek-v4.1-flash","object":"chat.completion.chunk","created":1790561792,"choices":[{"index":0,"delta":{"content":"","reasoning_content":"我们需要","function_call":null,"refusal":"","tool_calls":[],"extra_fields":null},"logprobs":null,"finish_reason":""}],"usage":null}`
	got := SanitizePayload(frame)
	if strings.Contains(got, `"tool_calls"`) {
		t.Fatalf("empty tool_calls survived: %s", got)
	}
	// Everything else must survive verbatim, including the large integer that a
	// round-trip through map[string]any would have re-rendered as 1.79e+09.
	for _, want := range []string{
		`"created":1790561792`,
		`"id":"cmb-9f3d"`,
		`"reasoning_content":"我们需要"`,
		`"function_call":null`,
		`"refusal":""`,
		`"extra_fields":null`,
		`"finish_reason":""`,
		`"model":"deepseek-v4.1-flash"`,
		`"usage":null`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("sanitized frame lost %s: %s", want, got)
		}
	}
	var decoded map[string]any
	if errUnmarshal := json.Unmarshal([]byte(got), &decoded); errUnmarshal != nil {
		t.Fatalf("sanitized frame is not valid JSON: %v (%s)", errUnmarshal, got)
	}
}

func TestSanitizePayloadKeepsMeaningfulToolCalls(t *testing.T) {
	// A frame that really does carry tool calls must be relayed untouched, or
	// tool use breaks.
	frame := `{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]}}]}`
	if got := SanitizePayload(frame); got != frame {
		t.Fatalf("real tool call was rewritten:\n got %s\nwant %s", got, frame)
	}
}

func TestSanitizePayloadLeavesNonEmptyArraysAlone(t *testing.T) {
	for _, frame := range []string{
		`{"choices":[{"delta":{"tool_calls":[{"index":0}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{}]}}]}`,
	} {
		if got := SanitizePayload(frame); got != frame {
			t.Errorf("non-empty tool_calls was rewritten: %s -> %s", frame, got)
		}
	}
}

// `null` and a missing member already mean "no tool call" to the AI SDK, so both
// must be relayed byte for byte rather than normalised.
func TestSanitizePayloadIgnoresNullAndMissing(t *testing.T) {
	for _, frame := range []string{
		`{"choices":[{"delta":{"reasoning_content":"x","tool_calls":null}}]}`,
		`{"choices":[{"delta":{"reasoning_content":"x"}}]}`,
		`{"choices":[{"delta":{}}]}`,
	} {
		if got := SanitizePayload(frame); got != frame {
			t.Errorf("frame with no empty array was rewritten: %s -> %s", frame, got)
		}
	}
}

// The sanitizer splices text; it must never re-serialise. Whitespace, key order
// and vendor fields therefore survive exactly.
func TestSanitizePayloadPreservesFormatting(t *testing.T) {
	frame := "{\n  \"choices\" : [ {\n    \"delta\" : { \"reasoning_content\" : \"x\" , \"tool_calls\" : [ ] , \"z\" : 1 }\n  } ]\n}"
	got := SanitizePayload(frame)
	if strings.Contains(got, "tool_calls") {
		t.Fatalf("empty tool_calls survived: %q", got)
	}
	var decoded map[string]any
	if errUnmarshal := json.Unmarshal([]byte(got), &decoded); errUnmarshal != nil {
		t.Fatalf("not valid JSON: %v (%q)", errUnmarshal, got)
	}
	if !strings.Contains(got, `"z" : 1`) {
		t.Errorf("formatting was normalised: %q", got)
	}
}

// The member may be the first, the only, or the last key of `delta`; the comma
// handling differs in each case.
func TestSanitizePayloadHandlesCommaPositions(t *testing.T) {
	cases := []struct {
		name  string
		frame string
	}{
		{"only member", `{"choices":[{"delta":{"tool_calls":[]}}]}`},
		{"first member", `{"choices":[{"delta":{"tool_calls":[],"content":"hi"}}]}`},
		{"middle member", `{"choices":[{"delta":{"content":"hi","tool_calls":[],"reasoning_content":"x"}}]}`},
		{"last member", `{"choices":[{"delta":{"content":"hi","tool_calls":[]}}]}`},
		{"spaced", `{"choices":[{"delta":{ "content" : "hi" , "tool_calls" : [ ] }}]}`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := SanitizePayload(testCase.frame)
			if strings.Contains(got, "tool_calls") {
				t.Fatalf("empty tool_calls survived: %s", got)
			}
			var decoded map[string]any
			if errUnmarshal := json.Unmarshal([]byte(got), &decoded); errUnmarshal != nil {
				t.Fatalf("not valid JSON: %v (%s)", errUnmarshal, got)
			}
			choices, _ := decoded["choices"].([]any)
			if len(choices) != 1 {
				t.Fatalf("choices changed shape: %s", got)
			}
			choice, _ := choices[0].(map[string]any)
			delta, _ := choice["delta"].(map[string]any)
			if _, present := delta["tool_calls"]; present {
				t.Errorf("tool_calls still present after decode: %s", got)
			}
			if testCase.name == "middle member" && delta["content"] != "hi" {
				t.Errorf("sibling member was damaged: %s", got)
			}
		})
	}
}

// Only `delta.tool_calls` is in scope. A `tool_calls` member elsewhere in the
// frame (a top-level vendor extension, or one nested in a tool result) must not
// be touched, or the sanitizer would be rewriting unrelated parts of the API.
func TestSanitizePayloadOnlyTouchesDeltaToolCalls(t *testing.T) {
	frame := `{"tool_calls":[],"choices":[{"delta":{"content":"hi"},"message":{"tool_calls":[]}}]}`
	if got := SanitizePayload(frame); got != frame {
		t.Fatalf("a tool_calls member outside delta was rewritten: %s", got)
	}
}

// A frame the walker cannot parse must be relayed untouched: a broken frame is
// the executor's business to classify, not the sanitizer's to rewrite.
func TestSanitizePayloadLeavesUnparsablePayloadAlone(t *testing.T) {
	for _, frame := range []string{
		``,
		`not json`,
		`{"choices":[{"delta":{"tool_calls":[]}}`, // truncated
		`[DONE]`,
		`{"choices":[{"delta":{"tool_calls":[}}]}`, // malformed value
	} {
		if got := SanitizePayload(frame); got != frame {
			t.Errorf("unparsable payload was rewritten: %q -> %q", frame, got)
		}
	}
}

// Payload is the single choke point every passthrough executor funnels through,
// so the sanitizer must be visible from there.
func TestPayloadSanitizesEmptyToolCalls(t *testing.T) {
	payload := Payload(`{"choices":[{"delta":{"reasoning_content":"x","tool_calls":[]}}]}`)
	if strings.Contains(string(payload), "tool_calls") {
		t.Fatalf("Payload did not sanitize: %s", payload)
	}
	if strings.HasPrefix(string(payload), "data:") {
		t.Fatalf("payload must stay bare: %s", payload)
	}
}
