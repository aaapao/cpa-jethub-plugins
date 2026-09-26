package main

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

// expiryFromTokenMillis returns the access token's own expiry, read from the
// JWT `exp` claim.
//
// Cline's WorkOS access token is a JWT, while neither `/api/v1/auth/refresh` nor
// `/api/v1/auth/register` answers with an absolute expiry. Without this the
// credential keeps the expiry it was first written with, so a token that has
// actually been renewed an hour ago still reads as expired and the plugin renews
// it again on every single request.
//
// A token that is not a JWT, or whose payload cannot be read, yields 0: an
// expiry is never fabricated.
func expiryFromTokenMillis(token string) int64 {
	claims := jwtClaims(token)
	if claims == nil {
		return 0
	}
	return secondsClaimToMillis(claims["exp"])
}

// jwtClaims decodes the payload segment of a JWT. A scheme prefix such as
// `workos:` is dropped first: the credential stores the prefixed form, and the
// prefix carries no meaning for the token itself.
func jwtClaims(token string) map[string]any {
	trimmed := strings.TrimSpace(token)
	if index := strings.LastIndex(trimmed, ":"); index >= 0 {
		trimmed = trimmed[index+1:]
	}
	segments := strings.Split(trimmed, ".")
	if len(segments) < 2 {
		return nil
	}
	// JWT segments are base64url without padding; tolerate padding anyway.
	payload, errDecode := base64.RawURLEncoding.DecodeString(strings.TrimRight(segments[1], "="))
	if errDecode != nil {
		return nil
	}
	var claims map[string]any
	if errUnmarshal := json.Unmarshal(payload, &claims); errUnmarshal != nil {
		return nil
	}
	return claims
}

// secondsClaimToMillis converts a JWT-style seconds claim into a millisecond
// epoch. Values that are not a positive number yield 0.
func secondsClaimToMillis(value any) int64 {
	var seconds float64
	switch typed := value.(type) {
	case float64:
		seconds = typed
	case json.Number:
		parsed, errNumber := typed.Float64()
		if errNumber != nil {
			return 0
		}
		seconds = parsed
	case string:
		parsed, errParse := parseFloat(typed)
		if errParse != nil {
			return 0
		}
		seconds = parsed
	default:
		return 0
	}
	if seconds <= 0 {
		return 0
	}
	return int64(seconds * 1000)
}

// relativeExpiryMillis converts a vendor `expiresIn` / `expires_in` (seconds,
// relative to now) into an absolute millisecond epoch. It is the last resort
// after the token's own `exp` claim, which describes the token rather than the
// response that carried it.
func relativeExpiryMillis(payload map[string]any, now time.Time) int64 {
	for _, key := range []string{"expiresIn", "expires_in"} {
		value, ok := payload[key]
		if !ok {
			continue
		}
		var seconds float64
		switch typed := value.(type) {
		case float64:
			seconds = typed
		case json.Number:
			parsed, errNumber := typed.Float64()
			if errNumber != nil {
				continue
			}
			seconds = parsed
		case string:
			parsed, errParse := parseFloat(typed)
			if errParse != nil {
				continue
			}
			seconds = parsed
		default:
			continue
		}
		if seconds <= 0 {
			continue
		}
		return now.Add(time.Duration(seconds * float64(time.Second))).UnixMilli()
	}
	return 0
}

func parseFloat(value string) (float64, error) {
	var parsed float64
	if errUnmarshal := json.Unmarshal([]byte(strings.TrimSpace(value)), &parsed); errUnmarshal != nil {
		return 0, errUnmarshal
	}
	return parsed, nil
}
