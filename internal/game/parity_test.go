package game

import (
	"testing"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
)

// Replays the exact report sequence of "rocket bubbles an opponent, then the
// opponent is kicked": knock, kick try, kick confirmed.
func TestParityRocketBubbleThenKick(t *testing.T) {
	life := NewBattleLife(nil, false)
	for slot := 0; slot < 2; slot++ {
		if err := life.Bind(uint8(slot), objectFor(slot), uint8(slot+1)); err != nil {
			t.Fatalf("bind %d: %v", slot, err)
		}
	}
	knock := protocol.PeerLifeEvent{
		Kind:         protocol.LifeKnocked,
		VictimObject: objectFor(0),
		ActorObject:  objectFor(1),
		Tick:         500,
	}
	if ok, err := life.Apply(0, knock, false); err != nil || !ok {
		t.Fatalf("knock accepted=%v err=%v", ok, err)
	}
	s1, _ := life.Score(1)
	if s1.BubbleCount != 1 {
		t.Fatalf("bubble count = %d, want 1", s1.BubbleCount)
	}
	try := protocol.PeerKickEvent{Kind: protocol.KickTry, Victim: objectFor(0), Actor: objectFor(1)}
	if ok, err := life.ApplyKick(1, try); err != nil || !ok {
		t.Fatalf("kick try accepted=%v err=%v", ok, err)
	}
	confirm := protocol.PeerKickEvent{Kind: protocol.KickConfirmed, Victim: objectFor(0), Actor: objectFor(1), Succeeded: true}
	if ok, err := life.ApplyKick(0, confirm); err != nil || !ok {
		t.Fatalf("kick confirm accepted=%v err=%v", ok, err)
	}
	s2, _ := life.Score(1)
	if s2.PangCount != 1 {
		t.Fatalf("pang count = %d, want 1", s2.PangCount)
	}
	// A second kick on the same bubble must not count twice.
	if ok, _ := life.ApplyKick(0, confirm); ok {
		t.Fatal("the same bubble was credited twice")
	}
}
