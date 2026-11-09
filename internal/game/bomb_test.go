package game

import (
	"testing"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
)

// bombTestRoom builds a bomb room with one seat per team, a bound life tracker
// and the series a start would have opened.
func bombTestRoom() (*Server, *Room) {
	server := captainTestServer()
	room, _ := captainTestRoomWithRecorders([maxRoomSlots]uint8{1, 2})
	room.Setting.Game.Mode = bombModeID
	room.beginBombSeries()
	return server, room
}

// bombReport is the report a seat's client sends for one transition.
func bombReport(operator, state uint32) protocol.BombState {
	return protocol.BombState{Operator: operator, State: state, X: 10, Y: 20, Z: 30}
}

// TestBombPlantBelongsToTheAttacker checks that only the attacking team plants,
// and that the plant position is the only one the bomb ever takes.
func TestBombPlantBelongsToTheAttacker(t *testing.T) {
	server, room := bombTestRoom()
	room.mu.Lock()
	defer room.mu.Unlock()

	if reason := server.bombTransition(room, 1, bombReport(700, protocol.BombStatePlanting)); reason != "team" {
		t.Fatalf("a defender's plant was not refused for its team: %q", reason)
	}
	if reason := server.bombTransition(room, 0, bombReport(700, protocol.BombStatePlanting)); reason != "" {
		t.Fatalf("the attacker's plant was refused: %q", reason)
	}
	if room.bomb.planter != 0 || room.bomb.planterRef != 700 {
		t.Fatalf("planter not recorded: seat %d ref %d", room.bomb.planter, room.bomb.planterRef)
	}
	if room.bomb.position != [3]float32{10, 20, 30} {
		t.Fatalf("plant position not stored: %v", room.bomb.position)
	}
	// A second plant belongs to no transition the client can make.
	if reason := server.bombTransition(room, 0, bombReport(700, protocol.BombStatePlanting)); reason != "state" {
		t.Fatalf("a repeated plant was accepted: %q", reason)
	}
}

// TestBombPositionIsPinned checks that every report after the plant has to repeat
// the position the plant carried.
func TestBombPositionIsPinned(t *testing.T) {
	server, room := bombTestRoom()
	room.mu.Lock()
	defer room.mu.Unlock()

	if reason := server.bombTransition(room, 0, bombReport(700, protocol.BombStatePlanting)); reason != "" {
		t.Fatalf("plant refused: %q", reason)
	}
	moved := protocol.BombState{Operator: 700, State: protocol.BombStateCountdown, X: 11, Y: 20, Z: 30}
	if reason := server.bombTransition(room, 0, moved); reason != "position" {
		t.Fatalf("a moved bomb was accepted: %q", reason)
	}
	armed := protocol.BombState{Operator: 700, State: protocol.BombStateCountdown, X: 10, Y: 20, Z: 30}
	if reason := server.bombTransition(room, 0, armed); reason != "" {
		t.Fatalf("arming the planted bomb was refused: %q", reason)
	}
	if !room.bomb.armed() {
		t.Fatal("the bomb is not armed after its countdown state")
	}
}

// TestBombOperatorIsRemembered checks that a seat cannot report a transition under
// another player's operator reference.
func TestBombOperatorIsRemembered(t *testing.T) {
	server, room := bombTestRoom()
	room.mu.Lock()
	defer room.mu.Unlock()

	if reason := server.bombTransition(room, 0, bombReport(700, protocol.BombStatePlanting)); reason != "" {
		t.Fatalf("plant refused: %q", reason)
	}
	if reason := server.bombTransition(room, 0, bombReport(701, protocol.BombStateCountdown)); reason != "operator" {
		t.Fatalf("a changed operator reference was accepted: %q", reason)
	}
}

// TestBombDefuseChain checks the defender's defuse: only the defender starts it,
// the interrupted defuse returns to the countdown, and a completed one clears the
// planter.
func TestBombDefuseChain(t *testing.T) {
	server, room := bombTestRoom()
	room.mu.Lock()
	defer room.mu.Unlock()

	if reason := server.bombTransition(room, 0, bombReport(700, protocol.BombStatePlanting)); reason != "" {
		t.Fatalf("plant refused: %q", reason)
	}
	if reason := server.bombTransition(room, 0, bombReport(700, protocol.BombStateCountdown)); reason != "" {
		t.Fatalf("arming refused: %q", reason)
	}
	// The planter cannot defuse its own bomb: it is not on the defending team.
	if reason := server.bombTransition(room, 0, bombReport(700, protocol.BombStateDefusing)); reason != "team" {
		t.Fatalf("the attacker defused its own bomb: %q", reason)
	}
	if reason := server.bombTransition(room, 1, bombReport(900, protocol.BombStateDefusing)); reason != "" {
		t.Fatalf("the defender's defuse was refused: %q", reason)
	}
	if room.bomb.defuser != 1 {
		t.Fatalf("defuser not recorded: %d", room.bomb.defuser)
	}
	if reason := server.bombTransition(room, 1, bombReport(900, protocol.BombStateDefuseInterrupted)); reason != "" {
		t.Fatalf("the interrupted defuse was refused: %q", reason)
	}
	if room.bomb.state != protocol.BombStateCountdown || room.bomb.defuser != -1 {
		t.Fatalf("an interrupted defuse did not return to the countdown: state %d defuser %d",
			room.bomb.state, room.bomb.defuser)
	}
	if reason := server.bombTransition(room, 1, bombReport(900, protocol.BombStateDefusing)); reason != "" {
		t.Fatalf("the second defuse was refused: %q", reason)
	}
	if reason := server.bombTransition(room, 1, bombReport(900, protocol.BombStateDefused)); reason != "" {
		t.Fatalf("the completed defuse was refused: %q", reason)
	}
	if room.bomb.planter != -1 {
		t.Fatalf("a completed defuse kept the planter: %d", room.bomb.planter)
	}
}

// TestBombDetonationBelongsToTheMaster checks that only the room master may report
// the detonation, and only for the bomb it actually planted.
func TestBombDetonationBelongsToTheMaster(t *testing.T) {
	server, room := bombTestRoom()
	room.Master = 0
	room.mu.Lock()
	defer room.mu.Unlock()

	if reason := server.bombTransition(room, 0, bombReport(700, protocol.BombStatePlanting)); reason != "" {
		t.Fatalf("plant refused: %q", reason)
	}
	if reason := server.bombTransition(room, 0, bombReport(700, protocol.BombStateCountdown)); reason != "" {
		t.Fatalf("arming refused: %q", reason)
	}
	if reason := server.bombTransition(room, 1, bombReport(900, protocol.BombStateExploded)); reason != "master" {
		t.Fatalf("a seat that is not the master reported the detonation: %q", reason)
	}
	wrongPlanter := protocol.BombState{Operator: 701, State: protocol.BombStateExploded, X: 10, Y: 20, Z: 30}
	if reason := server.bombTransition(room, 0, wrongPlanter); reason != "operator" {
		t.Fatalf("a detonation under another operator was accepted: %q", reason)
	}
	if reason := server.bombTransition(room, 0, protocol.BombState{Operator: 700, State: protocol.BombStateExploded, X: 10, Y: 20, Z: 30}); reason != "" {
		t.Fatalf("the master's detonation was refused: %q", reason)
	}
}

// TestBombMissionOverMatching checks the three shapes of the master's report: an
// explosion, a defuse and a wiped team before the plant.
func TestBombMissionOverMatching(t *testing.T) {
	server, room := bombTestRoom()
	room.mu.Lock()
	defer room.mu.Unlock()

	if reason := server.bombTransition(room, 0, bombReport(700, protocol.BombStatePlanting)); reason != "" {
		t.Fatalf("plant refused: %q", reason)
	}
	if reason := server.bombTransition(room, 0, bombReport(700, protocol.BombStateCountdown)); reason != "" {
		t.Fatalf("arming refused: %q", reason)
	}
	if reason := server.bombTransition(room, 0, bombReport(700, protocol.BombStateExploded)); reason != "" {
		t.Fatalf("detonation refused: %q", reason)
	}
	// The attacking team wins an explosion, and the report has to name the planter.
	if !server.bombMatchesResult(room, room.bomb, protocol.BombMissionOver{
		WinningTeam: 1, Planter: 700, Defuser: protocol.BombOperatorNone,
	}) {
		t.Fatal("an explosion report was not matched")
	}
	if server.bombMatchesResult(room, room.bomb, protocol.BombMissionOver{
		WinningTeam: 2, Planter: 700, Defuser: protocol.BombOperatorNone,
	}) {
		t.Fatal("an explosion was awarded to the defending team")
	}

	// A wiped team before the plant ends the sub-round for the other side.
	freshServer, fresh := bombTestRoom()
	fresh.mu.Lock()
	defer fresh.mu.Unlock()
	fresh.life.Drop(1)
	if !freshServer.bombMatchesResult(fresh, fresh.bomb, protocol.BombMissionOver{
		WinningTeam: 1, Planter: protocol.BombOperatorNone, Defuser: protocol.BombOperatorNone,
	}) {
		t.Fatal("a wiped defending team before the plant was not matched")
	}
	if freshServer.bombMatchesResult(fresh, fresh.bomb, protocol.BombMissionOver{
		WinningTeam: 2, Planter: protocol.BombOperatorNone, Defuser: protocol.BombOperatorNone,
	}) {
		t.Fatal("a wiped team still won its own report")
	}
}

// TestBombSeriesAlternatesSides checks the series counters and the side swap: two
// sub-round wins take the match, and every sub-round swaps the attacking team.
func TestBombSeriesAlternatesSides(t *testing.T) {
	_, room := bombTestRoom()
	room.mu.Lock()
	defer room.mu.Unlock()

	if got := room.bomb.attackingTeam(); got != 1 {
		t.Fatalf("sub-round zero does not start with team 1 attacking: %d", got)
	}
	room.bomb.roundIndex = 1
	if got := room.bomb.attackingTeam(); got != 2 {
		t.Fatalf("sub-round one does not swap the sides: %d", got)
	}
	room.bomb.roundIndex = 2
	if got := room.bomb.attackingTeam(); got != 1 {
		t.Fatalf("sub-round two does not swap back: %d", got)
	}
}

// TestBombSeriesSettingWords checks the two game setting words the client reads a
// series entry from.
func TestBombSeriesSettingWords(t *testing.T) {
	_, room := bombTestRoom()
	room.mu.Lock()
	defer room.mu.Unlock()

	if room.Setting.Game.Values148[6]&1 == 0 {
		t.Fatal("the first sub-round does not set the first-round flag")
	}
	if room.Setting.Game.Values148[7] != 0 {
		t.Fatalf("sub-round zero does not report index 0: %d", room.Setting.Game.Values148[7])
	}
	room.bomb.roundIndex = 1
	room.applyBombSeriesSettingLocked()
	if room.Setting.Game.Values148[6]&1 != 0 {
		t.Fatal("a later sub-round still carries the first-round flag")
	}
	if room.Setting.Game.Values148[7] != 1 {
		t.Fatalf("the sub-round index was not written: %d", room.Setting.Game.Values148[7])
	}
}

// TestBombObserverReachesEveryMember checks that the observer answer goes to the
// whole room and names the downed seat's scene object.
func TestBombObserverReachesEveryMember(t *testing.T) {
	server, room := bombTestRoom()
	room.mu.Lock()
	recorders := []*captainRecorder{}
	for slot := range room.slots {
		if room.slots[slot].session != nil {
			recorders = append(recorders, room.slots[slot].session.send.(*captainRecorder))
		}
	}
	room.mu.Unlock()

	server.bombObserve(room, 1)
	want := objectFor(1)
	for index, recorder := range recorders {
		found := false
		for _, packet := range recorder.sent {
			if packet.ID != 974 || len(packet.Payload) != 4 {
				continue
			}
			if actor := uint32(packet.Payload[0]) | uint32(packet.Payload[1])<<8 |
				uint32(packet.Payload[2])<<16 | uint32(packet.Payload[3])<<24; actor == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("member %d was not told about the observer %d", index, want)
		}
	}
}

// TestBombSeriesCountsWinsAndEndsTheMatch checks the two-win rule and that the
// match winner, not the last sub-round's scores, decides the settlement.
func TestBombSeriesCountsWinsAndEndsTheMatch(t *testing.T) {
	server, room := bombTestRoom()
	// The pause timers are not part of the assertions; stop them before they run.
	defer func() {
		room.mu.Lock()
		if room.endTimer != nil {
			room.endTimer.Stop()
			room.endTimer = nil
		}
		room.mu.Unlock()
	}()

	server.endBombSubRound(room, 1)
	room.mu.RLock()
	wins, decided, winner, phase := room.bomb.wins, room.bomb.decided, room.seriesWinner, room.phase
	room.mu.RUnlock()
	if wins != [2]int{1, 0} || !decided || winner != 0 || phase != phaseSettling {
		t.Fatalf("first sub-round recorded wrong: wins %v decided %v winner %d phase %d",
			wins, decided, winner, phase)
	}

	// A repeated report for the sub-round that just ended must not award a second win.
	server.endBombSubRound(room, 1)
	room.mu.RLock()
	wins = room.bomb.wins
	room.mu.RUnlock()
	if wins != [2]int{1, 0} {
		t.Fatalf("a repeated decision counted twice: %v", wins)
	}

	// The next sub-round is a fresh decision.
	room.mu.Lock()
	room.bomb.resetBomb()
	room.bomb.roundIndex = 1
	room.phase = phasePlaying
	room.mu.Unlock()

	server.endBombSubRound(room, 1)
	room.mu.RLock()
	wins, winner, seriesWinner := room.bomb.wins, room.bomb.winner, room.seriesWinner
	room.mu.RUnlock()
	if wins != [2]int{2, 0} || winner != 1 || seriesWinner != 1 {
		t.Fatalf("the match did not end after two wins: wins %v winner %d series %d",
			wins, winner, seriesWinner)
	}
}

// TestBombObserverIsAnnouncedOnce checks that a client asking for a respawn twice
// does not re-announce the same observer to the room.
func TestBombObserverIsAnnouncedOnce(t *testing.T) {
	server, room := bombTestRoom()
	room.mu.Lock()
	recorders := []*captainRecorder{}
	for slot := range room.slots {
		if room.slots[slot].session != nil {
			recorders = append(recorders, room.slots[slot].session.send.(*captainRecorder))
		}
	}
	room.mu.Unlock()

	server.bombObserve(room, 1)
	server.bombObserve(room, 1)
	for index, recorder := range recorders {
		count := 0
		for _, packet := range recorder.sent {
			if packet.ID == 974 {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("member %d saw %d observer announcements, want 1", index, count)
		}
	}
}
