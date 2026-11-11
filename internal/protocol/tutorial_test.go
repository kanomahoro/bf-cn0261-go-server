package protocol

import "testing"

// The play request's own wire order: thirteen MapMeta words, six flag words,
// three wide strings and the difficulty.
func TestReadTutorialPlayRequestKeepsItsFields(t *testing.T) {
	w := NewWriter()
	for index := 0; index < 13; index++ {
		value := uint32(index)
		if index == 8 {
			value = 210000
		}
		if index == 11 {
			value = 0x10
		}
		w.Unsigned(value)
	}
	for index := 0; index < 6; index++ {
		w.Unsigned(0xFFFFFFFF)
	}
	w.String(U16String("Forest"))
	w.String(U16String("display"))
	w.String(U16String("sin_mission_help_lv_02"))
	w.Unsigned(2)
	packet := Packet{ID: 107, Payload: w.Payload()}
	request, err := ReadTutorialPlayRequest(packet)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if request.Directory != "Forest" || request.Display != "display" ||
		request.MapKey != "sin_mission_help_lv_02" {
		t.Fatalf("strings = %+v", request)
	}
	if request.Difficulty != 2 {
		t.Fatalf("difficulty = %d, want 2", request.Difficulty)
	}
	if request.TimeAttackMs != 210000 {
		t.Fatalf("time attack = %d, want 210000", request.TimeAttackMs)
	}
	if request.Flags52 != 0x10 {
		t.Fatalf("flags52 = %x, want 10", request.Flags52)
	}
}

// Every truncated body is refused, and trailing bytes are refused too.
func TestReadTutorialPlayRequestRefusesIncompleteBodies(t *testing.T) {
	w := NewWriter()
	for index := 0; index < 13; index++ {
		w.Unsigned(uint32(index))
	}
	for index := 0; index < 6; index++ {
		w.Unsigned(0)
	}
	w.String(U16String("Forest"))
	w.String(U16String("display"))
	w.String(U16String("rel_prologue_01"))
	w.Unsigned(1)
	body := w.Payload()
	for length := 0; length < len(body); length++ {
		if _, err := ReadTutorialPlayRequest(Packet{ID: 107, Payload: body[:length]}); err == nil {
			t.Fatalf("a %d byte body must be refused", length)
		}
	}
	trailing := append(append([]byte{}, body...), 0)
	if _, err := ReadTutorialPlayRequest(Packet{ID: 107, Payload: trailing}); err == nil {
		t.Fatal("trailing bytes must be refused")
	}
}

// The stage end carries one word, and only 0 or 1 is a stage result.
func TestReadTutorialEndStage(t *testing.T) {
	for value, want := range map[uint32]bool{0: false, 1: true} {
		w := NewWriter()
		w.Unsigned(value)
		got, err := ReadTutorialEndStage(Packet{ID: 724, Payload: w.Payload()})
		if err != nil {
			t.Fatalf("value %d: %v", value, err)
		}
		if got != want {
			t.Fatalf("value %d decoded as %v", value, got)
		}
	}
	w := NewWriter()
	w.Unsigned(2)
	if _, err := ReadTutorialEndStage(Packet{ID: 724, Payload: w.Payload()}); err == nil {
		t.Fatal("an unknown completion flag must be refused")
	}
}

// The stage list is a counted list of three-word rows.
func TestCreateTutorialStageInfoReply(t *testing.T) {
	packet := CreateTutorialStageInfoReply([]TutorialStageClear{{Stage: 0}, {Stage: 11, BestCount: 3, BestSeconds: 75}})
	if packet.ID != 922 {
		t.Fatalf("id = %d, want 922", packet.ID)
	}
	r := NewReader(packet.Payload)
	count, err := r.Unsigned()
	if err != nil || count != 2 {
		t.Fatalf("count = %d (%v), want 2", count, err)
	}
	if stage, err := r.Unsigned(); err != nil || stage != 0 {
		t.Fatalf("first stage = %d (%v)", stage, err)
	}
	if _, err := r.Unsigned(); err != nil {
		t.Fatalf("first record: %v", err)
	}
	if _, err := r.Unsigned(); err != nil {
		t.Fatalf("first record: %v", err)
	}
	if stage, err := r.Unsigned(); err != nil || stage != 11 {
		t.Fatalf("second stage = %d (%v)", stage, err)
	}
	if best, err := r.Unsigned(); err != nil || best != 3 {
		t.Fatalf("second record = %d (%v)", best, err)
	}
	if seconds, err := r.Unsigned(); err != nil || seconds != 75 {
		t.Fatalf("second record = %d (%v)", seconds, err)
	}
	if err := r.RequireEnd(); err != nil {
		t.Fatalf("trailing bytes: %v", err)
	}
}

// The scene transition is one cleared flag.
func TestCreateGotoMenuReply(t *testing.T) {
	packet := CreateGotoMenuReply()
	if packet.ID != 108 || len(packet.Payload) != 4 {
		t.Fatalf("id %d, %d bytes", packet.ID, len(packet.Payload))
	}
}

// tutorialScoreReportBody builds C710 the way the client serializes it.
func tutorialScoreReportBody(kind uint8) []byte {
	w := NewWriter()
	w.Byte(kind)
	w.Unsigned(0) // unknown_8
	w.Unsigned(0) // unknown_12
	w.Byte(0)     // unknown_16
	w.Byte(0)     // reporter slot
	w.Byte(0)     // unknown_18
	w.Unsigned(0) // unknown_20 and unknown_22
	return w.Payload()
}

// Kinds 1 and 22 are bubbles and 4 is a rescue; every other kind belongs to
// somebody else's report.
func TestReadTutorialScoreReportKinds(t *testing.T) {
	cases := []struct {
		kind uint8
		goal TutorialScoreGoal
		ok   bool
	}{
		{1, TutorialGoalBubble, true},
		{22, TutorialGoalBubble, true},
		{4, TutorialGoalHelp, true},
		{2, 0, false},
		{3, 0, false},
		{27, 0, false},
	}
	for _, testCase := range cases {
		goal, ok, err := ReadTutorialScoreReport(Packet{ID: 710, Payload: tutorialScoreReportBody(testCase.kind)})
		if err != nil {
			t.Fatalf("kind %d: %v", testCase.kind, err)
		}
		if ok != testCase.ok || (ok && goal != testCase.goal) {
			t.Fatalf("kind %d decoded as %v/%v", testCase.kind, goal, ok)
		}
	}
	// The compact report is six bytes and only kind 2 is a pang.
	w := NewWriter()
	w.Byte(2)
	w.Unsigned(0)
	w.Byte(0)
	if goal, ok, err := ReadTutorialOptimizedScoreReport(Packet{ID: 2023, Payload: w.Payload()}); err != nil ||
		!ok || goal != TutorialGoalPang {
		t.Fatalf("compact pang = %v/%v (%v)", goal, ok, err)
	}
	w = NewWriter()
	w.Byte(1)
	w.Unsigned(0)
	w.Byte(0)
	if _, ok, err := ReadTutorialOptimizedScoreReport(Packet{ID: 2023, Payload: w.Payload()}); err != nil || ok {
		t.Fatalf("an unusable compact kind must be ignored (%v)", err)
	}
}

// The score update is the bare score record: seventeen-team table, sixteen rows
// and no reward tail, with the counters where the client reads them.
func TestCreateTutorialScoreReply(t *testing.T) {
	row := GameResultPlayer{
		Slot: 0, AccountID: 7, Nickname: U16String("tester"), Team: 1, Level: 3,
		Bubbles: 11, Pangs: 12, Rescues: 13, AwardGrade: Unset,
	}
	packet, err := CreateTutorialScoreReply([]GameResultPlayer{row})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if packet.ID != 729 {
		t.Fatalf("id = %d, want 729", packet.ID)
	}
	r := NewReader(packet.Payload)
	if count, err := r.Unsigned(); err != nil || count != 17 {
		t.Fatalf("team count = %d (%v)", count, err)
	}
	if winner, err := r.Unsigned(); err != nil || winner != 0 {
		t.Fatalf("winner = %d (%v)", winner, err)
	}
	if _, err := r.Unsigned(); err != nil {
		t.Fatalf("flag: %v", err)
	}
	for team := 1; team <= 17; team++ {
		for word := 0; word < 4; word++ {
			if _, err := r.Unsigned(); err != nil {
				t.Fatalf("team %d: %v", team, err)
			}
		}
	}
	if slot, err := r.Unsigned(); err != nil || slot != 0 {
		t.Fatalf("row slot = %d (%v)", slot, err)
	}
	if account, err := r.Unsigned(); err != nil || account != 7 {
		t.Fatalf("row account = %d (%v)", account, err)
	}
	if nickname, err := r.String(SessionStringLimit); err != nil {
		t.Fatalf("row nickname: %v", err)
	} else if FromU16(nickname) != "tester" {
		t.Fatalf("row nickname = %q", FromU16(nickname))
	}
	if team, err := r.Unsigned(); err != nil || team != 1 {
		t.Fatalf("row team = %d (%v)", team, err)
	}
	if level, err := r.Unsigned(); err != nil || level != 3 {
		t.Fatalf("row level = %d (%v)", level, err)
	}
	bubbles, err := r.Unsigned()
	if err != nil || bubbles != 11 {
		t.Fatalf("bubbles = %d (%v)", bubbles, err)
	}
	pangs, err := r.Unsigned()
	if err != nil || pangs != 12 {
		t.Fatalf("pangs = %d (%v)", pangs, err)
	}
	if _, err := r.Unsigned(); err != nil {
		t.Fatalf("unknown_52: %v", err)
	}
	helps, err := r.Unsigned()
	if err != nil || helps != 13 {
		t.Fatalf("helps = %d (%v)", helps, err)
	}
}

// The stage's result is the same bare record, naming the winning side: the
// tutorial's result step reads it instead of the two team bytes.
func TestCreateTutorialResultReply(t *testing.T) {
	for _, completed := range []bool{true, false} {
		row := GameResultPlayer{
			Slot: 0, AccountID: 7, Nickname: U16String("tester"), Team: 1, Level: 2,
			Bubbles: 5, Pangs: 6, Rescues: 7, AwardGrade: Unset,
		}
		packet, err := CreateTutorialResultReply(row, completed)
		if err != nil {
			t.Fatalf("completed %v: %v", completed, err)
		}
		if packet.ID != 732 {
			t.Fatalf("id = %d, want 732", packet.ID)
		}
		r := NewReader(packet.Payload)
		if count, err := r.Unsigned(); err != nil || count != 17 {
			t.Fatalf("team count = %d (%v)", count, err)
		}
		wantWinner := uint32(2)
		wantFirst, wantSecond := uint32(0), uint32(1)
		if completed {
			wantWinner, wantFirst, wantSecond = 1, 1, 0
		}
		if winner, err := r.Unsigned(); err != nil || winner != wantWinner {
			t.Fatalf("winner = %d (%v), want %d", winner, err, wantWinner)
		}
		if _, err := r.Unsigned(); err != nil {
			t.Fatalf("flag: %v", err)
		}
		first, err := r.Unsigned()
		if err != nil {
			t.Fatalf("team 1 prefix: %v", err)
		}
		total1, err := r.Unsigned()
		if err != nil {
			t.Fatalf("team 1 total: %v", err)
		}
		_ = first
		if total1 != wantFirst {
			t.Fatalf("team 1 total = %d, want %d", total1, wantFirst)
		}
		if _, err := r.Unsigned(); err != nil {
			t.Fatalf("team 1 suffix: %v", err)
		}
		if _, err := r.Unsigned(); err != nil {
			t.Fatalf("team 1 suffix: %v", err)
		}
		if _, err := r.Unsigned(); err != nil {
			t.Fatalf("team 2 prefix: %v", err)
		}
		total2, err := r.Unsigned()
		if err != nil {
			t.Fatalf("team 2 total: %v", err)
		}
		if total2 != wantSecond {
			t.Fatalf("team 2 total = %d, want %d", total2, wantSecond)
		}
	}
}
