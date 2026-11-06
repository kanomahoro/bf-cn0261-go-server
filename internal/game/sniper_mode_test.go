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
func TestNewerModesResolveToTheCompetitivePool(t *testing.T) {
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
func TestNewerModesDoNotOpenOtherModes(t *testing.T) {
	maps := loadTestMaps(t)
	for _, mode := range []uint32{2, 5, 6, 13, 16, 19, 20, 101} {
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

// The hot-blood team battles are the competitive round without the bubble
// stage, so they must resolve maps the same way and keep the create rules.
func TestHotBloodTeamBattlesUseTheCompetitivePool(t *testing.T) {
	maps := loadTestMaps(t)
	for _, mode := range []uint32{teamSurvivalModeID, sniperTeamSurvivalModeID} {
		random := protocol.GameSetting{Mode: mode}
		random.Strings16[1] = protocol.U16String("Random")
		random.Strings16[3] = protocol.U16String("Random")
		setting, err := normalizeGameSetting(random, maps)
		if err != nil {
			t.Fatalf("mode %d random room rejected: %v", mode, err)
		}
		if setting.Mode != mode {
			t.Errorf("mode %d came back as %d", mode, setting.Mode)
		}
		if setting.Values128[2] != 240000 || setting.Values128[0] != 20 {
			t.Errorf("mode %d rules = %d/%d, want the map defaults 240000/20",
				mode, setting.Values128[2], setting.Values128[0])
		}
		if got := len(maps.Candidates(mode, "All")); got != 57 {
			t.Errorf("mode %d candidates = %d, want 57", mode, got)
		}
	}
}

// The individual hot-blood battle (13) needs a rule the server does not have
// yet -- an opponent test that spans both seats of a teamless round -- so it
// stays closed.
func TestHotBloodIndividualStaysClosed(t *testing.T) {
	maps := loadTestMaps(t)
	request := protocol.GameSetting{Mode: 13}
	request.Strings16[1] = protocol.U16String("Random")
	request.Strings16[3] = protocol.U16String("Random")
	if _, err := normalizeGameSetting(request, maps); err == nil {
		t.Error("mode 13 was accepted")
	}
	if got := maps.Candidates(13, "All"); len(got) != 0 {
		t.Errorf("mode 13 got %d map candidates", len(got))
	}
}
