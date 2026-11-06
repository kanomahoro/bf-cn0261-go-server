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
	for _, mode := range []uint32{2, 5, 6, 15, 16, 19, 20, 101} {
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

// The individual hot-blood battle is open: it is the same competitive round with
// no teams at all, so only admission and the map pool change on this side.
func TestHotBloodIndividualOpens(t *testing.T) {
	maps := loadTestMaps(t)
	request := protocol.GameSetting{Mode: singlePvpModeID}
	request.Strings16[1] = protocol.U16String("Random")
	request.Strings16[3] = protocol.U16String("Random")
	setting, err := normalizeGameSetting(request, maps)
	if err != nil {
		t.Fatalf("individual hot-blood room rejected: %v", err)
	}
	if setting.Values128[2] != 240000 || setting.Values128[0] != 20 {
		t.Errorf("rules = %d/%d, want the map defaults 240000/20",
			setting.Values128[2], setting.Values128[0])
	}
	if got := len(maps.Candidates(singlePvpModeID, "All")); got != 57 {
		t.Errorf("candidates = %d, want 57", got)
	}
}

// A teamless round has no sides: the zero team every seat carries must not read
// as friendly, or the round would credit nobody.
func TestTeamlessRoundCountsEverySeatAsAnOpponent(t *testing.T) {
	life := NewBattleLife(nil, true)
	if err := life.Bind(0, objectFor(0), 0); err != nil {
		t.Fatalf("a teamless seat must bind with team zero: %v", err)
	}
	if err := life.Bind(1, objectFor(1), 0); err != nil {
		t.Fatalf("second teamless seat bind: %v", err)
	}
	event := protocol.PeerLifeEvent{
		Kind:         protocol.LifeKnocked,
		VictimObject: objectFor(0),
		ActorObject:  objectFor(1),
		Tick:         100,
		DirectDeath:  true,
	}
	accepted, err := life.Apply(0, event, false)
	if err != nil || !accepted {
		t.Fatalf("teamless direct death rejected: accepted=%v err=%v", accepted, err)
	}
	if score, err := life.Score(1); err != nil || score.PangCount != 1 {
		t.Errorf("attacker pangs = %v (err %v), want 1", score.PangCount, err)
	}
}

// The team rule must not loosen for the modes that have sides.
func TestTeamRoundStillRefusesAFriendlyKill(t *testing.T) {
	life := NewBattleLife(nil, false)
	for slot := uint8(0); slot < 2; slot++ {
		if err := life.Bind(slot, objectFor(int(slot)), 1); err != nil {
			t.Fatalf("bind %d: %v", slot, err)
		}
	}
	event := protocol.PeerLifeEvent{
		Kind:         protocol.LifeKnocked,
		VictimObject: objectFor(0),
		ActorObject:  objectFor(1),
		Tick:         100,
		DirectDeath:  true,
	}
	accepted, err := life.Apply(0, event, false)
	if err != nil {
		t.Fatalf("friendly knock raised an error: %v", err)
	}
	if accepted {
		t.Error("a friendly direct death was accepted")
	}
	if score, _ := life.Score(1); score.PangCount != 0 {
		t.Errorf("teammate got %d pangs", score.PangCount)
	}
}

// A teamless seat may respawn on either battle group; a seated one stays inside
// its own group.
func TestTeamlessSpawnMarkersSpanBothGroups(t *testing.T) {
	teamless := NewBattleLife(nil, true)
	for _, marker := range []uint32{11, 18, 21, 28} {
		if !teamless.validMarker(0, marker) {
			t.Errorf("teamless marker %d rejected", marker)
		}
	}
	for _, marker := range []uint32{0, 9, 10, 31, 280} {
		if teamless.validMarker(0, marker) {
			t.Errorf("teamless marker %d accepted", marker)
		}
	}
	team := NewBattleLife(nil, false)
	if !team.validMarker(1, 11) || team.validMarker(1, 21) {
		t.Error("a team-one seat left its own marker group")
	}
}

// Every admitted mode must keep the pool it had: the refactor of Candidates
// collapsed four identical no-item branches into one, so pin the counts.
func TestMapPoolsPerAdmittedMode(t *testing.T) {
	maps := loadTestMaps(t)
	for _, mode := range []uint32{1, 8, 12, 13, 14, 18} {
		if got := len(maps.Candidates(mode, "All")); got != 57 {
			t.Errorf("mode %d candidates = %d, want 57", mode, got)
		}
	}
	if got := len(maps.Candidates(3, "All")); got != 57 {
		t.Errorf("item mode candidates = %d, want 57", got)
	}
	if got := len(maps.Candidates(1, "Military")); got == 0 || got >= 57 {
		t.Errorf("theme filter returned %d of 57", got)
	}
}

// The individual mode leaves the respawn point to the client, the way the zombie
// battle does, so the whole respawn walk has to accept the marker it reports. A
// marker it cannot use would strand the seat in the offered state for the round.
func TestTeamlessRoundTakesTheClientsSpawnPoint(t *testing.T) {
	life := NewBattleLife(nil, true)
	for slot := 0; slot < 2; slot++ {
		if err := life.Bind(uint8(slot), objectFor(slot), 0); err != nil {
			t.Fatalf("bind %d: %v", slot, err)
		}
	}
	knock := protocol.PeerLifeEvent{
		Kind:         protocol.LifeKnocked,
		VictimObject: objectFor(0),
		ActorObject:  objectFor(1),
		Tick:         100,
		DirectDeath:  true,
	}
	if accepted, err := life.Apply(0, knock, false); err != nil || !accepted {
		t.Fatalf("direct death rejected: accepted=%v err=%v", accepted, err)
	}
	if err := life.RequestRespawn(0, objectFor(0)); err != nil {
		t.Fatalf("request: %v", err)
	}
	if !life.BeginRespawn(0) {
		t.Fatal("the dead seat did not enter the respawn wait")
	}
	if _, err := life.OfferRespawn(0, 11); err != nil {
		t.Fatalf("offer: %v", err)
	}
	// The client answers with a point of its own, outside the marker group the
	// server offered: that is the client's call in this mode.
	if _, err := life.ConfirmRespawn(0, objectFor(0), 3); err != nil {
		t.Errorf("teamless round refused the client's confirmation: %v", err)
	}
	if err := life.AdoptSpawnIndex(0, 3); err != nil {
		t.Errorf("teamless round refused the client's marker: %v", err)
	}

	// A seated round keeps the group rule.
	seated := NewBattleLife(nil, false)
	if err := seated.Bind(0, objectFor(0), 1); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if err := seated.AdoptSpawnIndex(0, 27); err == nil {
		t.Error("a team-one seat adopted the other block's marker")
	}
}

// The individual mode draws its respawn point from the whole map and never the
// point that seat used last, which is the one piece of new spawn logic the mode
// adds.
func TestIndividualModeDrawsARandomSpawnPoint(t *testing.T) {
	room := &Room{Setting: protocol.SessionSetting{Game: protocol.GameSetting{Mode: singlePvpModeID}}}
	previous := uint32(0)
	for i := 0; i < 200; i++ {
		marker := room.NextSpawnIndex(0, 8)
		group, within := marker/10, marker%10
		if (group != 1 && group != 2) || within < 1 || within > 8 {
			t.Fatalf("marker %d is outside both battle groups", marker)
		}
		if marker == previous {
			t.Fatalf("marker %d repeated", marker)
		}
		previous = marker
	}

	// A seated round keeps the team ring: team one never leaves its own group.
	seated := &Room{Setting: protocol.SessionSetting{Game: protocol.GameSetting{Mode: 1}}}
	seated.slots[0].team = 1
	seated.slots[0].session = &Session{}
	for i := 0; i < 20; i++ {
		if marker := seated.NextSpawnIndex(0, 8); marker/10 != 1 {
			t.Fatalf("a team-one seat took marker %d", marker)
		}
	}
}
