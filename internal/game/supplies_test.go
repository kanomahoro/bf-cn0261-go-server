package game

import (
	"testing"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/world"
)

// testMaps is a two-map catalog: one that runs both modes and one that runs the
// normal mode only.
func testMaps() world.Maps {
	return world.Maps{
		DefaultRulesMap: "rel_Test_Normal",
		Maps: []world.Map{
			{ResourceName: "rel_Test_Item", Theme: "Village", NormalMode: true, ItemMode: true,
				TimeLimitMilliseconds: 240000, ScoreLimit: 20, TeamSpawnCount: 8},
			{ResourceName: "rel_Test_Normal", Theme: "Village", NormalMode: true, ItemMode: false,
				TimeLimitMilliseconds: 240000, ScoreLimit: 20, TeamSpawnCount: 8},
		},
	}
}

func itemRequest(mapName string) protocol.GameSetting {
	var request protocol.GameSetting
	request.Mode = itemModeID
	request.Strings16[0] = protocol.U16String("items")
	request.Strings16[1] = protocol.U16String(mapName)
	request.Strings16[2] = protocol.U16String("Village")
	request.Strings16[3] = protocol.U16String(mapName)
	return request
}

func TestNormalizeGameSettingAcceptsItemMode(t *testing.T) {
	setting, err := normalizeGameSetting(itemRequest("rel_Test_Item"), testMaps())
	if err != nil {
		t.Fatalf("item mode rejected: %v", err)
	}
	if setting.Mode != itemModeID {
		t.Fatalf("mode = %d, want %d", setting.Mode, itemModeID)
	}
	if got := protocol.FromU16(setting.Strings16[1]); got != "rel_Test_Item" {
		t.Fatalf("map = %q", got)
	}
	if setting.Values128[2] != 240000 {
		t.Fatalf("time limit = %d", setting.Values128[2])
	}
}

func TestNormalizeGameSettingRejectsItemModeWithoutItemMap(t *testing.T) {
	if _, err := normalizeGameSetting(itemRequest("rel_Test_Normal"), testMaps()); err == nil {
		t.Fatal("a map without the item flag must be refused in mode 3")
	}
}

func TestNormalizeGameSettingStillRejectsOtherModes(t *testing.T) {
	request := itemRequest("rel_Test_Item")
	for _, mode := range []uint32{8, 13, 16, 101} {
		request.Mode = mode
		if _, err := normalizeGameSetting(request, testMaps()); err == nil {
			t.Fatalf("mode %d must stay refused until its round is ported", mode)
		}
	}
}

func TestItemModeKinds(t *testing.T) {
	for _, kind := range []uint32{0, 101, 102, 108, 109, 115, 120, 123, 127, 128, 130, 136, 137, 138, 139, 141} {
		if !itemModeAccepts(kind) {
			t.Fatalf("kind %d must be offered", kind)
		}
	}
	for _, kind := range []uint32{103, 116, 133, 229, 1000} {
		if itemModeAccepts(kind) {
			t.Fatalf("kind %d is not in the cn-717 item sheet", kind)
		}
	}
}

func TestSupplyIntervalRespectsBounds(t *testing.T) {
	point := protocol.SupplyReportPoint{MinMs: 1000, MaxMs: 1200}
	for i := 0; i < 200; i++ {
		delay := supplyInterval(point)
		if delay < time.Second || delay > 1200*time.Millisecond {
			t.Fatalf("interval %v outside [1s, 1.2s]", delay)
		}
	}
	// A point that states no interval gets the reference defaults.
	if delay := supplyInterval(protocol.SupplyReportPoint{}); delay < defaultSupplyMinMs*time.Millisecond ||
		delay > defaultSupplyMaxMs*time.Millisecond {
		t.Fatalf("default interval %v", delay)
	}
}
