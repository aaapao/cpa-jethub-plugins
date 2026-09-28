package sse

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// The two JSON members the sanitizer reasons about: a chunk's incremental
// assistant message, and the tool-call fragments it may carry.
const (
	deltaMember     = "delta"
	toolCallsMember = "tool_calls"
)

// SanitizePayload removes `delta.tool_calls` members whose value is an EMPTY
// array from one chat-completion chunk.
//
// Upstreams disagree about how to spell "this frame carries no tool call".
// CodeBuddy — and every gateway that re-exposes its wire format — sends
// `"tool_calls":[]` on each reasoning frame, while others omit the member
// entirely or send `null`.
//
// An empty array is not harmless. The Vercel AI SDK's openai-compatible
// provider ends the active reasoning segment whenever `delta.tool_calls` is not
// null — `[]` included — and the following reasoning frame starts a fresh one
// (`@ai-sdk/openai-compatible`, `dist/index.js`):
//
//	if (delta.tool_calls != null) {
//	  if (isActiveReasoning) controller.enqueue({ type: "reasoning-end", … });
//	  for (const toolCallDelta of delta.tool_calls) { … }   // empty: no-op
//	}
//
// One reasoning segment per token is what renders as "one word per row" in
// clients that lay out reasoning segments as separate blocks (ZCode shows one
// `思考` row per segment). Clients that accumulate reasoning into a single block
// (pi-ai, used by DSH) or that smooth incoming chunks (markstream-vue, used by
// Kimi Code) never surfaced the problem, which is why it looked client-specific.
//
// Deleting the member is semantically free: an empty array carries exactly as
// much information as a missing one. A frame that DOES carry tool calls is left
// untouched, as is every other member — the payload is spliced in place rather
// than re-marshalled, so field order, number formatting (a re-marshal through
// `map[string]any` would turn `"created":1790561792` into `1.79e+09`) and
// vendor-specific extras survive byte for byte.
func SanitizePayload(payload string) string {
	// Fast path: most upstreams never mention tool calls in a reasoning frame,
	// and a substring probe is far cheaper than walking the JSON.
	if !strings.Contains(payload, toolCallsMember) {
		return payload
	}
	spans, ok := emptyToolCallSpans([]byte(payload))
	if !ok || len(spans) == 0 {
		return payload
	}
	return string(spliceOutSpans([]byte(payload), spans))
}

// emptyToolCallSpans returns the half-open byte ranges covering every
// `…delta.tool_calls` member whose value is an empty array, in document order.
//
// The boolean is false when the payload is not a JSON value this walker can
// traverse, in which case the caller must relay the payload untouched: a frame
// that cannot be parsed is not a frame this sanitizer may rewrite.
func emptyToolCallSpans(data []byte) ([][2]int, bool) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	walker := &spanWalker{decoder: decoder, data: data}
	if errWalk := walker.value(); errWalk != nil {
		return nil, false
	}
	return walker.spans, true
}

// spanWalker streams one JSON document, tracking the member path it is inside so
// it can recognise `delta.tool_calls` without decoding the whole document into
// `any` (which would lose the original byte offsets).
type spanWalker struct {
	decoder *json.Decoder
	data    []byte
	path    []string
	spans   [][2]int
}

// value consumes exactly one JSON value.
func (w *spanWalker) value() error {
	token, errToken := w.decoder.Token()
	if errToken != nil {
		return errToken
	}
	delim, isDelim := token.(json.Delim)
	if !isDelim {
		return nil
	}
	switch delim {
	case '{':
		return w.object()
	case '[':
		return w.array()
	}
	// A stray '}' or ']' is the enclosing container's own terminator.
	return nil
}

// object consumes the body of an object whose '{' has already been read,
// recording one span per empty `delta.tool_calls` member.
func (w *spanWalker) object() error {
	for w.decoder.More() {
		// The decoder sits on the comma that separates this member from the
		// previous one, so step over it and any spacing to reach the key's
		// opening quote. Anchoring the span there (rather than on the comma)
		// keeps the comma bookkeeping in spliceOutSpans in one place.
		keyStart := skipMemberSeparator(w.data, int(w.decoder.InputOffset()))
		keyToken, errKey := w.decoder.Token()
		if errKey != nil {
			return errKey
		}
		key, isKey := keyToken.(string)
		if !isKey {
			return errNotJSON
		}
		keyEnd := int(w.decoder.InputOffset())
		parentIsDelta := len(w.path) > 0 && w.path[len(w.path)-1] == deltaMember
		w.path = append(w.path, key)
		if errValue := w.value(); errValue != nil {
			return errValue
		}
		w.path = w.path[:len(w.path)-1]
		if !parentIsDelta || key != toolCallsMember {
			continue
		}
		// The value just consumed ends here, so the member's span is
		// [keyStart, valueEnd). The decoder consumed the colon internally but
		// InputOffset still reports the offset just past the key, so step over
		// the colon and any spacing to find where the value begins.
		valueEnd := int(w.decoder.InputOffset())
		valueStart := skipValueStart(w.data, keyEnd)
		if valueStart < 0 || valueEnd > len(w.data) || valueStart >= valueEnd {
			continue
		}
		if !isEmptyArrayValue(w.data[valueStart:valueEnd]) {
			continue
		}
		w.spans = append(w.spans, [2]int{keyStart, valueEnd})
	}
	// Consume the closing '}'.
	_, errClose := w.decoder.Token()
	return errClose
}

// array consumes the body of an array whose '[' has already been read. Elements
// push a positional placeholder so the path length stays meaningful.
func (w *spanWalker) array() error {
	for index := 0; w.decoder.More(); index++ {
		w.path = append(w.path, strconv.Itoa(index))
		if errValue := w.value(); errValue != nil {
			return errValue
		}
		w.path = w.path[:len(w.path)-1]
	}
	_, errClose := w.decoder.Token()
	return errClose
}

// spliceOutSpans deletes each span plus exactly one adjacent comma so the
// result stays valid JSON. Spans are in document order and therefore never
// nested, which makes a single forward pass sufficient.
func spliceOutSpans(data []byte, spans [][2]int) []byte {
	out := make([]byte, 0, len(data))
	cursor := 0
	for _, span := range spans {
		start, end := span[0], span[1]
		if start < cursor || end > len(data) {
			continue
		}
		// Prefer the comma that precedes the member. When the member comes first
		// in its object there is none, so swallow the one that follows instead.
		cutStart := start
		back := start - 1
		for back >= cursor && isJSONSpace(data[back]) {
			back--
		}
		if back >= cursor && data[back] == ',' {
			cutStart = back
		} else {
			forward := end
			for forward < len(data) && isJSONSpace(data[forward]) {
				forward++
			}
			if forward < len(data) && data[forward] == ',' {
				end = forward + 1
			}
		}
		out = append(out, data[cursor:cutStart]...)
		cursor = end
	}
	return append(out, data[cursor:]...)
}

// errNotJSON reports a token stream that does not match the JSON grammar.
var errNotJSON = errUnparsable("sse: payload is not a JSON object")

type errUnparsable string

func (e errUnparsable) Error() string { return string(e) }

// isEmptyArrayValue reports whether raw is an array with no elements, tolerating
// whitespace inside the brackets (`[ ]` is a legal spelling of `[]`).
func isEmptyArrayValue(raw []byte) bool {
	inner := bytes.TrimSpace(raw)
	if len(inner) < 2 || inner[0] != '[' || inner[len(inner)-1] != ']' {
		return false
	}
	return len(bytes.TrimSpace(inner[1:len(inner)-1])) == 0
}

// skipMemberSeparator returns the offset of the first byte that begins an
// object member's key, starting from the comma (or the opening brace) the
// decoder stopped on.
func skipMemberSeparator(data []byte, from int) int {
	from = skipSpaceForward(data, from)
	if from < len(data) && data[from] == ',' {
		from = skipSpaceForward(data, from+1)
	}
	return from
}

// skipSpaceForward returns the offset of the first non-space byte at or after
// from, or len(data) when only spaces follow.
func skipSpaceForward(data []byte, from int) int {
	for from < len(data) && isJSONSpace(data[from]) {
		from++
	}
	return from
}

// skipValueStart returns the offset of the first byte of a member's value,
// given the offset just past its key. It steps over the colon and any spacing.
// The result is -1 when the byte stream does not look like `key : value`.
func skipValueStart(data []byte, keyEnd int) int {
	from := skipSpaceForward(data, keyEnd)
	if from >= len(data) || data[from] != ':' {
		return -1
	}
	return skipSpaceForward(data, from+1)
}

// isJSONSpace reports whether b is whitespace as defined by RFC 8259.
func isJSONSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}
