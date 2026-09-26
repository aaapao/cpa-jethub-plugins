package sse

import (
	"encoding/json"
	"strings"
	"testing"
)

// The host frames every plugin stream chunk it forwards (`data: %s\n\n`) and
// writes the terminal `data: [DONE]` itself. A payload that carried its own
// framing therefore reached the client as `data: data: {…}`, and every SSE
// client failed to parse that frame as JSON.
func TestPayloadIsBareAndJSONReady(t *testing.T) {
	payload := Payload(`{"id":"c1","object":"chat.completion.chunk"}`)
	text := string(payload)
	if strings.HasPrefix(text, "data:") || strings.HasSuffix(text, "\n") {
		t.Fatalf("payload must be bare, got %q", text)
	}
	var decoded map[string]any
	if errUnmarshal := json.Unmarshal(payload, &decoded); errUnmarshal != nil {
		t.Fatalf("payload is not JSON: %v (%q)", errUnmarshal, text)
	}
}

func TestPayloadJSONMarshalsWithoutFraming(t *testing.T) {
	payload, errPayload := PayloadJSON(map[string]any{"choices": []any{}})
	if errPayload != nil {
		t.Fatalf("PayloadJSON: %v", errPayload)
	}
	if got := string(payload); got != `{"choices":[]}` {
		t.Fatalf("payload = %q, want the marshalled JSON alone", got)
	}
}

// Scanning upstream frames still strips the prefix the vendor sent; only the
// outbound side is bare.
func TestScannerStillStripsUpstreamFraming(t *testing.T) {
	scanner := &Scanner{}
	events := scanner.Feed([]byte("data: {\"a\":1}\n\ndata: [DONE]\n\n"))
	if len(events) != 2 || events[0] != `{"a":1}` || events[1] != Done {
		t.Fatalf("events = %#v", events)
	}
}
