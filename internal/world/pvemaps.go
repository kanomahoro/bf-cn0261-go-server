package world

// The cooperative zombie raid's own map pool (mode 16, wgtZombiePve).
//
// The client's MapMeta.xml flags seven released rows with isZombiePveModeMap,
// and each of them ships its own <dir>_ZombiePve.ini: the script that spawns the
// citizens, the guardians, the zombie groups and the boss (NameID 31 at clock
// 185000). Every row is 1..4 players, isTeam and isSingle both set, ItemEnable 0,
// WaitForLoading 90 s, and TimeAttackTimeLimit 600 s -- the mode plays
// TimeAttack, so the round is ten minutes long, not the 240 s its
// DeathMatchTimeLimit column states.
//
// None of the seven is an ordinary competitive map, so the game_maps catalog has
// no entry for them and they live here the same way the bomb mission's and the
// pirate battle's own rows do.
//
//	rel_zombie_01         Village  泡泡村防御大作战！
//	rel_zombie02_Twobridge Ice     守护积雪桥！
//	rel_zombie03_Circle   Village  僵尸公园
//	rel_zombie04_Oneway   Desert   沙漠小路僵尸袭击！
//	rel_zombie04_1_Oneway Desert   [进阶]沙漠小路袭击！ (the advanced variant; its
//	                              ZombiePve resource states no mission groups)
//	rel_zombie05_Tear     Pirate   骷髅的眼泪僵尸袭击!
//	rel_zombie06_Frontal  Pirate   僵尸！正面突破！！
var pveMapPool = []Map{
	{ResourceName: "rel_zombie_01", Theme: "Village", Difficulty: 1, TimeLimitMilliseconds: 600000, TeamSpawnCount: 8},
	{ResourceName: "rel_zombie02_Twobridge", Theme: "Ice", Difficulty: 1, TimeLimitMilliseconds: 600000, TeamSpawnCount: 8},
	{ResourceName: "rel_zombie03_Circle", Theme: "Village", Difficulty: 1, TimeLimitMilliseconds: 600000, TeamSpawnCount: 8},
	{ResourceName: "rel_zombie04_Oneway", Theme: "Desert", Difficulty: 1, TimeLimitMilliseconds: 600000, TeamSpawnCount: 8},
	{ResourceName: "rel_zombie04_1_Oneway", Theme: "Desert", Difficulty: 2, TimeLimitMilliseconds: 600000, TeamSpawnCount: 8},
	{ResourceName: "rel_zombie05_Tear", Theme: "Pirate", Difficulty: 1, TimeLimitMilliseconds: 600000, TeamSpawnCount: 8},
	{ResourceName: "rel_zombie06_Frontal", Theme: "Pirate", Difficulty: 1, TimeLimitMilliseconds: 600000, TeamSpawnCount: 8},
}

// PveMaps returns the cooperative zombie raid's map pool, in the client's order.
func (Maps) PveMaps() []Map { return pveMapPool }

// IsPveMap reports whether a resource name belongs to the raid's own pool.
func (Maps) IsPveMap(name string) bool {
	for _, entry := range pveMapPool {
		if entry.ResourceName == name {
			return true
		}
	}
	return false
}

// PveCandidates returns the raid maps of a theme.
func (Maps) PveCandidates(theme string) []Map {
	var out []Map
	for _, entry := range pveMapPool {
		if theme != "" && theme != "All" && theme != entry.Theme {
			continue
		}
		out = append(out, entry)
	}
	return out
}
