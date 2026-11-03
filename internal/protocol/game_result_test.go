package protocol

import "testing"

// resultPlayer builds one valid settlement row.
func resultPlayer(slot uint8, team uint8) GameResultPlayer {
	return GameResultPlayer{
		Slot:       slot,
		AccountID:  uint32(slot) + 1,
		Nickname:   U16String("player" + string(rune('A'+slot))),
		Team:       team,
		Level:      1,
		LevelStart: 0,
		Experience: 0,
		NextLevel:  100,
	}
}

// The zombie battle decides the round between team 1 and team 14, so the result
// screen must carry both the zombie rows and the zombie team as the winner.
func TestGameResultCarriesTheZombieTeams(t *testing.T) {
	players := []GameResultPlayer{
		resultPlayer(0, 1),
		resultPlayer(1, 14),
		resultPlayer(2, 14),
	}
	packet, err := CreateGameResultReply([2]uint16{0, 0}, 14, players)
	if err != nil {
		t.Fatalf("zombie result refused: %v", err)
	}
	if packet.ID != 729 || len(packet.Payload) == 0 {
		t.Fatalf("packet id = %d, payload = %d bytes", packet.ID, len(packet.Payload))
	}
	reader := NewReader(packet.Payload)
	capacity, err := reader.Unsigned()
	if err != nil || capacity != 17 {
		t.Fatalf("team capacity = %d (%v), want 17", capacity, err)
	}
	winner, err := reader.Unsigned()
	if err != nil || winner != 14 {
		t.Fatalf("winner = %d (%v), want the zombie team 14", winner, err)
	}
}

// A winner the header cannot name is refused, and a row outside the zombie teams
// stays refused.
func TestGameResultRejectsUnknownTeams(t *testing.T) {
	if _, err := CreateGameResultReply([2]uint16{0, 0}, 20, []GameResultPlayer{resultPlayer(0, 1)}); err == nil {
		t.Fatal("a winner outside the scoreboard's teams must be refused")
	}
	if _, err := CreateGameResultReply([2]uint16{0, 0}, 0, []GameResultPlayer{resultPlayer(0, 3)}); err == nil {
		t.Fatal("team 3 is not a battle, zombie or cooperative team")
	}
	// Both team totals still follow the two-team comparison when no winner is named.
	packet, err := CreateGameResultReply([2]uint16{3, 1}, 0, []GameResultPlayer{resultPlayer(0, 1)})
	if err != nil {
		t.Fatalf("score-decided result refused: %v", err)
	}
	reader := NewReader(packet.Payload)
	if _, err := reader.Unsigned(); err != nil {
		t.Fatal(err)
	}
	if winner, err := reader.Unsigned(); err != nil || winner != 1 {
		t.Fatalf("winner = %d (%v), want 1", winner, err)
	}
}
