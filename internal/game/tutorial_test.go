package game

import (
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/features"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/store"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/world"
)

// tutorialTestServer is a server whose only wired dependency is the profile
// projection the tutorial envelopes need.
func tutorialTestServer() *Server {
	return &Server{
		log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		svc: features.NewService(nil, world.Progression{}),
	}
}

// tutorialRecorder captures what the tutorial sends to its session. The stage's
// timers send from their own goroutines, so the list is guarded.
type tutorialRecorder struct {
	mu   sync.Mutex
	sent []protocol.Packet
}

func (r *tutorialRecorder) Send(packet protocol.Packet) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, packet)
	return nil
}

func (r *tutorialRecorder) ids() []uint16 {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]uint16, 0, len(r.sent))
	for _, packet := range r.sent {
		out = append(out, packet.ID)
	}
	return out
}

func (r *tutorialRecorder) first(id uint16) (protocol.Packet, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, packet := range r.sent {
		if packet.ID == id {
			return packet, true
		}
	}
	return protocol.Packet{}, false
}

func (r *tutorialRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = nil
}

func (r *tutorialRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.sent)
}

// tutorialTestSession is a logged-in session with no room and no plaza.
func tutorialTestSession(recorder *tutorialRecorder) *Session {
	session := &Session{
		ID:        1,
		Account:   store.Account{ID: 7},
		Character: store.Character{ID: 1, AccountID: 7, Nickname: "tester", CharacterType: 1, Experience: 0},
		send:      recorder,
	}
	return session
}

// tutorialPlayPacket builds C107 the way the client serializes it: thirteen
// MapMeta words, six flag words, three wide strings and the difficulty.
func tutorialPlayPacket(mapKey, directory, display string, difficulty, timeAttackMs uint32) protocol.Packet {
	w := protocol.NewWriter()
	for index := 0; index < 13; index++ {
		value := uint32(index + 100)
		if index == 8 {
			value = timeAttackMs
		}
		w.Unsigned(value)
	}
	for index := 0; index < 6; index++ {
		w.Unsigned(0)
	}
	w.String(protocol.U16String(directory))
	w.String(protocol.U16String(display))
	w.String(protocol.U16String(mapKey))
	w.Unsigned(difficulty)
	return protocol.Packet{ID: 107, Payload: w.Payload()}
}

// tutorialScoreboard decodes an S732 body: the result record's team count, its
// winner and the first two team totals.
func tutorialScoreboard(t *testing.T, payload []byte) (count, winner, first, second uint32) {
	t.Helper()
	r := protocol.NewReader(payload)
	var err error
	if count, err = r.Unsigned(); err != nil {
		t.Fatalf("read the team count: %v", err)
	}
	if winner, err = r.Unsigned(); err != nil {
		t.Fatalf("read the winner: %v", err)
	}
	if _, err = r.Unsigned(); err != nil {
		t.Fatalf("read the trailing flag: %v", err)
	}
	if count != 17 {
		t.Fatalf("team count = %d, want 17", count)
	}
	for team := uint32(1); team <= 2; team++ {
		if _, err := r.Unsigned(); err != nil {
			t.Fatalf("team %d prefix: %v", team, err)
		}
		total, err := r.Unsigned()
		if err != nil {
			t.Fatalf("team %d total: %v", team, err)
		}
		if team == 1 {
			first = total
		} else {
			second = total
		}
		for word := 0; word < 2; word++ {
			if _, err := r.Unsigned(); err != nil {
				t.Fatalf("team %d suffix: %v", team, err)
			}
		}
	}
	return count, winner, first, second
}

func tutorialPacket(id uint16, payload []byte) protocol.Packet {
	return protocol.Packet{ID: id, Payload: payload}
}

// The tutorial button's request must be empty: a body is dropped without
// touching the connection and without opening the select scene.
func TestTutorialCreateRejectsANonEmptyRequest(t *testing.T) {
	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	if err := server.handleTutorialCreate(session, tutorialPacket(405, []byte{1, 2, 3})); err != nil {
		t.Fatalf("the request must be dropped without an error: %v", err)
	}
	if recorder.count() != 0 {
		t.Fatalf("a malformed create request must send nothing, got %v", recorder.ids())
	}
	if _, ok := server.tutorialCurrent(session); ok {
		t.Fatal("a malformed create request must not open a tutorial session")
	}
}

// The select scene opens on mode 0 with every map string empty and one seat: the
// stage's own mode and MapMeta row must never ride on it.
func TestTutorialSelectOpensOnAnEmptySetting(t *testing.T) {
	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	if err := server.handleTutorialCreate(session, tutorialPacket(405, nil)); err != nil {
		t.Fatalf("create: %v", err)
	}
	packet, ok := recorder.first(110)
	if !ok {
		t.Fatalf("the select scene must be an S110 envelope, got %v", recorder.ids())
	}
	setting, err := protocol.ReadSessionSetting(protocol.NewReader(packet.Payload))
	if err != nil {
		t.Fatalf("read the envelope: %v", err)
	}
	if setting.Kind != tutorialSelectKind {
		t.Fatalf("select kind = %d, want %d", setting.Kind, tutorialSelectKind)
	}
	if setting.Game.Mode != 0 {
		t.Fatalf("select mode = %d, want 0", setting.Game.Mode)
	}
	for index, value := range setting.Game.Strings16 {
		if len(value) != 0 {
			t.Fatalf("select map string %d must be empty", index)
		}
	}
	state, ok := server.tutorialCurrent(session)
	if !ok || state.phase != tutorialSelecting {
		t.Fatalf("the session must sit in the select scene, got %+v", state)
	}
}

// The stage list answers with all 25 rows, keyed by the stage numbers the client
// looks its rows up by.
func TestTutorialStageListCarriesEveryStage(t *testing.T) {
	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	if err := server.handleTutorialCreate(session, tutorialPacket(405, nil)); err != nil {
		t.Fatalf("create: %v", err)
	}
	recorder.reset()
	if err := server.handleTutorialStageList(session, tutorialPacket(920, nil)); err != nil {
		t.Fatalf("stage list: %v", err)
	}
	packet, ok := recorder.first(922)
	if !ok {
		t.Fatalf("the stage list must answer with S922, got %v", recorder.ids())
	}
	r := protocol.NewReader(packet.Payload)
	count, err := r.Unsigned()
	if err != nil {
		t.Fatalf("read the count: %v", err)
	}
	want := []uint32{0, 1, 2, 3, 4, 5, 6, 7, 8, 11, 12, 13, 14, 15, 16, 17, 18, 21, 22, 23, 24, 25, 26, 27, 28}
	if int(count) != len(want) {
		t.Fatalf("stage count = %d, want %d", count, len(want))
	}
	for index, stage := range want {
		got, err := r.Unsigned()
		if err != nil {
			t.Fatalf("row %d: %v", index, err)
		}
		if got != stage {
			t.Fatalf("row %d stage = %d, want %d", index, got, stage)
		}
		for field := 0; field < 2; field++ {
			if _, err := r.Unsigned(); err != nil {
				t.Fatalf("row %d record: %v", index, err)
			}
		}
	}
	if err := r.RequireEnd(); err != nil {
		t.Fatalf("trailing bytes: %v", err)
	}
}

// A play request outside the released list, without a difficulty or without a
// TimeAttack limit is dropped: any of the three leaves the stage unable to run.
func TestTutorialPlayRejectsUnusableRequests(t *testing.T) {
	cases := []struct {
		name   string
		packet protocol.Packet
	}{
		{"unknown stage", tutorialPlayPacket("rel_unknown_01", "Forest", "x", 1, 180000)},
		{"no difficulty", tutorialPlayPacket("sin_mission_pang_lv_01", "Forest", "x", 0, 180000)},
		{"no time attack", tutorialPlayPacket("sin_mission_pang_lv_01", "Forest", "x", 1, 0)},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			server := tutorialTestServer()
			recorder := &tutorialRecorder{}
			session := tutorialTestSession(recorder)
			if err := server.handleTutorialCreate(session, tutorialPacket(405, nil)); err != nil {
				t.Fatalf("create: %v", err)
			}
			recorder.reset()
			if err := server.handleTutorialPlay(session, testCase.packet); err != nil {
				t.Fatalf("the request must be dropped without an error: %v", err)
			}
			if recorder.count() != 0 {
				t.Fatalf("a rejected request must send nothing, got %v", recorder.ids())
			}
			state, ok := server.tutorialCurrent(session)
			if !ok || state.phase != tutorialSelecting {
				t.Fatalf("the session must stay in the select scene, got %+v", state)
			}
		})
	}
}

// A numbered stage enters mode 15, a prologue enters mode 103, and both carry the
// row's directory, current map and difficulty.
func TestTutorialPlayOpensTheStageMode(t *testing.T) {
	cases := []struct {
		name     string
		mapKey   string
		wantMode uint32
	}{
		{"prologue", "rel_prologue_01", protocol.TutorialHordeMode},
		{"numbered stage", "sin_mission_wall_lv_02", protocol.TutorialSinglePlayMode},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			server := tutorialTestServer()
			recorder := &tutorialRecorder{}
			session := tutorialTestSession(recorder)
			if err := server.handleTutorialCreate(session, tutorialPacket(405, nil)); err != nil {
				t.Fatalf("create: %v", err)
			}
			recorder.reset()
			request := tutorialPlayPacket(testCase.mapKey, "Forest", "display", 2, 210000)
			if err := server.handleTutorialPlay(session, request); err != nil {
				t.Fatalf("play: %v", err)
			}
			packet, ok := recorder.first(110)
			if !ok {
				t.Fatalf("the stage must open with an S110 envelope, got %v", recorder.ids())
			}
			setting, err := protocol.ReadSessionSetting(protocol.NewReader(packet.Payload))
			if err != nil {
				t.Fatalf("read the envelope: %v", err)
			}
			if setting.Kind != tutorialPlayKind {
				t.Fatalf("play kind = %d, want %d", setting.Kind, tutorialPlayKind)
			}
			if setting.Game.Mode != testCase.wantMode {
				t.Fatalf("mode = %d, want %d", setting.Game.Mode, testCase.wantMode)
			}
			if got := protocol.FromU16(setting.Game.Strings16[0]); got != "Forest" {
				t.Fatalf("directory = %q, want Forest", got)
			}
			for _, index := range []int{1, 2, 3} {
				if got := protocol.FromU16(setting.Game.Strings16[index]); got != testCase.mapKey {
					t.Fatalf("map string %d = %q, want %q", index, got, testCase.mapKey)
				}
			}
			if setting.Game.Values148[4] != 2 {
				t.Fatalf("difficulty = %d, want 2", setting.Game.Values148[4])
			}
			state, ok := server.tutorialCurrent(session)
			if !ok || state.phase != tutorialPlaying || !state.awaitingLoad {
				t.Fatalf("the session must wait for the stage's load, got %+v", state)
			}
			// The stage start follows the game setting and the seat, and carries the
			// row's clock in the state the client enters on. The session signature is
			// not part of the tutorial entry (Fantasy's own entry omits it either).
			for _, id := range []uint16{505, 503} {
				if _, ok := recorder.first(id); !ok {
					t.Fatalf("the entry burst is missing %d: %v", id, recorder.ids())
				}
			}
			start, ok := recorder.first(510)
			if !ok {
				t.Fatalf("the entry burst is missing S510: %v", recorder.ids())
			}
			r := protocol.NewReader(start.Payload)
			if _, err := r.Unsigned(); err != nil {
				t.Fatalf("read the entry result: %v", err)
			}
			if _, err := r.String(protocol.SessionStringLimit); err != nil {
				t.Fatalf("read the entry map: %v", err)
			}
			for index := 0; index < 2; index++ {
				if _, err := r.Unsigned(); err != nil {
					t.Fatalf("read entry word %d: %v", index, err)
				}
			}
			stateWord, err := r.Unsigned()
			if err != nil {
				t.Fatalf("read the entry state: %v", err)
			}
			if stateWord != tutorialEnterState {
				t.Fatalf("entry state = %d, want %d", stateWord, tutorialEnterState)
			}
		})
	}
}

// The load report answers with the gate, the everybody-loaded notice and the
// fixed tutorial countdown, and the start that follows carries the stage clock.
func TestTutorialLoadReportStartsTheStage(t *testing.T) {
	previous := tutorialStartDelay
	tutorialStartDelay = 20 * time.Millisecond
	defer func() { tutorialStartDelay = previous }()

	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	if err := server.handleTutorialCreate(session, tutorialPacket(405, nil)); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := server.handleTutorialPlay(session,
		tutorialPlayPacket("sin_mission_roll_lv_01", "Forest", "display", 1, 90000)); err != nil {
		t.Fatalf("play: %v", err)
	}
	recorder.reset()
	if !server.handleTutorialMapLoaded(session) {
		t.Fatal("the stage's load report must belong to the tutorial")
	}
	for _, id := range []uint16{701, 702, 703} {
		if _, ok := recorder.first(id); !ok {
			t.Fatalf("the load answer is missing %d: %v", id, recorder.ids())
		}
	}
	countdown, _ := recorder.first(703)
	want := uint32(tutorialStartDelay / time.Millisecond)
	if got := countdown.Payload; len(got) != 4 || got[0] != byte(want) {
		t.Fatalf("countdown payload = %x, want %d ms", got, want)
	}
	deadline := time.Now().Add(time.Second)
	for {
		if start, ok := recorder.first(704); ok {
			r := protocol.NewReader(start.Payload)
			clock, err := r.Unsigned()
			if err != nil {
				t.Fatalf("read the start clock: %v", err)
			}
			if clock != 90000 {
				t.Fatalf("start clock = %d, want 90000", clock)
			}
			mirror, err := r.Unsigned()
			if err != nil {
				t.Fatalf("read the start word: %v", err)
			}
			if mirror != 0 {
				t.Fatalf("start word = %d, want 0", mirror)
			}
			second, err := r.Unsigned()
			if err != nil {
				t.Fatalf("read the mirrored clock: %v", err)
			}
			if second != 90000 {
				t.Fatalf("mirrored clock = %d, want 90000", second)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the stage never started: %v", recorder.ids())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// A stage end reports its outcome at once, and a numbered stage walks the two
// result ticks before it returns to the select scene.
func TestTutorialStageEndWalksTheResultTicks(t *testing.T) {
	previousScore, previousResult := tutorialScoreDelay, tutorialResultDelay
	tutorialScoreDelay, tutorialResultDelay = 10*time.Millisecond, 10*time.Millisecond
	defer func() {
		tutorialScoreDelay, tutorialResultDelay = previousScore, previousResult
	}()

	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	if err := server.handleTutorialCreate(session, tutorialPacket(405, nil)); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := server.handleTutorialPlay(session,
		tutorialPlayPacket("sin_mission_help_lv_01", "Forest", "display", 1, 180000)); err != nil {
		t.Fatalf("play: %v", err)
	}
	recorder.reset()
	completed := protocol.NewWriter()
	completed.Unsigned(1)
	if err := server.handleTutorialEndStage(session, tutorialPacket(724, completed.Payload())); err != nil {
		t.Fatalf("end stage: %v", err)
	}
	end, ok := recorder.first(705)
	if !ok {
		t.Fatalf("the stage end must report at once, got %v", recorder.ids())
	}
	r := protocol.NewReader(end.Payload)
	if _, err := r.Unsigned(); err != nil {
		t.Fatalf("read the end word: %v", err)
	}
	outcome, err := r.Unsigned()
	if err != nil {
		t.Fatalf("read the outcome: %v", err)
	}
	if outcome != 3 {
		t.Fatalf("a cleared stage reports 3, got %d", outcome)
	}
	deadline := time.Now().Add(time.Second)
	var selectPacket protocol.Packet
	for {
		if packet, ok := recorder.first(110); ok {
			selectPacket = packet
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the stage never returned to the select scene: %v", recorder.ids())
		}
		time.Sleep(5 * time.Millisecond)
	}
	selectSetting, err := protocol.ReadSessionSetting(protocol.NewReader(selectPacket.Payload))
	if err != nil {
		t.Fatalf("read the returned select scene: %v", err)
	}
	if selectSetting.Kind != tutorialSelectKind {
		t.Fatalf("returned kind = %d, want %d", selectSetting.Kind, tutorialSelectKind)
	}
	winner, ok := recorder.first(732)
	if !ok {
		t.Fatalf("the result tick is missing S732: %v", recorder.ids())
	}
	// The tutorial's own result stage reads a scoreboard, not the two team bytes
	// an ordinary round carries.
	_, winnerTeam, firstTotal, secondTotal := tutorialScoreboard(t, winner.Payload)
	if winnerTeam != 1 || firstTotal != 1 || secondTotal != 0 {
		t.Fatalf("S732 = winner %d totals %d:%d, want 1 and 1:0", winnerTeam, firstTotal, secondTotal)
	}
	state, ok := server.tutorialCurrent(session)
	if !ok || state.phase != tutorialSelecting {
		t.Fatalf("the session must be back in the select scene, got %+v", state)
	}
}

// A lost stage reports its own outcome and the mirrored winner.
func TestTutorialStageEndReportsALoss(t *testing.T) {
	previousScore, previousResult := tutorialScoreDelay, tutorialResultDelay
	tutorialScoreDelay, tutorialResultDelay = 10*time.Millisecond, 10*time.Millisecond
	defer func() {
		tutorialScoreDelay, tutorialResultDelay = previousScore, previousResult
	}()

	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	if err := server.handleTutorialCreate(session, tutorialPacket(405, nil)); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := server.handleTutorialPlay(session,
		tutorialPlayPacket("sin_mission_roll_lv_02", "Forest", "display", 2, 45000)); err != nil {
		t.Fatalf("play: %v", err)
	}
	recorder.reset()
	failed := protocol.NewWriter()
	failed.Unsigned(0)
	if err := server.handleTutorialEndStage(session, tutorialPacket(724, failed.Payload())); err != nil {
		t.Fatalf("end stage: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		if winner, ok := recorder.first(732); ok {
			_, winnerTeam, firstTotal, secondTotal := tutorialScoreboard(t, winner.Payload)
			if winnerTeam != 2 || firstTotal != 0 || secondTotal != 1 {
				t.Fatalf("S732 = winner %d totals %d:%d, want 2 and 0:1", winnerTeam, firstTotal, secondTotal)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no result tick: %v", recorder.ids())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// The prologue ends straight into the landing page: its own envelope and the
// scene transition, with no result ticks in between.
func TestTutorialPrologueEndsIntoTheLandingPage(t *testing.T) {
	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	if err := server.handleTutorialCreate(session, tutorialPacket(405, nil)); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := server.handleTutorialPlay(session,
		tutorialPlayPacket("rel_prologue_01", "Forest", "display", 1, 99999000)); err != nil {
		t.Fatalf("play: %v", err)
	}
	recorder.reset()
	ended := protocol.NewWriter()
	ended.Unsigned(1)
	if err := server.handleTutorialEndStage(session, tutorialPacket(724, ended.Payload())); err != nil {
		t.Fatalf("end stage: %v", err)
	}
	if _, ok := recorder.first(732); ok {
		t.Fatalf("the prologue must not send a result tick: %v", recorder.ids())
	}
	if _, ok := recorder.first(108); ok {
		t.Fatalf("a finished stage must not ask for the scene transition: %v", recorder.ids())
	}
	menu, ok := recorder.first(110)
	if !ok {
		t.Fatalf("the prologue must restate the landing page envelope: %v", recorder.ids())
	}
	setting, err := protocol.ReadSessionSetting(protocol.NewReader(menu.Payload))
	if err != nil {
		t.Fatalf("read the menu envelope: %v", err)
	}
	if setting.Kind != 1 {
		t.Fatalf("menu envelope kind = %d, want 1", setting.Kind)
	}
	if _, ok := server.tutorialCurrent(session); ok {
		t.Fatal("the tutorial state must be gone once the prologue ends")
	}
}

// The stage end outside a tutorial is not read at all: the cooperative modes'
// own stage end must keep whatever they do with it.
func TestTutorialStageEndOutsideATutorialSendsNothing(t *testing.T) {
	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	completed := protocol.NewWriter()
	completed.Unsigned(1)
	if err := server.handleTutorialEndStage(session, tutorialPacket(724, completed.Payload())); err != nil {
		t.Fatalf("a stray stage end must not fail: %v", err)
	}
	if recorder.count() != 0 {
		t.Fatalf("a stray stage end must send nothing, got %v", recorder.ids())
	}
}

// The prologue's NPC traffic is echoed to its own sender, and only inside a
// stage.
func TestTutorialPrologueEchoesItsNpcTraffic(t *testing.T) {
	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	// Outside a stage nothing is answered.
	if server.handleTutorialNpc(session, tutorialPacket(32857, []byte{1, 0, 0, 0})) {
		t.Fatal("NPC traffic outside a stage must not be answered")
	}
	if recorder.count() != 0 {
		t.Fatalf("nothing may be sent outside a stage, got %v", recorder.ids())
	}
	if err := server.handleTutorialCreate(session, tutorialPacket(405, nil)); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := server.handleTutorialPlay(session,
		tutorialPlayPacket("rel_prologue_01", "Forest", "display", 1, 99999000)); err != nil {
		t.Fatalf("play: %v", err)
	}
	recorder.reset()

	wave := protocol.NewWriter()
	wave.Unsigned(1)
	if !server.handleTutorialNpc(session, tutorialPacket(903, wave.Payload())) {
		t.Fatal("the wave start belongs to the stage")
	}
	if echo, ok := recorder.first(904); !ok {
		t.Fatalf("the wave start answers with 904, got %v", recorder.ids())
	} else if len(echo.Payload) != 4 {
		t.Fatalf("904 payload = %x, want the wave word", echo.Payload)
	}
	recorder.reset()

	if !server.handleTutorialNpc(session, tutorialPacket(32857, []byte{1, 0, 0, 0})) {
		t.Fatal("the spawn group belongs to the stage")
	}
	if echo, ok := recorder.first(32857); !ok {
		t.Fatalf("the spawn group is echoed back, got %v", recorder.ids())
	} else if len(echo.Payload) != 4 {
		t.Fatalf("spawn group echo = %x", echo.Payload)
	}
	recorder.reset()

	table := []byte{9, 9, 9, 9}
	if !server.handleTutorialNpc(session, tutorialPacket(908, table)) {
		t.Fatal("the wave table belongs to the stage")
	}
	empty, ok := recorder.first(908)
	if !ok {
		t.Fatalf("the wave table answers with an empty 908, got %v", recorder.ids())
	}
	if len(empty.Payload) != 0 {
		t.Fatalf("908 must be empty, got %x", empty.Payload)
	}
	if echo, ok := recorder.first(909); !ok {
		t.Fatalf("the wave table is echoed as 909, got %v", recorder.ids())
	} else if string(echo.Payload) != string(table) {
		t.Fatalf("909 echo = %x, want %x", echo.Payload, table)
	}
	recorder.reset()

	if !server.handleTutorialNpc(session, tutorialPacket(905, nil)) {
		t.Fatal("the end-all-waves report belongs to the stage")
	}
	if _, ok := recorder.first(705); !ok {
		t.Fatalf("a cleared prologue reports its end, got %v", recorder.ids())
	}
	if _, ok := recorder.first(110); !ok {
		t.Fatalf("a cleared prologue returns to the landing page, got %v", recorder.ids())
	}
}

// Leaving the tutorial sends the landing page back, and a leave outside one is
// left to the room flow.
func TestTutorialLeaveRestatesTheLandingPage(t *testing.T) {
	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	if server.handleTutorialLeave(session, tutorialPacket(404, nil)) {
		t.Fatal("a leave without a tutorial must fall through to the room flow")
	}
	if err := server.handleTutorialCreate(session, tutorialPacket(405, nil)); err != nil {
		t.Fatalf("create: %v", err)
	}
	recorder.reset()
	if !server.handleTutorialLeave(session, tutorialPacket(404, nil)) {
		t.Fatal("a leave inside the tutorial belongs to it")
	}
	if _, ok := recorder.first(110); !ok {
		t.Fatalf("the landing page envelope is missing: %v", recorder.ids())
	}
	if _, ok := recorder.first(108); !ok {
		t.Fatalf("the scene transition is missing: %v", recorder.ids())
	}
	if _, ok := server.tutorialCurrent(session); ok {
		t.Fatal("the tutorial state must be gone after the leave")
	}
}

// A timer armed by an earlier stage must not touch the stage that replaced it.
func TestTutorialTimersBelongToTheirStage(t *testing.T) {
	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	if err := server.handleTutorialCreate(session, tutorialPacket(405, nil)); err != nil {
		t.Fatalf("create: %v", err)
	}
	first := server.tutorialBegin(session, tutorialState{phase: tutorialPlaying})
	if err := server.handleTutorialCreate(session, tutorialPacket(405, nil)); err != nil {
		t.Fatalf("second create: %v", err)
	}
	if server.tutorialAlive(session, first) {
		t.Fatal("the earlier stage's generation must be dead once a new flow starts")
	}
	state, ok := server.tutorialCurrent(session)
	if !ok || state.phase != tutorialSelecting {
		t.Fatalf("the new flow must own the session, got %+v", state)
	}
}

// A stage end is accepted once: the client reports one on clear and one on exit,
// and the second must not start the result chain again.
func TestTutorialStageEndIsAcceptedOnce(t *testing.T) {
	previousScore, previousResult := tutorialScoreDelay, tutorialResultDelay
	tutorialScoreDelay, tutorialResultDelay = 15*time.Millisecond, 15*time.Millisecond
	defer func() {
		tutorialScoreDelay, tutorialResultDelay = previousScore, previousResult
	}()

	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	if err := server.handleTutorialCreate(session, tutorialPacket(405, nil)); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := server.handleTutorialPlay(session,
		tutorialPlayPacket("sin_mission_pang_lv_01", "Forest", "display", 1, 180000)); err != nil {
		t.Fatalf("play: %v", err)
	}
	recorder.reset()
	completed := protocol.NewWriter()
	completed.Unsigned(1)
	for attempt := 0; attempt < 3; attempt++ {
		if err := server.handleTutorialEndStage(session, tutorialPacket(724, completed.Payload())); err != nil {
			t.Fatalf("end stage %d: %v", attempt, err)
		}
	}
	deadline := time.Now().Add(time.Second)
	for {
		if _, ok := recorder.first(110); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the stage never returned to the select scene: %v", recorder.ids())
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	ends, winners, selects := 0, 0, 0
	for _, id := range recorder.ids() {
		switch id {
		case 705:
			ends++
		case 732:
			winners++
		case 110:
			selects++
		}
	}
	if ends != 1 || winners != 1 || selects != 1 {
		t.Fatalf("a repeated end must be ignored: ends=%d winners=%d selects=%d (%v)",
			ends, winners, selects, recorder.ids())
	}
}

// A stage cannot be started without a character, and the request must not cost
// the connection.
func TestTutorialPlayWithoutACharacterIsDropped(t *testing.T) {
	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := &Session{ID: 2, send: recorder}
	if err := server.handleTutorialPlay(session,
		tutorialPlayPacket("sin_mission_roll_lv_01", "Forest", "display", 1, 90000)); err != nil {
		t.Fatalf("a request from a session without a character must not fail the connection: %v", err)
	}
	if recorder.count() != 0 {
		t.Fatalf("nothing may be sent, got %v", recorder.ids())
	}
	if _, ok := server.tutorialCurrent(session); ok {
		t.Fatal("no tutorial state may be opened")
	}
}

// Once a stage is settled its own NPC traffic is no longer answered.
func TestTutorialNpcEchoStopsAfterTheStageSettles(t *testing.T) {
	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	if err := server.handleTutorialCreate(session, tutorialPacket(405, nil)); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := server.handleTutorialPlay(session,
		tutorialPlayPacket("rel_prologue_01", "Forest", "display", 1, 99999000)); err != nil {
		t.Fatalf("play: %v", err)
	}
	state, _ := server.tutorialCurrent(session)
	server.settleTutorial(session, state.generation)
	recorder.reset()
	if server.handleTutorialNpc(session, tutorialPacket(32857, []byte{1, 0, 0, 0})) {
		t.Fatal("a settled stage must not answer its NPC traffic")
	}
	if recorder.count() != 0 {
		t.Fatalf("nothing may be sent once settled, got %v", recorder.ids())
	}
}

// The tutorial's seat belongs to no team, which is what the client's own
// single-player stage carries.
func TestTutorialSeatBelongsToNoTeam(t *testing.T) {
	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	member, err := server.tutorialMember(session)
	if err != nil {
		t.Fatalf("seat: %v", err)
	}
	if member.Team != 0 {
		t.Fatalf("tutorial seat team = %d, want 0", member.Team)
	}
	if member.Character == nil || member.Character.AccountID == 0 {
		t.Fatal("the seat must carry the session's character")
	}
}

// The prologue's mob life points are echoed back; without the echo the local mob
// spends no health and pops at the first hit instead of bubbling.
func TestTutorialPrologueEchoesMobLifePoints(t *testing.T) {
	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	if err := server.handleTutorialCreate(session, tutorialPacket(405, nil)); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := server.handleTutorialPlay(session,
		tutorialPlayPacket("rel_prologue_01", "Forest", "display", 1, 99999000)); err != nil {
		t.Fatalf("play: %v", err)
	}
	recorder.reset()
	body := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	if !server.handleTutorialNpc(session, tutorialPacket(32920, body)) {
		t.Fatal("the mob life points belong to the stage")
	}
	echo, ok := recorder.first(32920)
	if !ok {
		t.Fatalf("the life points are echoed back, got %v", recorder.ids())
	}
	if string(echo.Payload) != string(body) {
		t.Fatalf("echo = %x, want %x", echo.Payload, body)
	}
}

// An in-stage report must not be read as the lobby packet of the same number: the
// move-into handler would pull the player out of the stage, and a character
// creation would drop the connection.
func TestTutorialStageSwallowsTheInMatchReports(t *testing.T) {
	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	if err := server.handleTutorialCreate(session, tutorialPacket(405, nil)); err != nil {
		t.Fatalf("create: %v", err)
	}
	// The select scene is not a stage: the ordinary handlers still own these.
	if server.tutorialInStage(session) {
		t.Fatal("the select scene is not a stage")
	}
	if err := server.handleTutorialPlay(session,
		tutorialPlayPacket("sin_mission_help_lv_01", "Forest", "display", 1, 180000)); err != nil {
		t.Fatalf("play: %v", err)
	}
	if !server.tutorialInStage(session) {
		t.Fatal("a playing stage must report itself as in-stage")
	}
	recorder.reset()
	for _, id := range []uint16{14, 16} {
		// The health-reset body: two words, which the lobby readers reject.
		report := protocol.NewWriter()
		report.Unsigned(1000000)
		report.Unsigned(1234)
		if err := server.handleTutorialInStageReport(session, tutorialPacket(id, report.Payload())); err != nil {
			t.Fatalf("id %d must be swallowed without an error: %v", id, err)
		}
	}
	if recorder.count() != 0 {
		t.Fatalf("an in-stage report must send nothing, got %v", recorder.ids())
	}
}

// A stage counts its own goals and answers each one with the updated tally: the
// client's HUD reads the counters back, and nothing is written to the database.
func TestTutorialCountsItsGoals(t *testing.T) {
	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	if err := server.handleTutorialCreate(session, tutorialPacket(405, nil)); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := server.handleTutorialPlay(session,
		tutorialPlayPacket("sin_mission_bubble_rifle_lv_01", "Forest", "display", 1, 180000)); err != nil {
		t.Fatalf("play: %v", err)
	}
	recorder.reset()

	bubble := protocol.NewWriter()
	bubble.Byte(1)
	bubble.Unsigned(0)
	bubble.Unsigned(0)
	bubble.Byte(0)
	bubble.Byte(0)
	bubble.Byte(0)
	bubble.Unsigned(0)
	if !server.handleTutorialScore(session, tutorialPacket(710, bubble.Payload())) {
		t.Fatal("a bubble belongs to the stage")
	}
	rescue := protocol.NewWriter()
	rescue.Byte(4)
	rescue.Unsigned(0)
	rescue.Unsigned(0)
	rescue.Byte(0)
	rescue.Byte(0)
	rescue.Byte(0)
	rescue.Unsigned(0)
	if !server.handleTutorialScore(session, tutorialPacket(710, rescue.Payload())) {
		t.Fatal("a rescue belongs to the stage")
	}
	pang := protocol.NewWriter()
	pang.Byte(2)
	pang.Unsigned(0)
	pang.Byte(0)
	if !server.handleTutorialScore(session, tutorialPacket(2023, pang.Payload())) {
		t.Fatal("a pang belongs to the stage")
	}
	// A kind this stage does not count is swallowed without an update.
	other := protocol.NewWriter()
	other.Byte(3)
	other.Unsigned(0)
	other.Unsigned(0)
	other.Byte(0)
	other.Byte(0)
	other.Byte(0)
	other.Unsigned(0)
	before := recorder.count()
	if !server.handleTutorialScore(session, tutorialPacket(710, other.Payload())) {
		t.Fatal("an uncounted kind still belongs to the stage")
	}
	if recorder.count() != before {
		t.Fatalf("an uncounted kind must not answer, got %v", recorder.ids())
	}
	state, ok := server.tutorialCurrent(session)
	if !ok {
		t.Fatal("the stage must still be running")
	}
	if state.bubbles != 1 || state.helps != 1 || state.pangs != 1 {
		t.Fatalf("counters = %d/%d/%d, want 1/1/1", state.bubbles, state.pangs, state.helps)
	}
	update, ok := recorder.first(729)
	if !ok {
		t.Fatalf("every counted goal answers with the tally, got %v", recorder.ids())
	}
	if len(update.Payload) == 0 {
		t.Fatal("the tally must carry the score record")
	}
}

// The goal reports of a session outside a tutorial are left to the ordinary
// dispatch.
func TestTutorialScoreOutsideAStageIsIgnored(t *testing.T) {
	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	bubble := protocol.NewWriter()
	bubble.Byte(1)
	bubble.Unsigned(0)
	bubble.Unsigned(0)
	bubble.Byte(0)
	bubble.Byte(0)
	bubble.Byte(0)
	bubble.Unsigned(0)
	if server.handleTutorialScore(session, tutorialPacket(710, bubble.Payload())) {
		t.Fatal("a goal outside a stage must not be handled here")
	}
	if recorder.count() != 0 {
		t.Fatalf("nothing may be sent, got %v", recorder.ids())
	}
}

// A room owns the session once one is open: the room's leave request must go to
// the room flow, and the leftover tutorial state must not answer it.
func TestTutorialLeaveYieldsToARoom(t *testing.T) {
	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	if err := server.handleTutorialCreate(session, tutorialPacket(405, nil)); err != nil {
		t.Fatalf("create: %v", err)
	}
	recorder.reset()
	session.room = &Room{}
	if server.handleTutorialLeave(session, tutorialPacket(404, nil)) {
		t.Fatal("a session that sits in a room leaves through the room flow")
	}
	if recorder.count() != 0 {
		t.Fatalf("the landing page must not be restated, got %v", recorder.ids())
	}
	if _, ok := server.tutorialCurrent(session); ok {
		t.Fatal("the leftover tutorial state must be dropped")
	}
}

// The dispatch is what has to reach the tutorial: 32920 (a mob's life points) and
// 710 (a goal) are otherwise claimed by the lobby or by nobody, so a helper-level
// test cannot prove the stage sees them.
func TestTutorialDispatchClaimsItsOwnNumbers(t *testing.T) {
	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	if err := server.Handle(session, tutorialPacket(405, nil)); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := server.Handle(session, tutorialPlayPacket(
		"rel_prologue_01", "Forest", "display", 1, 99999000)); err != nil {
		t.Fatalf("play: %v", err)
	}
	recorder.reset()

	life := []byte{0x11, 0x22, 0x33, 0x44}
	if err := server.Handle(session, tutorialPacket(32920, life)); err != nil {
		t.Fatalf("mob life points: %v", err)
	}
	if echo, ok := recorder.first(32920); !ok {
		t.Fatalf("the dispatch never reached the stage's life-point echo: %v", recorder.ids())
	} else if string(echo.Payload) != string(life) {
		t.Fatalf("life point echo = %x, want %x", echo.Payload, life)
	}
	recorder.reset()

	bubble := protocol.NewWriter()
	bubble.Byte(1)
	bubble.Unsigned(0)
	bubble.Unsigned(0)
	bubble.Byte(0)
	bubble.Byte(0)
	bubble.Byte(0)
	bubble.Unsigned(0)
	if err := server.Handle(session, tutorialPacket(710, bubble.Payload())); err != nil {
		t.Fatalf("goal report: %v", err)
	}
	if _, ok := recorder.first(729); !ok {
		t.Fatalf("the dispatch never reached the stage's goal counter: %v", recorder.ids())
	}
}

// Outside a tutorial the same numbers keep their ordinary handlers: 32920 goes to
// the life-point report, and a session with no room has nothing to do with it.
func TestTutorialDispatchLeavesTheOrdinaryNumbersAlone(t *testing.T) {
	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	if err := server.Handle(session, tutorialPacket(32920, []byte{1, 2, 3, 4})); err != nil {
		t.Fatalf("a life point outside a round must be dropped quietly: %v", err)
	}
	if recorder.count() != 0 {
		t.Fatalf("nothing may be sent, got %v", recorder.ids())
	}
}

// The tutorial's own numbers must stay clear of the multiplayer helper sets: if
// one of them were claimed there too, a tutorial stage's report would be relayed
// or scored as a room event.
func TestTutorialNumbersCollideWithNoRoomPath(t *testing.T) {
	ids := map[string]uint16{
		"start wave":   903,
		"end wave":     904,
		"all waves":    905,
		"wave table":   908,
		"goal report":  710,
		"compact goal": 2023,
		"spawn group":  32857,
		"member mob":   32862,
		"ai object":    32892,
		"life point":   32920,
		"create":       405,
		"stage list":   920,
		"play":         107,
		"end stage":    724,
	}
	for name, id := range ids {
		if protocol.IsPeerLife(id) || protocol.IsPeerKick(id) || protocol.IsPeerAction(id) || isItemRelayPacket(id) {
			t.Fatalf("%s (%d) is claimed by a room helper as well", name, id)
		}
	}
}

// The counters are native ushorts: a stage that reports past the ceiling keeps the
// last value instead of wrapping to zero.
func TestTutorialGoalCountersStopAtTheNativeCeiling(t *testing.T) {
	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	if err := server.handleTutorialCreate(session, tutorialPacket(405, nil)); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := server.handleTutorialPlay(session,
		tutorialPlayPacket("sin_mission_pang_lv_01", "Forest", "display", 1, 180000)); err != nil {
		t.Fatalf("play: %v", err)
	}
	session.tutorialMu.Lock()
	session.tutorial.bubbles = 0xFFFF
	session.tutorialMu.Unlock()
	if _, ok := server.tutorialCount(session, protocol.TutorialGoalBubble); !ok {
		t.Fatal("the stage must accept the report")
	}
	state, _ := server.tutorialCurrent(session)
	if state.bubbles != 0xFFFF {
		t.Fatalf("bubbles = %d, want the ceiling kept", state.bubbles)
	}
}

// A stage cannot open on a session that sits in a room: the two would fight over
// the load report and the score reports.
func TestTutorialPlayRefusesASessionInARoom(t *testing.T) {
	server := tutorialTestServer()
	recorder := &tutorialRecorder{}
	session := tutorialTestSession(recorder)
	if err := server.handleTutorialCreate(session, tutorialPacket(405, nil)); err != nil {
		t.Fatalf("create: %v", err)
	}
	recorder.reset()
	session.room = &Room{}
	if err := server.handleTutorialPlay(session,
		tutorialPlayPacket("sin_mission_roll_lv_01", "Forest", "display", 1, 90000)); err != nil {
		t.Fatalf("the request must be dropped without an error: %v", err)
	}
	if recorder.count() != 0 {
		t.Fatalf("nothing may be sent, got %v", recorder.ids())
	}
	state, ok := server.tutorialCurrent(session)
	if !ok || state.phase != tutorialSelecting {
		t.Fatalf("the session must stay in the select scene, got %+v", state)
	}
}
