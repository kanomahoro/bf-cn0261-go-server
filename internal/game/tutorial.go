package game

import (
	"errors"
	"math"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/world"
)

// The tutorial is the landing page's own single-player mode: the client asks for
// a tutorial session (C405), picks a stage in its select scene (C920/C107), plays
// it, reports its end (C724) and goes back to the select scene or the landing
// page. It never joins the plaza or a room, so this file keeps its whole state on
// the session and sends every packet straight to that session.
const (
	// tutorialSelectKind is the S110 kind whose stage is the select scene.
	tutorialSelectKind = 5
	// tutorialPlayKind is the S110 kind that hands the client into a stage.
	tutorialPlayKind = 2

	// tutorialSeat is the only seat a tutorial session has.
	tutorialSeat = 0

	// tutorialCountdownMs is the fixed countdown the client draws between its
	// load report and the stage start.
	tutorialCountdownMs = 4000

	// tutorialEnterState is the room state S510 carries into the stage. A
	// multiplayer round enters in state 3 (loading); the tutorial owns the stage
	// from the start, so it enters in 4.
	tutorialEnterState = 4

	// tutorialLoadTimeout releases a stage whose load report never arrives, so a
	// client that drops during loading cannot pin the session's state. It is the
	// same window the round path waits for every member.
	tutorialLoadTimeout = loadingTimeout

	// tutorialMaxNpcBody bounds the opaque prologue bodies that are echoed back.
	tutorialMaxNpcBody = 16384
)

// The stage's own ticks come from MapMeta: the fixed countdown between the load
// report and the start, and the two result ticks (EndGameTimeOverTick and
// EndGameNotifyResultTick). They are variables only so a test can shrink them.
var (
	tutorialStartDelay  = tutorialCountdownMs * time.Millisecond
	tutorialScoreDelay  = 3 * time.Second
	tutorialResultDelay = 5 * time.Second
)

// tutorialPhase is what the session is doing in the tutorial flow.
type tutorialPhase uint32

const (
	// tutorialSelecting is the select scene: stages listed, nothing playing.
	tutorialSelecting tutorialPhase = 1
	// tutorialPlaying is a stage: the client loaded a map and reports its end.
	tutorialPlaying tutorialPhase = 2
	// tutorialSettling is a stage whose result was reported: its result ticks run
	// here, and a repeated end report must not start them a second time.
	tutorialSettling tutorialPhase = 3
)

// tutorialState is one session's tutorial flow.
type tutorialState struct {
	phase tutorialPhase
	// stage is the row the client asked to play.
	stage world.TutorialStage
	// difficulty is the row's difficulty tab; mode 103 needs it to find the
	// wave table.
	difficulty uint32
	// timeAttackMs is the row's TimeAttackTimeLimit. The stage's own clock is
	// this value, and the client also reads it as the start's first field.
	timeAttackMs uint32
	// awaitingLoad marks the stage whose load report has not arrived yet.
	awaitingLoad bool
	// bubbles, pangs and helps are the stage's own goal counters; the client's HUD
	// reads them back from every score update this server sends.
	bubbles uint16
	pangs   uint16
	helps   uint16
	// generation guards the timers: a timer from an earlier stage must never act
	// on the state that replaced it.
	generation uint64
	// timers are the pending stage timers, stopped whenever the state is dropped.
	timers []*time.Timer
}

// tutorialCurrent returns a copy of the session's tutorial state.
func (s *Server) tutorialCurrent(session *Session) (tutorialState, bool) {
	session.tutorialMu.Lock()
	defer session.tutorialMu.Unlock()
	if session.tutorial == nil {
		return tutorialState{}, false
	}
	return *session.tutorial, true
}

// tutorialStagePlaying reports whether the session is inside a tutorial stage that
// has not been decided yet.
func (s *Server) tutorialStagePlaying(session *Session) bool {
	session.tutorialMu.Lock()
	defer session.tutorialMu.Unlock()
	return session.tutorial != nil && session.tutorial.phase == tutorialPlaying
}

// tutorialInStage reports whether the session sits in a tutorial stage, where the
// in-match reports replace the lobby ones.
func (s *Server) tutorialInStage(session *Session) bool {
	state, ok := s.tutorialCurrent(session)
	return ok && (state.phase == tutorialPlaying || state.phase == tutorialSettling)
}

// handleTutorialInStageReport swallows the in-match reports a tutorial stage
// sends under numbers the lobby also uses (C14 health reset, C16 recovery). They
// carry nothing this server acts on, and reading them as their lobby meaning
// would end the stage or drop the connection.
func (s *Server) handleTutorialInStageReport(session *Session, packet protocol.Packet) error {
	s.log.Debug("[Tutorial] in-stage report dropped", "session", session.ID, "id", packet.ID,
		"bytes", len(packet.Payload))
	return nil
}

// tutorialBegin installs a fresh tutorial state and returns its generation. Any
// timers left by the previous one are stopped first.
func (s *Server) tutorialBegin(session *Session, state tutorialState) uint64 {
	session.tutorialMu.Lock()
	defer session.tutorialMu.Unlock()
	s.tutorialStopLocked(session)
	session.tutorialSeq++
	state.generation = session.tutorialSeq
	session.tutorial = &state
	return state.generation
}

// settleTutorial moves a stage into its result ticks.
func (s *Server) settleTutorial(session *Session, generation uint64) {
	session.tutorialMu.Lock()
	defer session.tutorialMu.Unlock()
	if session.tutorial == nil || session.tutorial.generation != generation {
		return
	}
	session.tutorial.phase = tutorialSettling
}

// tutorialStopLocked drops the state and stops its timers; the caller holds the
// session's tutorial lock.
func (s *Server) tutorialStopLocked(session *Session) {
	if session.tutorial != nil {
		for _, timer := range session.tutorial.timers {
			if timer != nil {
				timer.Stop()
			}
		}
	}
	session.tutorial = nil
}

// tutorialStop drops the session's tutorial state.
func (s *Server) tutorialStop(session *Session) {
	session.tutorialMu.Lock()
	defer session.tutorialMu.Unlock()
	s.tutorialStopLocked(session)
}

// tutorialAlive reports whether the generation that armed a timer still owns the
// session's tutorial state.
func (s *Server) tutorialAlive(session *Session, generation uint64) bool {
	session.tutorialMu.Lock()
	defer session.tutorialMu.Unlock()
	return session.tutorial != nil && session.tutorial.generation == generation
}

// tutorialKeepTimer records a timer so dropping the state cancels it.
func (s *Server) tutorialKeepTimer(session *Session, generation uint64, timer *time.Timer) {
	session.tutorialMu.Lock()
	defer session.tutorialMu.Unlock()
	if session.tutorial == nil || session.tutorial.generation != generation {
		timer.Stop()
		return
	}
	session.tutorial.timers = append(session.tutorial.timers, timer)
}

// tutorialTakeLoad consumes the stage's load report. It reports the state when
// the report belongs to the stage that is waiting for one.
func (s *Server) tutorialTakeLoad(session *Session) (tutorialState, bool) {
	session.tutorialMu.Lock()
	defer session.tutorialMu.Unlock()
	if session.tutorial == nil || session.tutorial.phase != tutorialPlaying || !session.tutorial.awaitingLoad {
		return tutorialState{}, false
	}
	session.tutorial.awaitingLoad = false
	return *session.tutorial, true
}

// tutorialMember is the session's own seat in the tutorial envelopes: the client
// takes its seat index from it, and the "first seat is the room master" test is
// what lets it build the prologue's spawn groups.
func (s *Server) tutorialMember(session *Session) (protocol.SessionMember, error) {
	character, items := session.Profile()
	wire := s.svc.WireCharacter(character)
	if wire.AccountID == 0 {
		return protocol.SessionMember{}, errNoCharacter
	}
	return protocol.SessionMember{
		Character: &wire,
		Items:     s.svc.WireItems(items),
		// The tutorial's seat belongs to no team: the client's own single-player
		// stage carries 0, and its spawn index stays unset because of it.
		Team: 0,
	}, nil
}

// errNoCharacter reports a tutorial request from a session without a character.
// It is deliberately not a protocol error: a rejected tutorial request must not
// close the connection.
var errNoCharacter = errors.New("tutorial: the session has no character")

// tutorialSelectSetting is the S110 setting the select scene opens on: kind 5,
// mode 0 and every map string empty, so the stage never inherits a stage's mode
// or MapMeta row.
func tutorialSelectSetting() protocol.SessionSetting {
	setting := protocol.DefaultSessionSetting()
	setting.Kind = tutorialSelectKind
	setting.State = 0
	setting.MemberCount = 1
	setting.Capacity = 1
	// Mode 0 keeps a stage's mode and MapMeta row out of the select scene: the
	// client's stage dispatcher only reads the kind, but a mode 103 select envelope
	// sends it down the stage's own map lookup.
	setting.Game.Mode = 0
	// The select scene is neutral: it names no difficulty either (the default
	// record carries 1, which belongs to a play envelope).
	setting.Game.Values148[4] = 0
	return setting
}

// tutorialPlaySetting is the S110 setting a stage opens on.
func tutorialPlaySetting(stage world.TutorialStage, directory string, difficulty uint32, timeAttackMs uint32) protocol.SessionSetting {
	setting := protocol.DefaultSessionSetting()
	setting.Kind = tutorialPlayKind
	setting.State = 0
	setting.MemberCount = 1
	setting.Capacity = 1
	setting.Game.Mode = protocol.TutorialSinglePlayMode
	if stage.Prologue {
		// The prologue is the wave tutorial: its spawn groups and wave table only
		// exist under mode 103.
		setting.Game.Mode = protocol.TutorialHordeMode
	}
	if directory == "" {
		directory = stage.Key
	}
	// Strings16 order is the client's own GameSetting: directory (+16), current
	// map (+44), NPC table key (+72), map key (+100). The script map comes from
	// the current map, so the NPC slot repeats it.
	setting.Game.Strings16 = [4][]uint16{
		protocol.U16String(directory),
		protocol.U16String(stage.Key),
		protocol.U16String(stage.Key),
		protocol.U16String(stage.Key),
	}
	// Values148[4] is GameSetting +164, the Lua difficulty the wave tables are
	// indexed by.
	setting.Game.Values148[4] = difficulty
	// Values128[2] is the round duration the client mirrors into its stage.
	setting.Game.Values128[2] = timeAttackMs
	return setting
}

// handleTutorialCreate answers C405 gcpReqCreateTutorialSession: the landing
// page's tutorial button. The request must be empty; a malformed one is dropped
// without touching the connection.
func (s *Server) handleTutorialCreate(session *Session, packet protocol.Packet) error {
	if len(packet.Payload) != 0 {
		s.log.Warn("[Tutorial] the create request must be empty",
			"session", session.ID, "bytes", len(packet.Payload))
		return nil
	}
	if session.room != nil {
		s.log.Debug("[Tutorial] ignored while the session sits in a room", "session", session.ID)
		return nil
	}
	// The tutorial is opened from the landing page. A plaza seat would keep the
	// character listed there, so it is dropped first.
	if session.inPlaza {
		s.leavePlaza(session)
	}
	if err := s.sendTutorialSelect(session); err != nil {
		s.log.Warn("[Tutorial] the select scene could not be opened",
			"session", session.ID, "error", err)
		return nil
	}
	s.log.Info("[Tutorial] select", "session", session.ID)
	return nil
}

// sendTutorialSelect opens the select scene and moves the session into it.
func (s *Server) sendTutorialSelect(session *Session) error {
	// The select scene replaces whatever the session was doing: its timers stop
	// before the envelope goes out, so a failed send cannot leave them armed.
	s.tutorialBegin(session, tutorialState{phase: tutorialSelecting})
	member, err := s.tutorialMember(session)
	if err != nil {
		return err
	}
	reply, err := protocol.CreateRoomEntryReply(tutorialSelectSetting(), tutorialSeat, tutorialSeat,
		[]protocol.SessionMember{member})
	if err != nil {
		return err
	}
	return session.Send(reply)
}

// handleTutorialStageList answers C920 gcpReqSingleStageInfo with every stage
// cleared: the master's decision is to open the whole list rather than serve a
// progression the database does not carry.
func (s *Server) handleTutorialStageList(session *Session, packet protocol.Packet) error {
	if len(packet.Payload) != 0 {
		s.log.Warn("[Tutorial] the stage list request must be empty",
			"session", session.ID, "bytes", len(packet.Payload))
		return nil
	}
	if _, ok := s.tutorialCurrent(session); !ok {
		s.log.Debug("[Tutorial] stage list outside a tutorial session", "session", session.ID)
		return nil
	}
	stages := world.TutorialStages()
	rows := make([]protocol.TutorialStageClear, 0, len(stages))
	for _, stage := range stages {
		rows = append(rows, protocol.TutorialStageClear{Stage: stage.Num})
	}
	if err := session.Send(protocol.CreateTutorialStageInfoReply(rows)); err != nil {
		return err
	}
	s.log.Info("[Tutorial] stage list", "session", session.ID, "stages", len(rows))
	return nil
}

// handleTutorialPlay answers C107 gcpReqPlayTutorial: it opens the stage room,
// pushes the seat and enters the stage, which is what the client waits for after
// its own five-second callback.
func (s *Server) handleTutorialPlay(session *Session, packet protocol.Packet) error {
	request, err := protocol.ReadTutorialPlayRequest(packet)
	if err != nil {
		s.log.Warn("[Tutorial] unreadable play request", "session", session.ID, "error", err)
		return nil
	}
	stage, ok := world.FindTutorialStage(request.MapKey)
	if !ok {
		s.log.Warn("[Tutorial] the request names a stage outside the tutorial list",
			"session", session.ID, "map", request.MapKey)
		return nil
	}
	if request.Difficulty == 0 {
		s.log.Warn("[Tutorial] the request carries no difficulty; the wave tables would be empty",
			"session", session.ID, "map", stage.Key)
		return nil
	}
	if request.TimeAttackMs == 0 {
		s.log.Warn("[Tutorial] the request carries no TimeAttack limit; the letterbox would stay up",
			"session", session.ID, "map", stage.Key)
		return nil
	}
	if session.room != nil {
		// A room owns this session: a stage would fight the round for its load
		// report and its score reports.
		s.log.Warn("[Tutorial] a stage cannot open while the session sits in a room",
			"session", session.ID)
		return nil
	}
	member, err := s.tutorialMember(session)
	if err != nil {
		s.log.Warn("[Tutorial] the session cannot be seated in a stage",
			"session", session.ID, "error", err)
		return nil
	}
	setting := tutorialPlaySetting(stage, request.Directory, request.Difficulty, request.TimeAttackMs)
	entry, err := protocol.CreateRoomEntryReply(setting, tutorialSeat, tutorialSeat,
		[]protocol.SessionMember{member})
	if err != nil {
		return err
	}
	change, err := protocol.CreateRoomGameSettingReply(setting.Game)
	if err != nil {
		return err
	}
	slot, err := protocol.CreateRoomSlotReply(tutorialSeat, member)
	if err != nil {
		return err
	}
	start := protocol.CreateGameEntryReply(protocol.StartSuccess, protocol.U16String(stage.Key), tutorialEnterState)

	generation := s.tutorialBegin(session, tutorialState{
		phase:        tutorialPlaying,
		stage:        stage,
		difficulty:   request.Difficulty,
		timeAttackMs: request.TimeAttackMs,
		awaitingLoad: true,
	})
	for _, outgoing := range []protocol.Packet{entry, change, slot, start} {
		if err := session.Send(outgoing); err != nil {
			return err
		}
	}
	s.log.Info("[Tutorial] stage opened", "session", session.ID, "map", stage.Key,
		"stageNum", stage.Num, "prologue", stage.Prologue, "difficulty", request.Difficulty,
		"timeAttackMs", request.TimeAttackMs,
		"directory", protocol.FromU16(setting.Game.Strings16[0]), "flags52", request.Flags52)
	s.armTutorialLoadTimeout(session, generation)
	return nil
}

// armTutorialLoadTimeout releases a stage whose load report never arrives.
func (s *Server) armTutorialLoadTimeout(session *Session, generation uint64) {
	timer := time.AfterFunc(tutorialLoadTimeout, func() {
		if !s.tutorialAlive(session, generation) {
			return
		}
		if state, ok := s.tutorialCurrent(session); ok && !state.awaitingLoad {
			return
		}
		s.log.Warn("[Tutorial] the stage never reported its load; dropping it", "session", session.ID)
		s.tutorialStop(session)
	})
	s.tutorialKeepTimer(session, generation, timer)
}

// handleTutorialMapLoaded answers the stage's own load report: the client waits
// for the countdown and then for the start, which carries the stage's clock.
// It reports whether the packet belonged to a tutorial stage.
func (s *Server) handleTutorialMapLoaded(session *Session) bool {
	if session.room != nil {
		return false
	}
	state, ok := s.tutorialTakeLoad(session)
	if !ok {
		return false
	}
	delay := tutorialStartDelay
	mapLoaded, err := protocol.CreateMapLoadedReply(tutorialSeat)
	if err != nil {
		s.log.Warn("[Tutorial] map loaded reply failed", "session", session.ID, "error", err)
		return true
	}
	for _, outgoing := range []protocol.Packet{
		mapLoaded,
		protocol.CreateEverybodyLoadedReply(),
		protocol.CreateGameCountdownReply(uint32(delay / time.Millisecond)),
	} {
		if err := session.Send(outgoing); err != nil {
			return true
		}
	}
	timer := time.AfterFunc(delay, func() {
		if !s.tutorialAlive(session, state.generation) {
			return
		}
		// The start's first field arms the stage's clock and clears the letterbox;
		// the third mirrors it into the session. Zero leaves both unset.
		start := protocol.CreateGameStartReply(state.timeAttackMs, state.timeAttackMs)
		if err := session.Send(start); err != nil {
			s.log.Debug("[Tutorial] stage start failed", "session", session.ID, "error", err)
			return
		}
		s.log.Info("[Tutorial] stage started", "session", session.ID, "map", state.stage.Key,
			"clockMs", state.timeAttackMs)
	})
	s.tutorialKeepTimer(session, state.generation, timer)
	return true
}

// handleTutorialEndStage answers C724 gcpReqEndStage: the client reports whether
// the stage was cleared. A numbered stage shows the result ticks and returns to
// the select scene; the prologue ends straight into the landing page.
func (s *Server) handleTutorialEndStage(session *Session, packet protocol.Packet) error {
	state, ok := s.tutorialCurrent(session)
	if !ok || state.phase != tutorialPlaying {
		s.log.Debug("[Tutorial] stage end outside a stage", "session", session.ID, "id", packet.ID)
		return nil
	}
	completed, err := protocol.ReadTutorialEndStage(packet)
	if err != nil {
		s.log.Warn("[Tutorial] unreadable stage end", "session", session.ID, "error", err)
		return nil
	}
	if err := session.Send(protocol.CreateGameOutcomeReply(completed)); err != nil {
		return err
	}
	// The stage is decided: its timers own the session from here, so a second end
	// report (the client sends one on exit as well) is ignored instead of starting
	// the result chain twice.
	s.settleTutorial(session, state.generation)
	s.log.Info("[Tutorial] stage ended", "session", session.ID, "map", state.stage.Key,
		"completed", completed, "prologue", state.stage.Prologue)
	if state.stage.Prologue {
		s.leaveTutorialForMenu(session, state.generation, false)
		return nil
	}
	s.scheduleTutorialResult(session, state.generation, completed)
	return nil
}

// scheduleTutorialResult walks a numbered stage through its own result ticks and
// then back to the select scene.
func (s *Server) scheduleTutorialResult(session *Session, generation uint64, completed bool) {
	// The row is built once, on the stage's own goroutine, so the timer never
	// reads the session's profile.
	row, ok := s.tutorialResultRow(session)
	scoreDelay, resultDelay := tutorialScoreDelay, tutorialResultDelay
	scoreTimer := time.AfterFunc(scoreDelay, func() {
		if !s.tutorialAlive(session, generation) {
			return
		}
		if !ok {
			return
		}
		winner, err := protocol.CreateTutorialResultReply(row, completed)
		if err != nil {
			s.log.Warn("[Tutorial] the result tick could not be built",
				"session", session.ID, "error", err)
			return
		}
		if err := session.Send(winner); err != nil {
			return
		}
		resultTimer := time.AfterFunc(resultDelay, func() {
			if !s.tutorialAlive(session, generation) {
				return
			}
			if err := s.sendTutorialSelect(session); err != nil {
				s.log.Warn("[Tutorial] return to the select scene failed",
					"session", session.ID, "error", err)
				return
			}
			s.log.Info("[Tutorial] back to the select scene", "session", session.ID)
		})
		s.tutorialKeepTimer(session, generation, resultTimer)
	})
	s.tutorialKeepTimer(session, generation, scoreTimer)
}

// tutorialCount raises one of the stage's own counters and reports the stage the
// report belongs to. The counters live only in this state: nothing is written to
// the database, so a stage's tally ends with it.
func (s *Server) tutorialCount(session *Session, goal protocol.TutorialScoreGoal) (tutorialState, bool) {
	session.tutorialMu.Lock()
	defer session.tutorialMu.Unlock()
	if session.tutorial == nil || session.tutorial.phase != tutorialPlaying {
		return tutorialState{}, false
	}
	// The counters are native ushorts: a stage that somehow reported past the
	// ceiling keeps the last value instead of wrapping to zero.
	switch goal {
	case protocol.TutorialGoalBubble:
		if session.tutorial.bubbles < math.MaxUint16 {
			session.tutorial.bubbles++
		}
	case protocol.TutorialGoalPang:
		if session.tutorial.pangs < math.MaxUint16 {
			session.tutorial.pangs++
		}
	case protocol.TutorialGoalHelp:
		if session.tutorial.helps < math.MaxUint16 {
			session.tutorial.helps++
		}
	default:
		return tutorialState{}, false
	}
	return *session.tutorial, true
}

// handleTutorialScore answers the stage's own goal reports (C710 for a bubble or a
// rescue, C2023 for a finished target) and sends the updated counters back. It
// reports whether the packet belonged to a tutorial stage.
func (s *Server) handleTutorialScore(session *Session, packet protocol.Packet) bool {
	if !s.tutorialStagePlaying(session) {
		return false
	}
	var goal protocol.TutorialScoreGoal
	var raised bool
	var err error
	switch packet.ID {
	case 710:
		goal, raised, err = protocol.ReadTutorialScoreReport(packet)
	case 2023:
		goal, raised, err = protocol.ReadTutorialOptimizedScoreReport(packet)
	}
	if err != nil {
		// A score report the layout does not fit costs that report, not the stage.
		s.log.Debug("[Tutorial] unreadable score report", "session", session.ID,
			"id", packet.ID, "error", err)
		return true
	}
	if !raised {
		return true
	}
	state, ok := s.tutorialCount(session, goal)
	if !ok {
		return true
	}
	row, ok := s.tutorialResultRow(session)
	if !ok {
		return true
	}
	row.Bubbles, row.Pangs, row.Rescues = state.bubbles, state.pangs, state.helps
	// The live tally is neutral: the client's own tutorial report names no team.
	row.Team = 0
	update, err := protocol.CreateTutorialScoreReply([]protocol.GameResultPlayer{row})
	if err != nil {
		s.log.Warn("[Tutorial] the score update could not be built",
			"session", session.ID, "error", err)
		return true
	}
	if err := session.Send(update); err != nil {
		return true
	}
	s.log.Info("[Tutorial] goal counted", "session", session.ID, "map", state.stage.Key,
		"bubbles", state.bubbles, "pangs", state.pangs, "helps", state.helps)
	return true
}

// tutorialResultRow is the one row the stage's scoreboard carries: the player
// alone on team 1, with no grading (the tutorial awards nothing).
func (s *Server) tutorialResultRow(session *Session) (protocol.GameResultPlayer, bool) {
	character, _ := session.Profile()
	wire := s.svc.WireCharacter(character)
	if wire.AccountID == 0 || protocol.CheckNickname(wire.Nickname) != protocol.NicknameOK {
		return protocol.GameResultPlayer{}, false
	}
	level := wire.Level
	if level == 0 {
		level = 1
	}
	return protocol.GameResultPlayer{
		Slot:       tutorialSeat,
		AccountID:  wire.AccountID,
		Nickname:   wire.Nickname,
		Team:       1,
		Level:      level,
		LevelStart: 0,
		Experience: 0,
		NextLevel:  0,
		AwardGrade: protocol.Unset,
		AwardMedal: 0,
		AwardScore: 0,
		LucciGain:  0,
	}, true
}

// leaveTutorialForMenu returns the session to the landing page the tutorial was
// opened from. A stage that ends sends only the envelope, the way the client's own
// single-player stage does; a client that leaves the tutorial asks for the scene
// transition as well.
func (s *Server) leaveTutorialForMenu(session *Session, generation uint64, transition bool) {
	if generation != 0 && !s.tutorialAlive(session, generation) {
		return
	}
	s.tutorialStop(session)
	character, items := session.Profile()
	menu, err := protocol.CreateMainMenuReply(s.svc.WireCharacter(character), s.svc.WireItems(items))
	if err != nil {
		s.log.Warn("[Tutorial] main menu envelope failed", "session", session.ID, "error", err)
		return
	}
	if err := session.Send(menu); err != nil {
		return
	}
	if transition {
		if err := session.Send(protocol.CreateGotoMenuReply()); err != nil {
			return
		}
	}
	s.log.Info("[Tutorial] back to the landing page", "session", session.ID, "transition", transition)
}

// handleTutorialLeave answers C404 while the session sits in the tutorial: the
// client is going back to the landing page, so the flow is dropped and the menu
// envelope is sent in its place. It reports whether the leave belonged to a
// tutorial session.
func (s *Server) handleTutorialLeave(session *Session, packet protocol.Packet) bool {
	if _, ok := s.tutorialCurrent(session); !ok {
		return false
	}
	if session.room != nil {
		// A room owns this session now; the tutorial flow is left over from the
		// landing page and must not answer the room's leave request.
		s.tutorialStop(session)
		return false
	}
	if len(packet.Payload) != 0 {
		s.log.Warn("[Tutorial] the leave request must be empty", "session", session.ID)
		return true
	}
	s.log.Info("[Tutorial] left from the client", "session", session.ID)
	s.leaveTutorialForMenu(session, 0, true)
	return true
}

// tutorialNpcPacketID reports whether the packet belongs to the prologue's own
// NPC traffic.
func tutorialNpcPacketID(id uint16) bool {
	switch id {
	case 32857, 32862, 32892: // spawn group, its member mobs, one AI object
		return true
	case 32920: // gppLifePoint: the host's own mob life points
		return true
	case 903, 904, 905, 908: // wave start, wave end, all waves done, wave table
		return true
	}
	return false
}

// handleTutorialNpc answers the prologue's NPC traffic by echoing the host body
// back, which is what the client's own single-player stage expects. Nothing here
// runs outside a tutorial stage, so the ordinary 32xxx relay of the multiplayer
// modes is untouched.
func (s *Server) handleTutorialNpc(session *Session, packet protocol.Packet) bool {
	state, ok := s.tutorialCurrent(session)
	if !ok || state.phase != tutorialPlaying {
		return false
	}
	switch packet.ID {
	case 32857, 32862, 32892, 32920:
		// Opaque host bodies: the spawn-group index, one member mob, one AI object, and
		// a mob's life points. Without the life-point echo the local mob spends no
		// health and pops at the first hit instead of bubbling.
		if len(packet.Payload) == 0 || len(packet.Payload) > tutorialMaxNpcBody {
			s.log.Warn("[Tutorial] NPC body out of bounds", "session", session.ID,
				"id", packet.ID, "bytes", len(packet.Payload))
			return true
		}
		_ = session.Send(packet)
		s.log.Info("[Tutorial] spawn group echo", "session", session.ID, "id", packet.ID,
			"bytes", len(packet.Payload))
		return true
	case 903, 904:
		wave, err := tutorialWaveWord(packet)
		if err != nil {
			s.log.Warn("[Tutorial] unreadable wave report", "session", session.ID,
				"id", packet.ID, "error", err)
			return true
		}
		reply := uint16(904)
		if packet.ID == 904 {
			reply = 905
		}
		_ = session.Send(protocol.Packet{ID: reply, Payload: packet.Payload})
		s.log.Info("[Tutorial] wave", "session", session.ID, "wave", wave, "reply", reply)
		return true
	case 908:
		if len(packet.Payload) == 0 || len(packet.Payload) > tutorialMaxNpcBody {
			return true
		}
		_ = session.Send(protocol.Packet{ID: 908})
		_ = session.Send(protocol.Packet{ID: 909, Payload: packet.Payload})
		s.log.Info("[Tutorial] wave table echoed", "session", session.ID, "bytes", len(packet.Payload))
		return true
	case 905:
		// The host reports every wave done: the prologue is complete.
		if len(packet.Payload) != 0 {
			return true
		}
		if err := session.Send(protocol.CreateGameOutcomeReply(true)); err != nil {
			return true
		}
		s.log.Info("[Tutorial] prologue cleared every wave", "session", session.ID)
		s.leaveTutorialForMenu(session, state.generation, false)
		return true
	}
	return false
}

// tutorialWaveWord reads the one-word wave report.
func tutorialWaveWord(packet protocol.Packet) (uint32, error) {
	r := protocol.NewReader(packet.Payload)
	wave, err := r.Unsigned()
	if err != nil {
		return 0, err
	}
	if err := r.RequireEnd(); err != nil {
		return 0, err
	}
	return wave, nil
}
