package world

import "testing"

// The Captain battle plays the same competitive maps as the no-item battle: the
// client's isCaptainModeMap list is the isNotemTeamModeMap list itself, so the
// pool check is that mode 20 sees exactly the normal-mode maps and nothing else.
func TestCaptainModeUsesTheNormalPool(t *testing.T) {
	maps := Maps{Maps: []Map{
		{ResourceName: "rel_A", NormalMode: true, ItemMode: true, Theme: "All", TeamSpawnCount: 8},
		{ResourceName: "rel_B", NormalMode: true, ItemMode: false, Theme: "All", TeamSpawnCount: 8},
		{ResourceName: "rel_C", NormalMode: false, ItemMode: true, Theme: "All", TeamSpawnCount: 8},
	}}
	names := func(mode uint32) []string {
		out := make([]string, 0, 3)
		for _, entry := range maps.Candidates(mode, "All") {
			out = append(out, entry.ResourceName)
		}
		return out
	}
	captain := names(20)
	plain := names(1)
	if len(captain) != 2 || len(plain) != 2 {
		t.Fatalf("captain %v, plain %v; want the two normal maps each", captain, plain)
	}
	for i := range captain {
		if captain[i] != plain[i] {
			t.Fatalf("captain pool %v differs from the no-item pool %v", captain, plain)
		}
	}
	for _, entry := range maps.Candidates(20, "All") {
		if !entry.NormalMode {
			t.Fatalf("%s is not a normal-mode map", entry.ResourceName)
		}
	}
}
