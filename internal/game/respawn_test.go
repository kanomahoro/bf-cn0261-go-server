package game

import (
	"testing"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/store"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/world"
)

// rapidRespawnNecklace is item 100142 in config/game_respawn.json.
const rapidRespawnNecklace = 100142

func testRespawnCatalog(t *testing.T) world.Respawn {
	t.Helper()
	catalog, err := world.LoadRespawn("../../config/game_respawn.json")
	if err != nil {
		t.Fatalf("load respawn catalog: %v", err)
	}
	return catalog
}

// TestRespawnItemIDsSkipBaggedGear pins the bug a player reported: unequipping
// the necklace left the shorter wait in place, because every inventory row fed
// the catalog instead of only the worn ones.
func TestRespawnItemIDsSkipBaggedGear(t *testing.T) {
	items := []store.Item{
		{ItemID: rapidRespawnNecklace, Quantity: 1, EquippedSlot: 9},
		{ItemID: 100241, Quantity: 1, EquippedSlot: 0},
		{ItemID: 100593, Quantity: 0, EquippedSlot: 4},
	}
	ids := respawnItemIDs(items)
	if len(ids) != 1 || ids[0] != rapidRespawnNecklace {
		t.Fatalf("respawn item ids = %v, want [%d]", ids, rapidRespawnNecklace)
	}
}

// TestRespawnDelayDropsWhenTheNecklaceComesOff walks the whole rule: worn gear
// shortens the wait, taking it off restores the character's base delay.
func TestRespawnDelayDropsWhenTheNecklaceComesOff(t *testing.T) {
	catalog := testRespawnCatalog(t)
	worn := []store.Item{{ItemID: rapidRespawnNecklace, Quantity: 1, EquippedSlot: 9}}
	bagged := []store.Item{{ItemID: rapidRespawnNecklace, Quantity: 1, EquippedSlot: 0}}

	const characterType = 1
	base := catalog.BaseDelay(characterType)
	if base != 7000*time.Millisecond {
		t.Fatalf("base delay = %v, want 7s", base)
	}
	if got := catalog.Delay(characterType, respawnItemIDs(worn)); got != 4800*time.Millisecond {
		t.Fatalf("worn delay = %v, want 4.8s", got)
	}
	if got := catalog.Delay(characterType, respawnItemIDs(bagged)); got != base {
		t.Fatalf("bagged delay = %v, want %v", got, base)
	}
}

// TestRespawnDelayKeepsTheLargestReduction covers two necks at once: the catalog
// credits the strongest reduction only, never the sum.
func TestRespawnDelayKeepsTheLargestReduction(t *testing.T) {
	catalog := testRespawnCatalog(t)
	items := []store.Item{
		{ItemID: 100142, Quantity: 1, EquippedSlot: 9},  // 2200
		{ItemID: 190022, Quantity: 1, EquippedSlot: 0},  // 3900 but bagged
		{ItemID: 100241, Quantity: 1, EquippedSlot: 10}, // 3800
	}
	if got := catalog.Delay(1, respawnItemIDs(items)); got != 3200*time.Millisecond {
		t.Fatalf("delay = %v, want 3.2s", got)
	}
}
