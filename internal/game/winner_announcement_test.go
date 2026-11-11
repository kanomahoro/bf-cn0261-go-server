package game

import "testing"

// The bomb series never announces its winner with S732: the client drops a few
// seconds after that packet, both inside a sub-round and on the settled match.
func TestBombSeriesSendsNoWinnerAnnouncement(t *testing.T) {
	if sendsWinnerAnnouncement(bombModeID) {
		t.Fatal("the bomb mode must not send S732")
	}
	for _, mode := range []uint32{1, itemModeID, zombieModeID, sniperModeID, captainModeID} {
		if !sendsWinnerAnnouncement(mode) {
			t.Fatalf("mode %d must keep its winner announcement", mode)
		}
	}
}
