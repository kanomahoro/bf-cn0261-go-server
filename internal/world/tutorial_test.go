package world

import "testing"

// The tutorial table is the client's own 25 rows: the prologue plus eight task
// groups on three difficulty tabs.
func TestTutorialStagesCoverTheReleasedRows(t *testing.T) {
	stages := TutorialStages()
	if len(stages) != 25 {
		t.Fatalf("stage count = %d, want 25", len(stages))
	}
	want := []uint32{0, 1, 2, 3, 4, 5, 6, 7, 8, 11, 12, 13, 14, 15, 16, 17, 18, 21, 22, 23, 24, 25, 26, 27, 28}
	for index, stage := range stages {
		if stage.Num != want[index] {
			t.Fatalf("row %d stageNum = %d, want %d", index, stage.Num, want[index])
		}
		if stage.Key == "" || stage.TimeAttackSeconds == 0 {
			t.Fatalf("row %d is incomplete: %+v", index, stage)
		}
	}
	// Only the prologue runs the wave chain; the numbered stages read their goals
	// from their own MapMeta row.
	for _, stage := range stages {
		if stage.Prologue != (stage.Key == "rel_prologue_01") {
			t.Fatalf("prologue flag wrong on %s", stage.Key)
		}
	}
}

// A play request resolves by MapMeta row name, whatever its case.
func TestFindTutorialStage(t *testing.T) {
	stage, ok := FindTutorialStage("REL_PROLOGUE_01")
	if !ok || !stage.Prologue || stage.Num != 0 {
		t.Fatalf("case-insensitive lookup failed: %+v %v", stage, ok)
	}
	stage, ok = FindTutorialStage("sin_mission_wall_lv_03")
	if !ok || stage.Num != 26 || stage.TimeAttackSeconds != 50 {
		t.Fatalf("wall lv_03 = %+v %v", stage, ok)
	}
	if _, ok := FindTutorialStage("rel_Tutorial_01"); ok {
		t.Fatal("the old tutorial map is not part of the released list")
	}
	if _, ok := FindTutorialStage("sin_mission_wall_lv_04"); ok {
		t.Fatal("a stage that does not exist must not resolve")
	}
}
