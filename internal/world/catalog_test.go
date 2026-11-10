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

// TestModePoolsCarryTheirOwnExtraMaps pins the map counts the client's own mode
// widgets show (cn-717 MapMeta.xml, release-only): modes 8/12/13 list 58 maps
// while the competitive catalog carries 57, because each of them adds one map
// that carries neither the no-item nor the item flag.
func TestModePoolsCarryTheirOwnExtraMaps(t *testing.T) {
	maps, err := LoadMaps("../../config/game_maps.json")
	if err != nil {
		t.Fatalf("the shipped map catalog does not load: %v", err)
	}
	cases := []struct {
		mode uint32
		want int
		only string
	}{
		{1, 57, ""},
		{8, 58, "rel_Stadium03_StardustArena"},
		{12, 58, "rel_pirate02_frontal"},
		{13, 58, "rel_Stadium03_StardustArena"},
		{20, 57, ""},
	}
	for _, c := range cases {
		got := maps.Candidates(c.mode, "All")
		if len(got) != c.want {
			t.Fatalf("mode %d offers %d maps, want %d", c.mode, len(got), c.want)
		}
		if c.only == "" {
			continue
		}
		found := false
		for _, entry := range got {
			if entry.ResourceName == c.only {
				found = true
			}
		}
		if !found {
			t.Fatalf("mode %d does not offer %s", c.mode, c.only)
		}
	}
	// The competitive pools must not inherit the mode-only rows.
	for _, entry := range maps.Candidates(1, "All") {
		if entry.ResourceName == "rel_Stadium03_StardustArena" || entry.ResourceName == "rel_pirate02_frontal" {
			t.Fatalf("the competitive pool picked up %s", entry.ResourceName)
		}
	}
}
