package game

import (
	"testing"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
)

// The bomb mission carries ammunition points: every seat reports them (C702) at
// every sub-round, so the mission has to place and refresh the boxes like the
// other ammunition modes.
func TestBombMissionPlacesAmmunitionBoxes(t *testing.T) {
	server := tutorialTestServer()
	if !server.supplyMode(&Room{Setting: protocol.SessionSetting{
		Game: protocol.GameSetting{Mode: bombModeID},
	}}) {
		t.Fatal("the bomb mission must accept map supply reports")
	}
	if !ammoSupplyMode(bombModeID) {
		t.Fatal("the bomb mission hands out ammunition boxes")
	}
	// A mode with no map boxes keeps its old behaviour.
	if server.supplyMode(&Room{Setting: protocol.SessionSetting{
		Game: protocol.GameSetting{Mode: 1},
	}}) {
		t.Fatal("the competitive battle places no boxes")
	}
}

// A box a previous sub-round left must not keep its point empty in the next one:
// the bomb mission reloads the stage and re-runs the same points.
func TestSupplyRoundRestartsEveryPoint(t *testing.T) {
	room := &Room{Started: true, phase: phasePlaying}
	state := &supplyRound{points: []*supplyPoint{
		{SupplyReportPoint: protocol.SupplyReportPoint{X: 1}, ActiveID: 7},
		{SupplyReportPoint: protocol.SupplyReportPoint{X: 2}, ActiveID: 9},
	}}
	room.supplies = state
	server := tutorialTestServer()
	server.startSupplies(room, state)
	room.mu.Lock()
	for index, point := range state.points {
		if point.ActiveID != 0 {
			t.Fatalf("point %d kept the stale box id %d", index, point.ActiveID)
		}
	}
	room.mu.Unlock()
}
