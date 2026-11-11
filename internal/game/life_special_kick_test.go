package game

import (
	"testing"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
)

// lifeTestRoom binds two opposing seats: slot 0 on team 1, slot 1 on team 2.
func lifeTestRoom() *BattleLife {
	life := NewBattleLife(nil, false)
	life.now = time.Now
	if err := life.Bind(0, objectFor(0), 1); err != nil {
		panic(err)
	}
	if err := life.Bind(1, objectFor(1), 2); err != nil {
		panic(err)
	}
	return life
}

// A bubble popped by a weapon arrives as a confirmed kick with the special flag
// and no contact attempt. The sniper battle scored nothing for it until the round
// credited it: every shot bubble is a pang.
func TestSpecialKickCreditsAPangInTheSniperBattle(t *testing.T) {
	life := lifeTestRoom()
	life.specialKickScores = true
	life.mu.Lock()
	life.players[1].state = stateBubbled
	life.mu.Unlock()

	accepted, err := life.ApplyKick(1, protocol.PeerKickEvent{
		Kind: protocol.KickConfirmed, Victim: objectFor(1), Actor: objectFor(0),
		Succeeded: true, SpecialKick: true,
	})
	if err != nil {
		t.Fatalf("special kick: %v", err)
	}
	if !accepted {
		t.Fatal("a special kick must be accepted")
	}
	score, err := life.Score(0)
	if err != nil {
		t.Fatalf("score: %v", err)
	}
	if score.PangCount != 1 {
		t.Fatalf("pang count = %d, want 1", score.PangCount)
	}
	if state := life.StateOf(1); state != "kicked" {
		t.Fatalf("victim state = %v, want kicked", state)
	}
	// The follow-up "got kicked" report must not score the pop a second time.
	accepted, err = life.ApplyKick(1, protocol.PeerKickEvent{
		Kind: protocol.KickGotKicked, Victim: objectFor(1), Actor: objectFor(0),
	})
	if err != nil {
		t.Fatalf("got kicked: %v", err)
	}
	if score, _ := life.Score(0); score.PangCount != 1 {
		t.Fatalf("a repeated report scored again: %d", score.PangCount)
	}
	_ = accepted
}

// Every other mode keeps refusing the special report: only the sniper battle is
// known to send it, and no other mode's scoring may move.
func TestSpecialKickOutsideTheSniperBattleIsRefused(t *testing.T) {
	life := lifeTestRoom()
	life.mu.Lock()
	life.players[1].state = stateBubbled
	life.mu.Unlock()

	accepted, err := life.ApplyKick(1, protocol.PeerKickEvent{
		Kind: protocol.KickConfirmed, Victim: objectFor(1), Actor: objectFor(0),
		Succeeded: true, SpecialKick: true,
	})
	if err != nil {
		t.Fatalf("special kick: %v", err)
	}
	if accepted {
		t.Fatal("a special kick outside the sniper battle must be refused")
	}
	if score, _ := life.Score(0); score.PangCount != 0 {
		t.Fatalf("pang count = %d, want 0", score.PangCount)
	}
}

// A non-special confirmation still needs the contact attempt that belongs to it:
// the melee path must keep its existing guard.
func TestConfirmedKickWithoutATryIsRefused(t *testing.T) {
	life := lifeTestRoom()
	life.mu.Lock()
	life.players[1].state = stateBubbled
	life.mu.Unlock()

	accepted, err := life.ApplyKick(1, protocol.PeerKickEvent{
		Kind: protocol.KickConfirmed, Victim: objectFor(1), Actor: objectFor(0),
		Succeeded: true,
	})
	if err != nil {
		t.Fatalf("confirmed kick: %v", err)
	}
	if accepted {
		t.Fatal("a confirmation without a contact attempt must be refused")
	}
	if score, _ := life.Score(0); score.PangCount != 0 {
		t.Fatalf("pang count = %d, want 0", score.PangCount)
	}
}
