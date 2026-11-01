// Package world loads the immutable gameplay catalogs the original server kept
// in JSON files.
package world

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Map describes one playable map.
type Map struct {
	ResourceName          string `json:"resourceName"`
	Theme                 string `json:"theme"`
	Difficulty            uint32 `json:"difficulty"`
	TimeLimitMilliseconds uint32 `json:"timeLimitMilliseconds"`
	ScoreLimit            uint32 `json:"scoreLimit"`
	NormalMode            bool   `json:"normalMode"`
	ItemMode              bool   `json:"itemMode"`
	TeamSpawnCount        uint32 `json:"teamSpawnCount"`
}

// Maps is the map catalog.
type Maps struct {
	DefaultRulesMap string `json:"defaultRulesMap"`
	Maps            []Map  `json:"maps"`
}

// Find returns the map with the given resource name.
func (m Maps) Find(name string) (Map, bool) {
	for _, entry := range m.Maps {
		if entry.ResourceName == name {
			return entry, true
		}
	}
	return Map{}, false
}

// Candidates returns the maps that support a mode and theme.
func (m Maps) Candidates(mode uint32, theme string) []Map {
	var out []Map
	for _, entry := range m.Maps {
		switch {
		case mode == 1 && !entry.NormalMode:
			continue
		case mode == 3 && !entry.ItemMode:
			continue
		case mode != 1 && mode != 3:
			continue
		}
		if theme != "" && theme != "All" && theme != entry.Theme {
			continue
		}
		if entry.TeamSpawnCount == 0 {
			continue
		}
		out = append(out, entry)
	}
	return out
}

// Damage is the weapon damage profile.
type Damage struct {
	BaseDamage            float64 `json:"baseDamage"`
	CriticalMultiplier    float64 `json:"criticalMultiplier"`
	BackMultiplier        float64 `json:"backMultiplier"`
	UnscopedMultiplier    float64 `json:"unscopedMultiplier"`
	WallAttackMultiplier  float64 `json:"wallAttackMultiplier"`
	WallDefenseMultiplier float64 `json:"wallDefenseMultiplier"`
}

// Weapon is one weapon definition.
type Weapon struct {
	DefinitionID    uint32 `json:"definitionId"`
	WeaponSlot      uint32 `json:"weaponSlot"`
	WeaponType      string `json:"weaponType"`
	ProjectileCount uint32 `json:"projectileCount"`
	Damage          Damage `json:"damage"`
}

// Weapons is the weapon catalog.
type Weapons struct {
	Weapons []Weapon `json:"weapons"`
}

// DefinitionIDs returns the slot-ordered definition ids used by peer validation.
func (w Weapons) DefinitionIDs() []uint32 {
	out := make([]uint32, 0, len(w.Weapons))
	for _, weapon := range w.Weapons {
		out = append(out, weapon.DefinitionID)
	}
	return out
}

// CharacterHealth is one character's health modifier.
type CharacterHealth struct {
	ID          uint32  `json:"id"`
	HealthDelta float64 `json:"healthDelta"`
}

// Characters is the character catalog.
type Characters struct {
	CommonHealth float64           `json:"commonHealth"`
	Characters   []CharacterHealth `json:"characters"`
}

// HealthFor returns the total health of a character type.
func (c Characters) HealthFor(characterType uint32) float64 {
	for _, entry := range c.Characters {
		if entry.ID == characterType {
			return c.CommonHealth + entry.HealthDelta
		}
	}
	return c.CommonHealth
}

// DefenseModifier is one resistance/attack modifier.
type DefenseModifier struct {
	Attribute string  `json:"attribute"`
	Ratio     float64 `json:"ratio"`
}

// DefenseItem carries the modifiers of one item.
type DefenseItem struct {
	ItemID    uint32            `json:"itemId"`
	Modifiers []DefenseModifier `json:"modifiers"`
}

// Defense is the defense catalog.
type Defense struct {
	Items            []DefenseItem `json:"items"`
	UnsupportedItems []uint32      `json:"unsupportedItems"`
}

// RespawnCharacter is a respawn delay per character.
type RespawnCharacter struct {
	ID                    uint32 `json:"id"`
	DurationMilliseconds  uint32 `json:"durationMilliseconds"`
	ReductionMilliseconds uint32 `json:"reductionMilliseconds"`
}

// Respawn is the respawn catalog.
type Respawn struct {
	Characters []RespawnCharacter `json:"characters"`
	Items      []RespawnCharacter `json:"items"`
}

// Delay returns the respawn wait for a character with the given equipped items.
func (r Respawn) Delay(characterType uint32, itemIDs []uint32) time.Duration {
	base := uint32(7000)
	for _, entry := range r.Characters {
		if entry.ID == characterType && entry.DurationMilliseconds > 0 {
			base = entry.DurationMilliseconds
			break
		}
	}
	reduction := uint32(0)
	for _, entry := range r.Items {
		if entry.ReductionMilliseconds == 0 {
			continue
		}
		for _, id := range itemIDs {
			if id == entry.ID && entry.ReductionMilliseconds > reduction {
				reduction = entry.ReductionMilliseconds
			}
		}
	}
	if reduction >= base {
		return 0
	}
	return time.Duration(base-reduction) * time.Millisecond
}

// Reward is one end-of-match reward bundle.
type Reward struct {
	Experience uint32 `json:"experience"`
	Lucci      uint32 `json:"lucci"`
}

// ScoreWeights weights each score component.
type ScoreWeights struct {
	Bubble uint32 `json:"bubble"`
	Pang   uint32 `json:"pang"`
	Rescue uint32 `json:"rescue"`
}

// MedalMinimums is the medal threshold list per component.
type MedalMinimums struct {
	Bubble []uint32 `json:"bubble"`
	Pang   []uint32 `json:"pang"`
	Rescue []uint32 `json:"rescue"`
}

// ResultAwards configures the award grading.
type ResultAwards struct {
	ScoreWeights       ScoreWeights  `json:"scoreWeights"`
	GradeMinimumScores []uint32      `json:"gradeMinimumScores"`
	MedalMinimumCounts MedalMinimums `json:"medalMinimumCounts"`
}

// Progression is the level curve and rewards.
type Progression struct {
	Version         string   `json:"version"`
	LevelExperience []uint32 `json:"levelExperience"`
	Rewards         struct {
		Win  Reward `json:"win"`
		Draw Reward `json:"draw"`
		Loss Reward `json:"loss"`
	} `json:"rewards"`
	ResultAwards ResultAwards `json:"resultAwards"`
}

// Award is the computed grade/medal/score for one finished player.
type Award struct {
	Score uint32
	Grade uint32
	Medal uint32
}

// EvaluateResult mirrors ProgressionPolicy::EvaluateResult: the client indexes
// its grade and medal tables by these values, so they must never stay unset.
func (p Progression) EvaluateResult(bubbles, pangs, rescues uint16) Award {
	counts := [3]uint32{uint32(bubbles), uint32(pangs), uint32(rescues)}
	weights := [3]uint32{
		p.ResultAwards.ScoreWeights.Bubble,
		p.ResultAwards.ScoreWeights.Pang,
		p.ResultAwards.ScoreWeights.Rescue,
	}
	thresholds := [3][]uint32{
		p.ResultAwards.MedalMinimumCounts.Bubble,
		p.ResultAwards.MedalMinimumCounts.Pang,
		p.ResultAwards.MedalMinimumCounts.Rescue,
	}
	var award Award
	bestTier := uint32(0)
	bestCount := uint32(0)
	for index := range counts {
		award.Score += counts[index] * weights[index]
		tier := uint32(0)
		if len(thresholds[index]) >= 2 {
			if counts[index] >= thresholds[index][1] {
				tier = 2
			} else if counts[index] >= thresholds[index][0] {
				tier = 1
			}
		}
		if tier > bestTier || (tier != 0 && tier == bestTier && counts[index] > bestCount) {
			bestTier = tier
			bestCount = counts[index]
			if tier == 2 {
				award.Medal = uint32(index) + 7
			} else {
				award.Medal = uint32(index) + 4
			}
		}
	}
	// grade = number of thresholds the score reaches, from the top down.
	grades := p.ResultAwards.GradeMinimumScores
	reached := uint32(0)
	for i := len(grades) - 1; i >= 0; i-- {
		if award.Score >= grades[i] {
			reached = uint32(len(grades) - i)
			break
		}
	}
	if reached > 0 {
		award.Grade = reached - 1
	}
	return award
}

// Level returns the level, its starting experience and the next level
// threshold for a total experience value.
// MaxExperience returns the last configured level threshold. The reference
// clamps a character's total experience to it, so a stored value can never grow
// past the table.
func (p Progression) MaxExperience() uint32 {
	if len(p.LevelExperience) == 0 {
		return 0
	}
	return p.LevelExperience[len(p.LevelExperience)-1]
}

func (p Progression) Level(experience uint32) (level, start, next uint32) {
	if len(p.LevelExperience) == 0 {
		return 1, 0, experience
	}
	level = 1
	start = p.LevelExperience[0]
	next = start
	for i := 1; i < len(p.LevelExperience); i++ {
		if experience < p.LevelExperience[i] {
			return uint32(i), p.LevelExperience[i-1], p.LevelExperience[i]
		}
		level = uint32(i + 1)
		start = p.LevelExperience[i]
		next = start
	}
	return level, start, next
}

func loadJSON(path string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open %s: %w", path, err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("cannot parse %s: %w", path, err)
	}
	return nil
}

// LoadMaps reads the map catalog.
func LoadMaps(path string) (Maps, error) {
	var out Maps
	if err := loadJSON(path, &out); err != nil {
		return out, err
	}
	if len(out.Maps) == 0 {
		return out, fmt.Errorf("%s contains no maps", path)
	}
	return out, nil
}

// LoadWeapons reads the weapon catalog.
func LoadWeapons(path string) (Weapons, error) {
	var out Weapons
	if err := loadJSON(path, &out); err != nil {
		return out, err
	}
	if len(out.Weapons) == 0 {
		return out, fmt.Errorf("%s contains no weapons", path)
	}
	return out, nil
}

// LoadCharacters reads the character catalog.
func LoadCharacters(path string) (Characters, error) {
	var out Characters
	if err := loadJSON(path, &out); err != nil {
		return out, err
	}
	if len(out.Characters) == 0 {
		return out, fmt.Errorf("%s contains no characters", path)
	}
	return out, nil
}

// LoadDefense reads the defense catalog.
func LoadDefense(path string) (Defense, error) {
	var out Defense
	err := loadJSON(path, &out)
	return out, err
}

// LoadRespawn reads the respawn catalog.
func LoadRespawn(path string) (Respawn, error) {
	var out Respawn
	err := loadJSON(path, &out)
	return out, err
}

// LoadProgression reads the progression policy.
func LoadProgression(path string) (Progression, error) {
	var out Progression
	if err := loadJSON(path, &out); err != nil {
		return out, err
	}
	if len(out.LevelExperience) < 2 {
		return out, fmt.Errorf("%s has an unusable level curve", path)
	}
	return out, nil
}
