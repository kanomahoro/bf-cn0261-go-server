package world

import "testing"

// TestBombPoolMatchesTheClientFlags checks the six released isBombmode maps, in
// the order the client's MapMeta lists them.
func TestBombPoolMatchesTheClientFlags(t *testing.T) {
	maps := Maps{}
	want := []string{
		"rel_Village03_CityHall", "rel_Desert02_Castle", "rel_Forest02_Ravine",
		"rel_Village02_waterPark", "rel_zombie_01_bomb", "rel_Sudden06_Tank",
	}
	got := maps.Candidates(bombModeID, "All")
	if len(got) != len(want) {
		t.Fatalf("bomb pool has %d maps, want %d", len(got), len(want))
	}
	for index, name := range want {
		if got[index].ResourceName != name {
			t.Fatalf("bomb pool entry %d is %s, want %s", index, got[index].ResourceName, name)
		}
		if got[index].TeamSpawnCount == 0 {
			t.Fatalf("%s cannot place spawn markers", name)
		}
	}
}

// TestBombPoolFiltersThemes checks that the bomb mission's theme filter works on
// its own pool.
func TestBombPoolFiltersThemes(t *testing.T) {
	maps := Maps{}
	if got := maps.Candidates(bombModeID, "Desert"); len(got) != 1 || got[0].ResourceName != "rel_Desert02_Castle" {
		t.Fatalf("the desert bomb pool is wrong: %+v", got)
	}
	if got := maps.Candidates(bombModeID, "Sudden"); len(got) != 1 || got[0].ResourceName != "rel_Sudden06_Tank" {
		t.Fatalf("the sudden bomb pool is wrong: %+v", got)
	}
}

// TestFindResolvesBombOnlyMaps checks that the two maps the competitive catalog
// does not carry still resolve for a bomb round.
func TestFindResolvesBombOnlyMaps(t *testing.T) {
	maps := Maps{}
	for _, name := range []string{"rel_zombie_01_bomb", "rel_Sudden06_Tank"} {
		entry, ok := maps.Find(name)
		if !ok {
			t.Fatalf("%s does not resolve", name)
		}
		if entry.TimeLimitMilliseconds != 240000 {
			t.Fatalf("%s has time limit %d", name, entry.TimeLimitMilliseconds)
		}
	}
	if _, ok := maps.Find("rel_not_a_map"); ok {
		t.Fatal("an unknown map resolved")
	}
}

// TestCompetitivePoolIgnoresTheBombPool checks that the bomb pool does not leak
// into the ordinary modes' candidates.
func TestCompetitivePoolIgnoresTheBombPool(t *testing.T) {
	maps := Maps{}
	for _, mode := range []uint32{1, 3, 8, 12, 13, 14, 18, 20} {
		for _, entry := range maps.Candidates(mode, "All") {
			if entry.ResourceName == "rel_Sudden06_Tank" || entry.ResourceName == "rel_zombie_01_bomb" {
				t.Fatalf("bomb-only map %s offered to mode %d", entry.ResourceName, mode)
			}
		}
	}
}

// TestBombPoolOverride checks that an explicit pool replaces the built-in one.
func TestBombPoolOverride(t *testing.T) {
	maps := Maps{BombPool: []Map{{ResourceName: "rel_Test_Bomb", TeamSpawnCount: 8}}}
	got := maps.Candidates(bombModeID, "All")
	if len(got) != 1 || got[0].ResourceName != "rel_Test_Bomb" {
		t.Fatalf("the override was not used: %+v", got)
	}
}
