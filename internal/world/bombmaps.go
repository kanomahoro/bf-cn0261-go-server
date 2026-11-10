package world

// The bomb mission's own map pool. The client's MapMeta.xml flags six released
// maps with isBombmode (the same list the mode's widget draws), and four of them
// are also ordinary competitive maps this catalog already carries:
//
//	rel_Village03_CityHall  Village difficulty 3
//	rel_Desert02_Castle     Desert  difficulty 4
//	rel_Forest02_Ravine     Forest  difficulty 3
//	rel_Village02_waterPark Village difficulty 4
//	rel_zombie_01_bomb      Village difficulty 5
//	rel_Sudden06_Tank       Sudden  difficulty 4
//
// The last two carry isNotemTeamModeMap = 0, so they are not in the competitive
// pool and the catalog has no entry for them: they live here instead. Their time
// limit is MapMeta's DeathMatchTimeLimit of 240 s; the bomb mission's own 240 s
// TimeAttackTimeLimit agrees with it in every one of the four released maps that
// also has a competitive entry.
//
// The pool is data, not a rule: it exists so a bomb room never mixes in a map the
// client would not offer, and so the two bomb-only maps resolve at all.
var bombMapPool = []Map{
	{ResourceName: "rel_Village03_CityHall", Theme: "Village", Difficulty: 3, TimeLimitMilliseconds: 240000, ScoreLimit: 20, NormalMode: true, ItemMode: true, TeamSpawnCount: 8},
	{ResourceName: "rel_Desert02_Castle", Theme: "Desert", Difficulty: 4, TimeLimitMilliseconds: 240000, ScoreLimit: 20, NormalMode: true, ItemMode: true, TeamSpawnCount: 8},
	{ResourceName: "rel_Forest02_Ravine", Theme: "Forest", Difficulty: 3, TimeLimitMilliseconds: 240000, ScoreLimit: 20, NormalMode: true, ItemMode: true, TeamSpawnCount: 8},
	{ResourceName: "rel_Village02_waterPark", Theme: "Village", Difficulty: 4, TimeLimitMilliseconds: 240000, ScoreLimit: 20, NormalMode: true, ItemMode: true, TeamSpawnCount: 8},
	{ResourceName: "rel_zombie_01_bomb", Theme: "Village", Difficulty: 5, TimeLimitMilliseconds: 240000, ScoreLimit: 20, NormalMode: false, ItemMode: false, TeamSpawnCount: 8},
	{ResourceName: "rel_Sudden06_Tank", Theme: "Sudden", Difficulty: 4, TimeLimitMilliseconds: 240000, ScoreLimit: 20, NormalMode: false, ItemMode: false, TeamSpawnCount: 8},
}

// BombMaps returns the bomb mission's map pool, in the client's own order.
func (Maps) BombMaps() []Map { return bombMapPool }

// IsBombMap reports whether a resource name belongs to the bomb mission's own
// pool. The client's create dialog starts on the startup map (an ordinary
// competitive one), so a bomb room has to tell its pool apart from the rest of
// the catalog before it starts a round on a map the mode cannot run.
func (Maps) IsBombMap(name string) bool {
	for _, entry := range bombMapPool {
		if entry.ResourceName == name {
			return true
		}
	}
	return false
}
