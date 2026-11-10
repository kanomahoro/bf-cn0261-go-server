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

// TestBombCreateReplacesAMapOutsideItsPool pins the fix for the client's create
// dialog starting on the startup map: a bomb room built with an ordinary
// competitive map is moved onto the mission's own pool instead of being refused
// or, worse, started on a map the mode cannot run.
func TestBombCreateReplacesAMapOutsideItsPool(t *testing.T) {
	maps := world.Maps{Maps: []world.Map{{
		ResourceName: "rel_Village01_Byway", Theme: "Village", NormalMode: true, ItemMode: true,
		TimeLimitMilliseconds: 240000, ScoreLimit: 20, TeamSpawnCount: 8,
	}}}
	var request protocol.GameSetting
	request.Mode = bombModeID
	request.Strings16[0] = protocol.U16String("room")
	request.Strings16[1] = protocol.U16String("rel_Village01_Byway")
	request.Strings16[2] = protocol.U16String("All")
	request.Strings16[3] = protocol.U16String("rel_Village01_Byway")

	setting, err := normalizeGameSetting(request, maps)
	if err != nil {
		t.Fatalf("a bomb room built from the startup map was refused: %v", err)
	}
	mapName := protocol.FromU16(setting.Strings16[1])
	if !maps.IsBombMap(mapName) {
		t.Fatalf("the bomb room kept a map outside its pool: %s", mapName)
	}
}

// TestBombCreateKeepsAMapInsideItsPool checks the replacement only happens for a
// map the mission cannot run.
func TestBombCreateKeepsAMapInsideItsPool(t *testing.T) {
	maps := world.Maps{}
	var request protocol.GameSetting
	request.Mode = bombModeID
	request.Strings16[0] = protocol.U16String("room")
	request.Strings16[1] = protocol.U16String("rel_Village02_waterPark")
	request.Strings16[2] = protocol.U16String("All")
	request.Strings16[3] = protocol.U16String("rel_Village02_waterPark")

	setting, err := normalizeGameSetting(request, maps)
	if err != nil {
		t.Fatalf("a bomb map was refused: %v", err)
	}
	if got := protocol.FromU16(setting.Strings16[1]); got != "rel_Village02_waterPark" {
		t.Fatalf("the bomb room's own map was replaced by %s", got)
	}
}

// TestModeOnlyMapsAreAdmittedByTheirOwnMode checks the two maps the client lists
// in one mode only: a zombie room may run rel_Stadium03_StardustArena, a sniper
// room may run rel_pirate02_frontal, and the competitive modes still refuse both.
func TestModeOnlyMapsAreAdmittedByTheirOwnMode(t *testing.T) {
	maps, err := world.LoadMaps("../../config/game_maps.json")
	if err != nil {
		t.Fatalf("the shipped map catalog does not load: %v", err)
	}
	admit := func(mode uint32, name string) error {
		var request protocol.GameSetting
		request.Mode = mode
		request.Strings16[0] = protocol.U16String("room")
		request.Strings16[1] = protocol.U16String(name)
		request.Strings16[2] = protocol.U16String("All")
		request.Strings16[3] = protocol.U16String(name)
		_, err := normalizeGameSetting(request, maps)
		return err
	}
	if err := admit(zombieModeID, "rel_Stadium03_StardustArena"); err != nil {
		t.Fatalf("the zombie battle refused its own map: %v", err)
	}
	if err := admit(sniperModeID, "rel_pirate02_frontal"); err != nil {
		t.Fatalf("the sniper battle refused its own map: %v", err)
	}
	if err := admit(1, "rel_Stadium03_StardustArena"); err == nil {
		t.Fatal("the competitive battle admitted a zombie-only map")
	}
	if err := admit(1, "rel_pirate02_frontal"); err == nil {
		t.Fatal("the competitive battle admitted a sniper-only map")
	}
}

// TestBombRoomKeepsTheRequestedTheme pins the map dialog fix: a bomb room used to
// rewrite its theme slot to the chosen map's theme, which left the client's map
// dialog on that tab (three Village maps) until the "all" tab refreshed six.
func TestBombRoomKeepsTheRequestedTheme(t *testing.T) {
	maps := world.Maps{}
	var request protocol.GameSetting
	request.Mode = bombModeID
	request.Strings16[0] = protocol.U16String("room")
	request.Strings16[1] = protocol.U16String("rel_Village02_waterPark")
	request.Strings16[2] = protocol.U16String("All")
	request.Strings16[3] = protocol.U16String("rel_Village02_waterPark")

	setting, err := normalizeGameSetting(request, maps)
	if err != nil {
		t.Fatalf("a bomb room was refused: %v", err)
	}
	if got := protocol.FromU16(setting.Strings16[2]); got != "All" {
		t.Fatalf("the bomb room rewrote its theme to %q, want All", got)
	}
}

// TestOtherRoomsKeepTheChosenMapsTheme checks the fresh echo is scoped to the bomb
// mission: every other mode still states the theme of the map it resolved.
func TestOtherRoomsKeepTheChosenMapsTheme(t *testing.T) {
	maps := world.Maps{Maps: []world.Map{{
		ResourceName: "rel_Village01_Byway", Theme: "Village", NormalMode: true, ItemMode: true,
		TimeLimitMilliseconds: 240000, ScoreLimit: 20, TeamSpawnCount: 8,
	}}}
	var request protocol.GameSetting
	request.Mode = 1
	request.Strings16[0] = protocol.U16String("room")
	request.Strings16[1] = protocol.U16String("rel_Village01_Byway")
	request.Strings16[2] = protocol.U16String("All")
	request.Strings16[3] = protocol.U16String("rel_Village01_Byway")

	setting, err := normalizeGameSetting(request, maps)
	if err != nil {
		t.Fatalf("a competitive room was refused: %v", err)
	}
	if got := protocol.FromU16(setting.Strings16[2]); got != "Village" {
		t.Fatalf("the competitive room states theme %q, want Village", got)
	}
}
