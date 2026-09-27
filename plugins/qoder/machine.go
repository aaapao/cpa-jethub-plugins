package main

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
)

// MachineIdentity is the device identity the Qoder server expects on
// `/sash/` endpoints and on the encrypted inference chain:
// the pair must be sent together (`Cosy-MachineToken` + `Cosy-MachineType`)
// or the server treats the caller as an unidentified client — measured by the
// Jet-Hub reference ablation: with the pair the campaigns list gains the daily
// `CLAIM_BENEFIT/100` entry, without it only a `VIEW_DETAILS` promo remains.
//
// The values come from the official client's `machine_token.json`, produced by
// its UMID subsystem (`runtime-info.exe`), which this plugin cannot replicate.
// The file is stable — the reference measured a 179-day-old token still working.
type MachineIdentity struct {
	Token string
	Type  string
}

var (
	machineMu       sync.RWMutex
	machineResolved *MachineIdentity // nil = not resolved yet; non-nil without Token = resolved but absent
)

// machineIdentityPath returns the configured override when set, otherwise the
// conventional location next to the CPA auth files.
func machineIdentityPath(cfg Config) string {
	if trimmed := strings.TrimSpace(cfg.MachineTokenPath); trimmed != "" {
		return trimmed
	}
	return ""
}

// loadMachineIdentity reads and validates one machine_token.json.
func loadMachineIdentity(path string) (*MachineIdentity, error) {
	raw, errRead := os.ReadFile(path)
	if errRead != nil {
		return nil, errRead
	}
	var parsed struct {
		Token string `json:"token"`
		Type  string `json:"type"`
	}
	if errUnmarshal := json.Unmarshal(raw, &parsed); errUnmarshal != nil {
		return nil, errUnmarshal
	}
	if strings.TrimSpace(parsed.Token) == "" || strings.TrimSpace(parsed.Type) == "" {
		return nil, os.ErrInvalid
	}
	return &MachineIdentity{Token: strings.TrimSpace(parsed.Token), Type: strings.TrimSpace(parsed.Type)}, nil
}

// machineIdentityFor resolves the device identity for the configured path,
// caching the outcome for the process lifetime (the file rarely changes and is
// read on every request otherwise).
//
// A missing or malformed file yields a nil identity: callers then send requests
// without the machine headers, which matches the pre-fix behaviour instead of
// failing the whole channel.
func machineIdentityFor(cfg Config) *MachineIdentity {
	path := machineIdentityPath(cfg)
	machineMu.RLock()
	resolved := machineResolved
	machineMu.RUnlock()
	if resolved != nil {
		return resolved
	}

	machineMu.Lock()
	defer machineMu.Unlock()
	if machineResolved != nil {
		return machineResolved
	}
	if path == "" {
		machineResolved = &MachineIdentity{}
		return machineResolved
	}
	identity, errLoad := loadMachineIdentity(path)
	if errLoad != nil || identity == nil {
		machineResolved = &MachineIdentity{}
		return machineResolved
	}
	machineResolved = identity
	return machineResolved
}

// applyMachineHeaders overlays the device-identity headers onto an outgoing
// header set. The encrypted path gets its headers from the WASM, which fills
// `Cosy-MachineToken` with the session machine_id and `Cosy-MachineType` with a
// constant — both are placeholders the server does not credit, so the official
// identity replaces them when known.
func applyMachineHeaders(headers map[string]string, cfg Config) {
	identity := machineIdentityFor(cfg)
	if identity == nil || identity.Token == "" {
		return
	}
	headers["Cosy-MachineToken"] = identity.Token
	headers["Cosy-MachineType"] = identity.Type
}
