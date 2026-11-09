package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// captainConfig writes a copy of the repository's own configuration with the
// captain block replaced by the given value (nil removes it), so the test runs
// against a file the server would really accept.
func captainConfig(t *testing.T, captain any) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "config", "server.json"))
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if captain == nil {
		delete(body, "captain")
	} else {
		body["captain"] = captain
	}
	out, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "server.json")
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// A configuration written before the Captain battle existed carries no captain
// block; the announcement delay still has to be the measured default.
func TestCaptainDelayDefaults(t *testing.T) {
	cfg, err := Load(captainConfig(t, nil))
	if err != nil {
		t.Fatalf("load without a captain block: %v", err)
	}
	if cfg.Captain.AppointDelayMs != defaultCaptainDelay {
		t.Fatalf("appointDelayMs = %d, want %d", cfg.Captain.AppointDelayMs, defaultCaptainDelay)
	}
}

// A stated delay is kept, and a value a typo would produce is refused before the
// server starts.
func TestCaptainDelayValidation(t *testing.T) {
	cfg, err := Load(captainConfig(t, map[string]any{"appointDelayMs": 1}))
	if err != nil {
		t.Fatalf("load with appointDelayMs 1: %v", err)
	}
	if cfg.Captain.AppointDelayMs != 1 {
		t.Fatalf("appointDelayMs = %d, want 1", cfg.Captain.AppointDelayMs)
	}
	if _, err := Load(captainConfig(t, map[string]any{"appointDelayMs": 9999})); err == nil {
		t.Fatal("appointDelayMs 9999 must be refused")
	}
}
