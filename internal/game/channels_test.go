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

// TestBombRoomAcceptsItsOwnMaps checks that a bomb room can pin every map of its
// pool: two of them carry neither the normal nor the item flag in MapMeta.
func TestBombRoomAcceptsItsOwnMaps(t *testing.T) {
	maps := world.Maps{}
	for _, name := range []string{"rel_Sudden06_Tank", "rel_zombie_01_bomb", "rel_Village03_CityHall"} {
		request := protocol.GameSetting{
			Mode:      bombModeID,
			Strings16: [4][]uint16{protocol.U16String("room"), protocol.U16String(name), protocol.U16String("All"), protocol.U16String(name)},
			Values128: [5]uint32{20, 0, 240000, 0, 1},
			Values148: [8]uint32{20, 0, 240000, 0, 1, 0xFFFFFFFF},
		}
		setting, err := normalizeGameSetting(request, maps)
		if err != nil {
			t.Fatalf("%s was refused: %v", name, err)
		}
		if protocol.FromU16(setting.Strings16[1]) != name {
			t.Fatalf("%s was not pinned: %q", name, protocol.FromU16(setting.Strings16[1]))
		}
	}
}

// TestBombRoomRejectsACompetitiveOnlyMap checks that the bomb pool really is the
// bound: a map outside it is still refused.
func TestBombRoomRejectsACompetitiveOnlyMap(t *testing.T) {
	maps := world.Maps{}
	request := protocol.GameSetting{
		Mode:      bombModeID,
		Strings16: [4][]uint16{protocol.U16String("room"), protocol.U16String("rel_Apex01_Diorama"), protocol.U16String("All"), protocol.U16String("rel_Apex01_Diorama")},
		Values128: [5]uint32{20, 0, 240000, 0, 1},
		Values148: [8]uint32{20, 0, 240000, 0, 1, 0xFFFFFFFF},
	}
	if _, err := normalizeGameSetting(request, maps); err == nil {
		t.Fatal("a map outside the bomb pool was accepted")
	}
}
