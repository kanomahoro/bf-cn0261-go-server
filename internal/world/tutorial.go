package world

import "strings"

// TutorialStage is one row of the client's own tutorial stage list: the
// MapMeta rows flagged isSinglePlayTutorial = 1. The client draws the list from
// its installed MapMeta, so this table only exists to validate a play request and
// to answer the cleared-stage list with the stage numbers the client looks rows
// up by (stageNum plus ten times its difficulty tab).
type TutorialStage struct {
	// Key is the MapMeta row name the play request carries.
	Key string
	// Num is MapMeta's stageNum; the select scene keys its rows by it.
	Num uint32
	// Name is the row's MissionName, kept for logs.
	Name string
	// TimeAttackSeconds is MapMeta's TimeAttackTimeLimit column.
	TimeAttackSeconds uint32
	// Prologue marks the one tutorial map that runs NPC waves (wgtHordeTutor,
	// mode 103); every other row is a numbered single-play stage (mode 15).
	Prologue bool
}

// tutorialStages is the 25 released tutorial rows in stageNum order, taken from
// the CN0261 client data (MapMeta.xml, workspace/notes/data-bf-tutorial-stages.csv).
var tutorialStages = []TutorialStage{
	{Key: "rel_prologue_01", Num: 0, Name: "프롤로그", TimeAttackSeconds: 99999, Prologue: true},

	{Key: "sin_mission_bubble_rifle_lv_01", Num: 1, Name: "버블-라이플", TimeAttackSeconds: 180},
	{Key: "sin_mission_bubble_shotgun_lv_01", Num: 2, Name: "버블-샷건", TimeAttackSeconds: 180},
	{Key: "sin_mission_bubble_sniper_lv_01", Num: 3, Name: "버블-스나이퍼", TimeAttackSeconds: 180},
	{Key: "sin_mission_pang_lv_01", Num: 4, Name: "팡", TimeAttackSeconds: 180},
	{Key: "sin_mission_help_lv_01", Num: 5, Name: "헬프", TimeAttackSeconds: 180},
	{Key: "sin_mission_wall_lv_01", Num: 6, Name: "엄폐", TimeAttackSeconds: 90},
	{Key: "sin_mission_roll_lv_01", Num: 7, Name: "구르기", TimeAttackSeconds: 90},
	{Key: "sin_mission_supang_sniper_lv_01", Num: 8, Name: "击爆_狙击", TimeAttackSeconds: 210},

	{Key: "sin_mission_bubble_rifle_lv_02", Num: 11, Name: "버블-라이플", TimeAttackSeconds: 210},
	{Key: "sin_mission_bubble_shotgun_lv_02", Num: 12, Name: "버블-샷건", TimeAttackSeconds: 210},
	{Key: "sin_mission_bubble_sniper_lv_02", Num: 13, Name: "버블-스나이퍼", TimeAttackSeconds: 210},
	{Key: "sin_mission_pang_lv_02", Num: 14, Name: "팡", TimeAttackSeconds: 210},
	{Key: "sin_mission_help_lv_02", Num: 15, Name: "헬프", TimeAttackSeconds: 210},
	{Key: "sin_mission_wall_lv_02", Num: 16, Name: "엄폐", TimeAttackSeconds: 60},
	{Key: "sin_mission_roll_lv_02", Num: 17, Name: "구르기", TimeAttackSeconds: 45},
	{Key: "sin_mission_supang_sniper_lv_02", Num: 18, Name: "击爆_狙击", TimeAttackSeconds: 210},

	{Key: "sin_mission_bubble_rifle_lv_03", Num: 21, Name: "버블-라이플", TimeAttackSeconds: 240},
	{Key: "sin_mission_bubble_shotgun_lv_03", Num: 22, Name: "버블-샷건", TimeAttackSeconds: 240},
	{Key: "sin_mission_bubble_sniper_lv_03", Num: 23, Name: "버블-스나이퍼", TimeAttackSeconds: 240},
	{Key: "sin_mission_pang_lv_03", Num: 24, Name: "팡", TimeAttackSeconds: 240},
	{Key: "sin_mission_help_lv_03", Num: 25, Name: "헬프", TimeAttackSeconds: 240},
	{Key: "sin_mission_wall_lv_03", Num: 26, Name: "엄폐", TimeAttackSeconds: 50},
	{Key: "sin_mission_roll_lv_03", Num: 27, Name: "구르기", TimeAttackSeconds: 15},
	{Key: "sin_mission_supang_sniper_lv_03", Num: 28, Name: "击爆_狙击", TimeAttackSeconds: 210},
}

// TutorialStages returns the tutorial stage table in the client's own order.
func TutorialStages() []TutorialStage { return tutorialStages }

// FindTutorialStage resolves a play request's map key. The client sends the row
// name exactly; the comparison is case-insensitive so a differently cased key
// still finds its stage.
func FindTutorialStage(key string) (TutorialStage, bool) {
	for _, stage := range tutorialStages {
		if strings.EqualFold(stage.Key, key) {
			return stage, true
		}
	}
	return TutorialStage{}, false
}
