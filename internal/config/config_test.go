package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeConfig puts a configuration file in a temporary directory and returns its
// path, so the loader can be exercised without the shipped file.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "server.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// A configuration written before the zombie mode existed has no zombie block. The
// waits are the mode's whole timing, so an absent block must fall back to the
// values measured on a real client instead of turning everyone at 0 ms.
func TestLoadDefaultsTheZombieWaits(t *testing.T) {
	cfg, err := Load(writeConfig(t, `{
		"gameMapsFile": "config/game_maps.json",
		"gameWeaponsFile": "config/game_weapons.json",
		"gameDefenseFile": "config/game_defense.json",
		"gameCharactersFile": "config/game_characters.json",
		"gameRespawnFile": "config/game_respawn.json",
		"progressionFile": "config/progression.json",
		"network": {"listenPort": 26100, "maxConnections": 8, "idleTimeoutSeconds": 60},
		"relay": {"enabled": false},
		"database": {"host": "127.0.0.1", "port": 3306, "databaseName": "bubblefighter_cn0261",
			"username": "bfserver", "connectTimeoutSeconds": 5, "queryTimeoutSeconds": 5,
			"poolSize": 2, "tlsMode": "disable"},
		"authentication": {"enabled": false},
		"logging": {"level": "info", "directory": "Logs", "maxFileSizeBytes": 5242880, "maxFiles": 3}
	}`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Zombie.HumanTurnMs != defaultHumanTurnMs || cfg.Zombie.ZombiniTurnMs != defaultZombiniTurnMs {
		t.Fatalf("zombie waits = %d/%d, want the measured defaults",
			cfg.Zombie.HumanTurnMs, cfg.Zombie.ZombiniTurnMs)
	}
}

// An explicit value is kept, and a typo outside the window is refused instead of
// quietly replacing the mode's timing.
func TestZombieWaitsAreValidated(t *testing.T) {
	body := func(human, zombini string) string {
		return `{
		"gameMapsFile": "config/game_maps.json",
		"gameWeaponsFile": "config/game_weapons.json",
		"gameDefenseFile": "config/game_defense.json",
		"gameCharactersFile": "config/game_characters.json",
		"gameRespawnFile": "config/game_respawn.json",
		"progressionFile": "config/progression.json",
		"network": {"listenPort": 26100, "maxConnections": 8, "idleTimeoutSeconds": 60},
		"relay": {"enabled": false},
		"database": {"host": "127.0.0.1", "port": 3306, "databaseName": "bubblefighter_cn0261",
			"username": "bfserver", "connectTimeoutSeconds": 5, "queryTimeoutSeconds": 5,
			"poolSize": 2, "tlsMode": "disable"},
		"authentication": {"enabled": false},
		"logging": {"level": "info", "directory": "Logs", "maxFileSizeBytes": 5242880, "maxFiles": 3},
		"zombie": {"humanTurnMs": ` + human + `, "zombiniTurnMs": ` + zombini + `}
	}`
	}

	cfg, err := Load(writeConfig(t, body("9000", "15000")))
	if err != nil {
		t.Fatalf("explicit waits rejected: %v", err)
	}
	if cfg.Zombie.HumanTurnMs != 9000 || cfg.Zombie.ZombiniTurnMs != 15000 {
		t.Fatalf("waits = %d/%d, want 9000/15000", cfg.Zombie.HumanTurnMs, cfg.Zombie.ZombiniTurnMs)
	}

	for _, bad := range [][2]string{{"10", "15000"}, {"9000", "120000"}} {
		if _, err := Load(writeConfig(t, body(bad[0], bad[1]))); err == nil {
			t.Fatalf("waits %s/%s must be refused", bad[0], bad[1])
		}
	}
}
