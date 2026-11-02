package config

import (
	"encoding/json"
	"os"
	"testing"
)

// An old configuration file, the one the 2026-11-02 builds shipped with its
// itemMode block, must still load: the block is accepted and ignored instead of
// failing as an unknown key. The behaviour those switches selected is now fixed.
func TestLoadAcceptsTheDeprecatedItemModeBlock(t *testing.T) {
	raw, err := os.ReadFile("../../config/server.json")
	if err != nil {
		t.Fatalf("read the shipped configuration: %v", err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("decode the shipped configuration: %v", err)
	}
	document["itemMode"] = json.RawMessage(`{
		"disableMapReportSpawns": false,
		"enforceBoxColour": false,
		"serverGrantsItem": true,
		"acceptRandomBoxDrops": false
	}`)
	old, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("encode the old configuration: %v", err)
	}
	path := t.TempDir() + "/server.json"
	if err := os.WriteFile(path, old, 0o600); err != nil {
		t.Fatalf("write the old configuration: %v", err)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("an old configuration must still load: %v", err)
	}
}
