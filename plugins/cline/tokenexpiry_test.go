package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// jwtFor builds a token shaped like the vendor's: three base64url segments whose
// payload is the given claims. The signature is never verified by a reader.
func jwtFor(t *testing.T, claims map[string]any) string {
	t.Helper()
	encode := func(value map[string]any) string {
		raw, errMarshal := json.Marshal(value)
		if errMarshal != nil {
			t.Fatalf("marshal segment: %v", errMarshal)
		}
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	return encode(map[string]any{"alg": "RS256", "typ": "JWT"}) + "." + encode(claims) + ".c2ln"
}

// The vendor's refresh and register answers carry no absolute expiry, so the
// token's own `exp` claim is the only source of truth; missing it is what made
// the credential renew on every request.
func TestExpiryFromTokenMillisReadsJWTExp(t *testing.T) {
	token := jwtFor(t, map[string]any{"exp": 1790453215, "iat": 1790449615})
	if got := expiryFromTokenMillis(token); got != 1790453215000 {
		t.Errorf("expiry = %d, want 1790453215000", got)
	}
	if got := expiryFromTokenMillis(TokenPrefix + token); got != 1790453215000 {
		t.Errorf("prefixed expiry = %d, want 1790453215000", got)
	}
}

func TestExpiryFromTokenMillisRefusesToFabricate(t *testing.T) {
	cases := map[string]string{
		"empty":          "",
		"opaque":         "not-a-jwt-at-all",
		"two segments":   "aGVhZGVy.cGF5bG9hZA",
		"payload not":    "aGVhZGVy.bm90LWpzb24.c2ln",
		"no exp":         jwtFor(t, map[string]any{"iat": 1}),
		"exp zero":       jwtFor(t, map[string]any{"exp": 0}),
		"exp not number": jwtFor(t, map[string]any{"exp": "later"}),
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			if got := expiryFromTokenMillis(token); got != 0 {
				t.Errorf("expiry = %d, want 0", got)
			}
		})
	}
}

func TestParseTokenEnvelopeTakesExpiryFromToken(t *testing.T) {
	token := jwtFor(t, map[string]any{"exp": 1790453215})
	body := fmt.Sprintf(`{"success":true,"data":{"accessToken":"%s%s","refreshToken":"r"}}`, TokenPrefix, token)

	tokens := parseTokenEnvelope([]byte(body))
	if tokens.ExpireTime != 1790453215000 {
		t.Errorf("ExpireTime = %d, want the token's exp", tokens.ExpireTime)
	}
}

// An explicit expiry in the answer still wins: the vendor describing its own
// response beats reading the token.
func TestParseTokenEnvelopePrefersExplicitExpiry(t *testing.T) {
	token := jwtFor(t, map[string]any{"exp": 1790453215})
	body := fmt.Sprintf(`{"data":{"accessToken":"%s%s","refreshToken":"r","expiresAt":1790999999000}}`, TokenPrefix, token)

	tokens := parseTokenEnvelope([]byte(body))
	if tokens.ExpireTime != 1790999999000 {
		t.Errorf("ExpireTime = %d, want the explicit expiry", tokens.ExpireTime)
	}
}

func TestParseTokenEnvelopeFallsBackToExpiresIn(t *testing.T) {
	now := time.Now()
	tokens := parseTokenEnvelope([]byte(`{"data":{"accessToken":"opaque-token","refreshToken":"r","expires_in":3600}}`))

	want := now.Add(time.Hour).UnixMilli()
	if difference := tokens.ExpireTime - want; difference > 5000 || difference < -5000 {
		t.Errorf("ExpireTime = %d, want ≈%d", tokens.ExpireTime, want)
	}
}

// The bug this guards: a renewal that leaves the old expiry in place.
func TestApplyRefreshMovesTheExpiryForward(t *testing.T) {
	token := jwtFor(t, map[string]any{"exp": 1790453215})
	existing := &Credential{
		AccessToken:  TokenPrefix + "old",
		RefreshToken: "refresh",
		ExpireTime:   1700000000000,
		AccountID:    "usr-1",
		Email:        "user@example.com",
	}

	renewed := existing.applyRefresh(clineTokens{
		AccessToken:  TokenPrefix + token,
		RefreshToken: "refresh",
		ExpireTime:   expiryFromTokenMillis(token),
	})

	if renewed.ExpireTime != 1790453215000 {
		t.Errorf("ExpireTime = %d, want the renewed token's expiry", renewed.ExpireTime)
	}
	if renewed.AccountID != "usr-1" || renewed.Email != "user@example.com" {
		t.Errorf("renewal dropped account fields: %+v", renewed)
	}
}
