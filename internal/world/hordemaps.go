package world

// The pirate battle's own map pool (mode 101, wgtHorde).
//
// The client's MapMeta.xml flags 33 released rows with isHordeModeMap: eleven
// themes at three difficulties each. The row name is the map key the room
// carries (npc_*), while the resource directory it is built from is the rel_*
// map of the same theme, so a horde room's key and its loaded scene differ.
// Every row is 1..4 players, TimeAttackTimeLimit 300 s, ItemEnable 0 and
// WaitForLoading 90 s (the three npc_Space01 rows say 100, which is the same
// column's value for a map that loads instantly and is not used here).
//
//   - The four npc_Zombie_* rows MapMeta also flags (isHordeModeMap = 1) are the
//     Halloween variant's leftovers: MapIdx -1, award table 0, no AI script. They
//     are not part of this client's mode 101 pool and stay out.
//   - Only the two angel-rescue maps ship an AIScriptFileName
//     (rel_pirate03_prison_horde.ini, rel_village08_daohouse_horde.ini); the rest
//     build their waves from the client's own tables.
//   - ScoreLimit is 0: a cooperative round is decided by the waves, and this
//     client never asks for a kill limit in the mode.
//
// The pool is data, not a rule: it exists so a pirate room never mixes in a map
// the client would not offer and so the npc_* keys resolve at all. Hardcoded on
// purpose, the same way the bomb mission's pool is.
var hordeMapPool = []Map{
	{ResourceName: "npc_Forest02_Ravine01", Theme: "Forest", Difficulty: 1, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Forest02_Ravine02", Theme: "Forest", Difficulty: 2, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Forest02_Ravine03", Theme: "Forest", Difficulty: 3, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Forest01_Hill01", Theme: "Forest", Difficulty: 1, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Forest01_Hill02", Theme: "Forest", Difficulty: 2, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Forest01_Hill03", Theme: "Forest", Difficulty: 3, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Stadium01_Pangpang01", Theme: "Stadium", Difficulty: 1, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Stadium01_Pangpang02", Theme: "Stadium", Difficulty: 2, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Stadium01_Pangpang03", Theme: "Stadium", Difficulty: 3, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Desert04_Pharaoh01", Theme: "Desert", Difficulty: 1, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Desert04_Pharaoh02", Theme: "Desert", Difficulty: 2, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Desert04_Pharaoh03", Theme: "Desert", Difficulty: 3, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Village07_Easy01", Theme: "Village", Difficulty: 1, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Village07_Easy02", Theme: "Village", Difficulty: 2, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Village07_Easy03", Theme: "Village", Difficulty: 3, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Forest03_Waterfall01", Theme: "Forest", Difficulty: 1, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Forest03_Waterfall02", Theme: "Forest", Difficulty: 2, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Forest03_Waterfall03", Theme: "Forest", Difficulty: 3, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_pirate03_prison01", Theme: "Pirate", Difficulty: 1, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_pirate03_prison02", Theme: "Pirate", Difficulty: 2, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_pirate03_prison03", Theme: "Pirate", Difficulty: 3, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_village08_daohouse01", Theme: "Village", Difficulty: 1, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_village08_daohouse02", Theme: "Village", Difficulty: 2, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_village08_daohouse03", Theme: "Village", Difficulty: 3, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Desert05_mummy01", Theme: "Desert", Difficulty: 1, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Desert05_mummy02", Theme: "Desert", Difficulty: 2, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Desert05_mummy03", Theme: "Desert", Difficulty: 3, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Village05_Playground01", Theme: "Village", Difficulty: 1, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Village05_Playground02", Theme: "Village", Difficulty: 2, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Village05_Playground03", Theme: "Village", Difficulty: 3, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Space01_Spaceship01", Theme: "Space", Difficulty: 1, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Space01_Spaceship02", Theme: "Space", Difficulty: 2, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
	{ResourceName: "npc_Space01_Spaceship03", Theme: "Space", Difficulty: 3, TimeLimitMilliseconds: 300000, TeamSpawnCount: 8},
}

// HordeMaps returns the pirate battle's map pool, in the client's own order.
func (Maps) HordeMaps() []Map { return hordeMapPool }

// IsHordeMap reports whether a resource name belongs to the pirate battle's own
// pool.
func (Maps) IsHordeMap(name string) bool {
	for _, entry := range hordeMapPool {
		if entry.ResourceName == name {
			return true
		}
	}
	return false
}

// HordeCandidates returns the pirate maps of a theme, optionally limited to one
// difficulty. A difficulty of 0 keeps all three, which is what the client's
// plain Random entry asks for.
func (Maps) HordeCandidates(theme string, difficulty uint32) []Map {
	var out []Map
	for _, entry := range hordeMapPool {
		if difficulty != 0 && entry.Difficulty != difficulty {
			continue
		}
		if theme != "" && theme != "All" && theme != entry.Theme {
			continue
		}
		out = append(out, entry)
	}
	return out
}
