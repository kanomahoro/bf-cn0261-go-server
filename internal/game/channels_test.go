package game

import (
	"testing"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/world"
)

// captainTestMaps is one normal-mode map, which is the whole pool the Captain
// battle needs.
func captainTestMaps() world.Maps {
	return world.Maps{Maps: []world.Map{{
		ResourceName: "rel_Test", Theme: "All", NormalMode: true, ItemMode: true,
		TimeLimitMilliseconds: 240000, ScoreLimit: 20, TeamSpawnCount: 8,
	}}}
}

// captainTestSetting is a create request for one mode pinned to that map.
func captainTestSetting(mode uint32) protocol.GameSetting {
	var setting protocol.GameSetting
	setting.Mode = mode
	setting.Strings16[0] = protocol.U16String("room")
	setting.Strings16[1] = protocol.U16String("rel_Test")
	setting.Strings16[2] = protocol.U16String("All")
	setting.Strings16[3] = protocol.U16String("rel_Test")
	return setting
}

// The Captain battle is admitted like the other competitive modes, and its rules
// are the ordinary ones: no clock or score is forced on it.
func TestCaptainModeIsAdmitted(t *testing.T) {
	normalized, err := normalizeGameSetting(captainTestSetting(captainModeID), captainTestMaps())
	if err != nil {
		t.Fatalf("mode 20 refused: %v", err)
	}
	if normalized.Mode != captainModeID {
		t.Fatalf("mode = %d, want %d", normalized.Mode, captainModeID)
	}
	if normalized.Values128[2] != 240000 || normalized.Values128[0] != 20 {
		t.Fatalf("rules = %v, want the map's 240000/20", normalized.Values128)
	}
	if protocol.FromU16(normalized.Strings16[1]) != "rel_Test" {
		t.Fatalf("map = %q, want rel_Test", protocol.FromU16(normalized.Strings16[1]))
	}
}

// A mode this server does not play is still refused, so admitting the Captain
// battle did not widen the door for the rest.
func TestUnsupportedModeStillRefused(t *testing.T) {
	if _, err := normalizeGameSetting(captainTestSetting(24), captainTestMaps()); err == nil {
		t.Fatal("mode 24 must stay unsupported")
	}
}
