package protocol

import "fmt"

// Tutorial mode ids the client itself selects from the stage row: the prologue
// runs the wave tutorial (wgtHordeTutor), every numbered stage the single-play
// scene (wgtSinglePlay).
const (
	TutorialHordeMode      uint32 = 103
	TutorialSinglePlayMode uint32 = 15
)

// TutorialPlayRequest is C107 gcpReqPlayTutorial. The client sends its nested
// MapMetaData record (13 words, six bool words), then three wide strings and the
// row's difficulty. Only three of those carry meaning here: the third string is
// the MapMeta row the select scene chose, word 8 of the record is the row's
// TimeAttackTimeLimit in milliseconds, and the trailing word is the difficulty
// tab.
type TutorialPlayRequest struct {
	// Directory is the first wide string (MapMeta Directory / path family).
	Directory string
	// Display is the second wide string; the client fills it with the row's own
	// name column.
	Display string
	// MapKey is the third wide string: the MapMeta row name to play.
	MapKey string
	// Difficulty is the row's difficulty tab (1..3); zero leaves the wave
	// tables empty in mode 103.
	Difficulty uint32
	// TimeAttackMs is MapMeta's TimeAttackTimeLimit in milliseconds. Zero leaves
	// the letterbox up in mode 103.
	TimeAttackMs uint32
	// Flags52 is the record's twelfth word, kept so a rejected request can log
	// what the client sent.
	Flags52 uint32
}

// tutorialPlayMetaWords is the nested MapMetaData word count the CN0261 client's
// reader consumes (cn-717 sub_8CFAF0).
const tutorialPlayMetaWords = 13

// tutorialPlayMetaTimeAttackIndex is MapMeta's TimeAttackTimeLimit column: the
// ninth of the thirteen words read, in milliseconds.
const tutorialPlayMetaTimeAttackIndex = 8

// tutorialPlayBoolWords is how many four-byte flags follow the record. cn-717
// reads six; the wire order is not the object order, which does not matter here
// because none of them is interpreted.
const tutorialPlayBoolWords = 6

// ReadTutorialPlayRequest parses C107.
func ReadTutorialPlayRequest(p Packet) (TutorialPlayRequest, error) {
	if p.ID != 107 {
		return TutorialPlayRequest{}, fmt.Errorf("%w: expected PlayTutorial", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	request := TutorialPlayRequest{}
	for index := 0; index < tutorialPlayMetaWords; index++ {
		value, err := r.Unsigned()
		if err != nil {
			return request, err
		}
		if index == tutorialPlayMetaTimeAttackIndex {
			request.TimeAttackMs = value
		}
		if index == 11 {
			request.Flags52 = value
		}
	}
	for index := 0; index < tutorialPlayBoolWords; index++ {
		if _, err := r.Unsigned(); err != nil {
			return request, err
		}
	}
	var raw []uint16
	var err error
	if raw, err = r.String(SessionStringLimit); err != nil {
		return request, err
	}
	request.Directory = FromU16(raw)
	if raw, err = r.String(SessionStringLimit); err != nil {
		return request, err
	}
	request.Display = FromU16(raw)
	if raw, err = r.String(SessionStringLimit); err != nil {
		return request, err
	}
	request.MapKey = FromU16(raw)
	if request.Difficulty, err = r.Unsigned(); err != nil {
		return request, err
	}
	if err := r.RequireEnd(); err != nil {
		return request, err
	}
	return request, nil
}

// ReadTutorialEndStage parses C724 gcpReqEndStage: one word, 1 when the stage
// was cleared and 0 when it was failed or abandoned.
func ReadTutorialEndStage(p Packet) (bool, error) {
	if p.ID != 724 {
		return false, fmt.Errorf("%w: expected EndStage", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	value, err := r.Unsigned()
	if err != nil {
		return false, err
	}
	if err := r.RequireEnd(); err != nil {
		return false, err
	}
	if value > 1 {
		return false, fmt.Errorf("%w: stage completion flag %d", ErrInvalidPacket, value)
	}
	return value == 1, nil
}

// TutorialStageClear is one row of S922 gspResSingleStageInfo.
type TutorialStageClear struct {
	Stage uint32
	// BestCount and BestSeconds are the row's record. Zero keeps the select
	// scene's record column empty without inventing a score.
	BestCount   uint32
	BestSeconds uint32
}

// CreateTutorialStageInfoReply builds S922: a counted list of cleared stages.
func CreateTutorialStageInfoReply(rows []TutorialStageClear) Packet {
	w := NewWriter()
	w.Unsigned(uint32(len(rows)))
	for _, row := range rows {
		w.Unsigned(row.Stage)
		w.Unsigned(row.BestCount)
		w.Unsigned(row.BestSeconds)
	}
	return Packet{ID: 922, Payload: w.Payload()}
}

// CreateTutorialResultReply builds S732 for a tutorial stage. Unlike an ordinary
// round, whose S732 carries the two team totals, the stage's own result step reads
// a score record: it names the winning side and shows the single seat's row. The
// body is the bare record, with no reward tail.
func CreateTutorialResultReply(row GameResultPlayer, completed bool) (Packet, error) {
	slots, err := resultSlots([]GameResultPlayer{row})
	if err != nil {
		return Packet{}, err
	}
	teamScores := [2]uint16{1, 0}
	winner := uint8(1)
	if !completed {
		teamScores[0], teamScores[1] = 0, 1
		winner = 2
	}
	w := NewWriter()
	writeScoreData(w, teamScores, winner, slots)
	return Packet{ID: 732, Payload: w.Payload()}, nil
}

// TutorialScoreGoal is the counter a stage's own score report raises.
type TutorialScoreGoal uint8

const (
	// TutorialGoalBubble counts the bubbles a bubble weapon pops (knock-downs).
	TutorialGoalBubble TutorialScoreGoal = iota
	// TutorialGoalPang counts a finished target (kills).
	TutorialGoalPang
	// TutorialGoalHelp counts a rescue.
	TutorialGoalHelp
)

// ReadTutorialScoreReport parses C710 gcpGameUpdateScore, the mixed-width report
// the client sends for a bubble or a rescue:
//
//	u8 kind, u32 unknown_8, u32 unknown_12, u8 unknown_16, u8 reporter slot,
//	u8 unknown_18, u16 unknown_20, u16 unknown_22
//
// Kinds 1 and 22 are bubbles and 4 is a rescue; every other kind is somebody
// else's report and is not counted here.
func ReadTutorialScoreReport(p Packet) (TutorialScoreGoal, bool, error) {
	if p.ID != 710 {
		return 0, false, fmt.Errorf("%w: expected UpdateScore", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	kind, err := r.Byte()
	if err != nil {
		return 0, false, err
	}
	for _, width := range []int{4, 4, 1, 1, 1} {
		if err := r.Skip(width); err != nil {
			return 0, false, err
		}
	}
	if _, err := r.Unsigned(); err != nil { // two u16 fields
		return 0, false, err
	}
	if err := r.RequireEnd(); err != nil {
		return 0, false, err
	}
	switch kind {
	case 1, 22:
		return TutorialGoalBubble, true, nil
	case 4:
		return TutorialGoalHelp, true, nil
	}
	return 0, false, nil
}

// ReadTutorialOptimizedScoreReport parses C2023 gcpGameUpdateScoreOptimze, the
// compact report the client sends when it finishes a target: one byte of kind,
// one word and the reporting seat. Kind 2 is a pang.
func ReadTutorialOptimizedScoreReport(p Packet) (TutorialScoreGoal, bool, error) {
	if p.ID != 2023 {
		return 0, false, fmt.Errorf("%w: expected UpdateScoreOptimze", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	kind, err := r.Byte()
	if err != nil {
		return 0, false, err
	}
	if _, err := r.Unsigned(); err != nil {
		return 0, false, err
	}
	if _, err := r.Byte(); err != nil {
		return 0, false, err
	}
	if err := r.RequireEnd(); err != nil {
		return 0, false, err
	}
	if kind == 2 {
		return TutorialGoalPang, true, nil
	}
	return 0, false, nil
}

// CreateTutorialScoreReply builds S729 gspGameBroadcastPlayerScoreData: the bare
// score record a stage's HUD reads after every counted goal. No reward tail goes
// with it — a stage's counters are the whole payload.
//
// cn-717's table resolves the score broadcast to 729 (verified: cn-717 sub_87EFE0
// case 729, and cn-717 sub_76C4D0 consumes the goal fields at row +44/+48/+56);
// 711 is that build's item-creation packet.
func CreateTutorialScoreReply(players []GameResultPlayer) (Packet, error) {
	slots, err := resultSlots(players)
	if err != nil {
		return Packet{}, err
	}
	w := NewWriter()
	writeScoreData(w, [2]uint16{}, 0, slots)
	return Packet{ID: 729, Payload: w.Payload()}, nil
}

// CreateGotoMenuReply builds S108 gspGotoMenu: one flag, cleared here.
func CreateGotoMenuReply() Packet {
	w := NewWriter()
	w.Boolean(false)
	return Packet{ID: 108, Payload: w.Payload()}
}
