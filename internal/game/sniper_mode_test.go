package game

import (
	"testing"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/world"
)

func loadTestMaps(t *testing.T) world.Maps {
	t.Helper()
	maps, err := world.LoadMaps("../../config/game_maps.json")
	if err != nil {
		t.Fatalf("load maps: %v", err)
	}
	return maps
}

// The sniper battle is the ordinary competitive round: it must resolve a map
// from the no-item pool and keep the create request's clock and score limit.
func TestSniperModeResolvesToTheCompetitivePool(t *testing.T) {
	maps := loadTestMaps(t)

	random := protocol.GameSetting{Mode: sniperModeID}
	random.Strings16[1] = protocol.U16String("Random")
	random.Strings16[3] = protocol.U16String("Random")
	setting, err := normalizeGameSetting(random, maps)
	if err != nil {
		t.Fatalf("random sniper room rejected: %v", err)
	}
	if setting.Values128[2] != 240000 {
		t.Errorf("clock = %d, want the map default 240000", setting.Values128[2])
	}
	if setting.Values128[0] != 20 {
		t.Errorf("score limit = %d, want the map default 20", setting.Values128[0])
	}

	pinned := protocol.GameSetting{Mode: sniperModeID}
	pinned.Strings16[1] = protocol.U16String("rel_Apex01_Diorama")
	pinned.Strings16[3] = protocol.U16String("rel_Apex01_Diorama")
	pinned.Values128[2] = 300000
	pinned.Values128[0] = 30
	setting, err = normalizeGameSetting(pinned, maps)
	if err != nil {
		t.Fatalf("pinned sniper room rejected: %v", err)
	}
	if got := protocol.FromU16(setting.Strings16[1]); got != "rel_Apex01_Diorama" {
		t.Errorf("map = %q, want rel_Apex01_Diorama", got)
	}
	if setting.Values128[2] != 300000 || setting.Values128[0] != 30 {
		t.Errorf("rules = %d/%d, want the request's 300000/30", setting.Values128[2], setting.Values128[0])
	}

	// Every competitive map is a sniper map in the client's MapMeta, so the pool
	// must be the whole no-item catalog.
	candidates := maps.Candidates(sniperModeID, "All")
	if len(candidates) != 57 {
		t.Errorf("sniper candidates = %d, want 57", len(candidates))
	}
	for _, entry := range candidates {
		if !entry.NormalMode {
			t.Errorf("%s is not a no-item map", entry.ResourceName)
		}
	}
}

// The mode still has to be a known one, and a sniper room must not fall through
// to the modes the server refuses.
func TestSniperModeDoesNotOpenOtherModes(t *testing.T) {
	maps := loadTestMaps(t)
	for _, mode := range []uint32{2, 5, 6, 13, 16, 18, 19, 101} {
		request := protocol.GameSetting{Mode: mode}
		request.Strings16[1] = protocol.U16String("Random")
		request.Strings16[3] = protocol.U16String("Random")
		if _, err := normalizeGameSetting(request, maps); err == nil {
			t.Errorf("mode %d was accepted", mode)
		}
		if got := maps.Candidates(mode, "All"); len(got) != 0 {
			t.Errorf("mode %d got %d map candidates", mode, len(got))
		}
	}
}
