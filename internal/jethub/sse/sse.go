// Package sse implements the Server-Sent Events framing used by the CodeArts
// Snap-Access chat endpoint and by CPA when streaming Chat Completions back to
// a client.
package sse

import (
	"bytes"
	"encoding/json"
	"strings"
)

// Done is the terminal payload of an OpenAI-style SSE stream.
const Done = "[DONE]"

// Scanner incrementally extracts `data:` payloads from a byte stream that may
// be split at arbitrary boundaries.
type Scanner struct {
	buf []byte
}

// Feed appends upstream bytes and returns every complete `data:` payload.
// Payloads are returned verbatim (without the `data:` prefix or trailing
// newline) so callers can compare against Done or unmarshal JSON.
func (s *Scanner) Feed(chunk []byte) []string {
	if len(chunk) > 0 {
		s.buf = append(s.buf, chunk...)
	}
	var events []string
	for {
		idx := bytes.IndexByte(s.buf, '\n')
		if idx < 0 {
			break
		}
		line := strings.TrimRight(string(s.buf[:idx]), "\r")
		s.buf = s.buf[idx+1:]
		if payload, ok := parseDataLine(line); ok {
			events = append(events, payload)
		}
	}
	return events
}

// Buffered reports whether an incomplete line is still pending.
func (s *Scanner) Buffered() bool { return len(bytes.TrimSpace(s.buf)) > 0 }

// parseDataLine recognises an SSE data field, tolerating the optional single
// space after the colon that the spec allows.
func parseDataLine(line string) (string, bool) {
	if !strings.HasPrefix(line, "data:") {
		return "", false
	}
	return strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "), true
}

// Payload returns one stream chunk payload exactly as the host expects it: the
// bare bytes, with no SSE framing of their own.
//
// The host frames every plugin chunk it forwards (`data: %s\n\n`) and writes the
// terminal `data: [DONE]` itself. A payload that already carries a `data:`
// prefix therefore reaches the client as `data: data: {…}`, and a client reading
// SSE then fails to parse that frame as JSON.
//
// The payload is also passed through SanitizePayload, which drops a `tool_calls`
// member whose value is an empty array. Every passthrough executor funnels its
// upstream frames through here, so this is the one place that has to know about
// the empty-array convention; see SanitizePayload for why leaving it in place
// splits a reasoning stream into one segment per token downstream.
func Payload(payload string) []byte {
	return []byte(SanitizePayload(payload))
}

// PayloadJSON marshals v into one stream chunk payload (bare, see Payload).
func PayloadJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return raw, nil
}
