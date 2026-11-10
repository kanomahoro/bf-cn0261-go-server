package game

import (
	"testing"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/world"
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
func TestBombSeriesNeverSwapsSides(t *testing.T) {
	_, room := bombTestRoom()
	room.mu.Lock()
	defer room.mu.Unlock()

	for round := 0; round <= 2; round++ {
		room.bomb.roundIndex = round
		if got := room.bomb.attackingTeam(); got != 1 {
			t.Fatalf("sub-round %d attacks with team %d; this client never swaps sides", round, got)
		}
		if got := room.bomb.defendingTeam(); got != 2 {
			t.Fatalf("sub-round %d defends with team %d, want 2", round, got)
		}
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

// TestBombRandomRoomPicksFromItsOwnPool checks the "Random" entry the bomb widget
// offers: a random bomb room must only ever roll one of its own six maps, never a
// competitive map that happens to be in the catalog.
func TestBombRandomRoomPicksFromItsOwnPool(t *testing.T) {
	maps := world.Maps{Maps: []world.Map{
		{ResourceName: "rel_Apex01_Diorama", Theme: "Military", NormalMode: true, ItemMode: true, TeamSpawnCount: 8},
		{ResourceName: "rel_Village01_Byway", Theme: "Village", NormalMode: true, ItemMode: true, TeamSpawnCount: 8},
	}, DefaultRulesMap: "rel_Village01_Byway"}
	setting := protocol.SessionSetting{Game: protocol.GameSetting{
		Mode: bombModeID,
		Strings16: [4][]uint16{
			protocol.U16String("Random"), {}, protocol.U16String("All"), protocol.U16String("Random"),
		},
	}}
	pool := map[string]bool{}
	for _, entry := range maps.BombMaps() {
		pool[entry.ResourceName] = true
	}
	seen := map[string]bool{}
	for attempt := 0; attempt < 200; attempt++ {
		got := roundMap(maps, setting)
		if !pool[got] {
			t.Fatalf("a random bomb room picked %q, which is not in its pool", got)
		}
		seen[got] = true
	}
	if len(seen) < 2 {
		t.Fatalf("a random bomb room never varied its pick in 200 tries: %v", seen)
	}
}

// TestBombRandomRoomHonoursTheTheme checks that the bomb widget's theme filter
// narrows its own pool.
func TestBombRandomRoomHonoursTheTheme(t *testing.T) {
	maps := world.Maps{}
	setting := protocol.SessionSetting{Game: protocol.GameSetting{
		Mode: bombModeID,
		Strings16: [4][]uint16{
			protocol.U16String("Random"), {}, protocol.U16String("Sudden"), protocol.U16String("Random"),
		},
	}}
	for attempt := 0; attempt < 20; attempt++ {
		if got := roundMap(maps, setting); got != "rel_Sudden06_Tank" {
			t.Fatalf("the sudden theme rolled %q", got)
		}
	}
}

// TestBombPinnedMapIsKept checks the non-random entry: the map the room pinned
// comes back unchanged, including the two maps only the bomb pool carries.
func TestBombPinnedMapIsKept(t *testing.T) {
	maps := world.Maps{}
	for _, name := range []string{"rel_Sudden06_Tank", "rel_zombie_01_bomb", "rel_Village03_CityHall"} {
		setting := protocol.SessionSetting{Game: protocol.GameSetting{
			Mode: bombModeID,
			Strings16: [4][]uint16{
				protocol.U16String(name), protocol.U16String(name), protocol.U16String("All"), protocol.U16String(name),
			},
		}}
		if got := roundMap(maps, setting); got != name {
			t.Fatalf("pinned %q came back as %q", name, got)
		}
	}
}

// TestBombTimeLimitAwardsAnExplodedBombToTheAttacker checks the clock running out
// between the client's explosion and its report: the bomb already went off, so the
// sub-round belongs to the attacking team, not to the defenders it beat.
func TestBombTimeLimitAwardsAnExplodedBombToTheAttacker(t *testing.T) {
	server, room := bombTestRoom()
	defer func() {
		room.mu.Lock()
		if room.endTimer != nil {
			room.endTimer.Stop()
			room.endTimer = nil
		}
		room.mu.Unlock()
	}()

	room.mu.Lock()
	room.bomb.state = protocol.BombStateExploded
	room.mu.Unlock()

	server.roundClockExpired(room)

	room.mu.RLock()
	wins, phase := room.bomb.wins, room.phase
	room.mu.RUnlock()
	if wins != [2]int{1, 0} {
		t.Fatalf("an exploded bomb at the clock did not go to the attacker: %v", wins)
	}
	if phase != phaseSettling {
		t.Fatalf("the sub-round did not end: phase %d", phase)
	}
}

// TestBombTimeLimitAwardsAnUnplantedRoundToTheDefender checks the ordinary case:
// the clock runs out on a bomb that never went off.
func TestBombTimeLimitAwardsAnUnplantedRoundToTheDefender(t *testing.T) {
	server, room := bombTestRoom()
	defer func() {
		room.mu.Lock()
		if room.endTimer != nil {
			room.endTimer.Stop()
			room.endTimer = nil
		}
		room.mu.Unlock()
	}()

	server.roundClockExpired(room)

	room.mu.RLock()
	wins := room.bomb.wins
	room.mu.RUnlock()
	if wins != [2]int{0, 1} {
		t.Fatalf("an unplanted round at the clock did not go to the defender: %v", wins)
	}
}

// TestBombReviveBudgetIsPerSubRound pins the rule master confirmed on 2026-11-10:
// each seat may respawn bombRevivesPerSubRound times inside a sub-round, and the
// next sub-round hands the budget back.
func TestBombReviveBudgetIsPerSubRound(t *testing.T) {
	server, room := bombTestRoom()
	room.mu.Lock()
	room.phase = phasePlaying
	room.mu.Unlock()

	for i := 1; i <= bombRevivesPerSubRound; i++ {
		if !server.bombReviveAllowed(room, 1) {
			t.Fatalf("revive %d of %d was refused", i, bombRevivesPerSubRound)
		}
	}
	if server.bombReviveAllowed(room, 1) {
		t.Fatalf("a seat over its %d-revive budget still respawned", bombRevivesPerSubRound)
	}
	// The seat's own counter is what the next sub-round clears.
	room.mu.Lock()
	room.bombRevives = [maxRoomSlots]uint8{}
	room.mu.Unlock()
	if !server.bombReviveAllowed(room, 1) {
		t.Fatal("the next sub-round did not hand the revive budget back")
	}
}

// TestBombReviveIsRefusedOutsideTheRound checks the budget only applies while a
// sub-round is actually being played.
func TestBombReviveIsRefusedOutsideTheRound(t *testing.T) {
	server, room := bombTestRoom()
	room.mu.Lock()
	room.phase = phaseLoading
	room.mu.Unlock()
	if server.bombReviveAllowed(room, 1) {
		t.Fatal("a seat respawned outside a playing sub-round")
	}
}

// TestBombSeriesKeepsItsSeatWhenTheRelayDrops checks the grace decision the series
// transition depends on: the series is active, so a relay teardown is a reload.
func TestBombSeriesKeepsItsSeatWhenTheRelayDrops(t *testing.T) {
	_, room := bombTestRoom()
	if !room.BombSeriesActive() {
		t.Fatal("a fresh sub-round does not report an active series")
	}
	room.mu.Lock()
	room.bomb.decided = true
	room.mu.Unlock()
	if room.BombSeriesActive() {
		t.Fatal("a decided sub-round still reports an active series")
	}
}

// TestBombSubRoundsSendTheOrdinaryRoundEnd checks the round-end notice: the bomb
// mission broadcasts the same packet an ordinary round sends instead of per-seat
// outcome words (this client reads 2 as "time over", which one side printed for a
// sub-round that simply ended). No scoreboard is published between sub-rounds
// either, because any S729 stops the bomb banner from showing.
func TestBombSubRoundsSendTheOrdinaryRoundEnd(t *testing.T) {
	server, room := bombTestRoom()
	room.mu.Lock()
	room.phase = phasePlaying
	var recorders []*captainRecorder
	for slot := range room.slots {
		if room.slots[slot].session != nil {
			recorders = append(recorders, room.slots[slot].session.send.(*captainRecorder))
		}
	}
	room.mu.Unlock()

	server.endBombSubRound(room, 1)

	for index, recorder := range recorders {
		sawEnd := false
		var tally *protocol.Packet
		for i := range recorder.sent {
			if recorder.sent[i].ID == 732 {
				tally = &recorder.sent[i]
			}
		}
		for _, packet := range recorder.sent {
			switch packet.ID {
			case 705:
				sawEnd = true
				if len(packet.Payload) != 8 {
					t.Fatalf("member %d read a %d-byte round end", index, len(packet.Payload))
				}
				// word 2 is the time-over flag in this client; a sub-round that ends
				// on its objective must not claim it.
				word := uint32(packet.Payload[4]) | uint32(packet.Payload[5])<<8 |
					uint32(packet.Payload[6])<<16 | uint32(packet.Payload[7])<<24
				if word != 1 {
					t.Fatalf("member %d read round-end word %d, want 1", index, word)
				}
			case 729:
				t.Fatalf("member %d received a scoreboard (id %d) for a sub-round", index, packet.ID)
			}
		}
		if !sawEnd {
			t.Fatalf("member %d saw no round end", index)
		}
		if tally != nil {
			t.Fatalf("member %d received a tally between sub-rounds: %v", index, tally.Payload)
		}
	}
}

// TestBombDecidedSeriesSurvivesADeparture checks a seat vanishing during the
// deciding banner: the series is already settled in the record, so the round must
// keep its settling phase instead of being cancelled and dropping everyone back
// to the waiting room.
func TestBombDecidedSeriesSurvivesADeparture(t *testing.T) {
	server, room := bombTestRoom()
	room.mu.Lock()
	room.phase = phaseSettling
	room.bomb.winner = 1
	room.mu.Unlock()

	server.roundAfterDeparture(room)

	room.mu.RLock()
	phase := room.phase
	room.mu.RUnlock()
	if phase != phaseSettling {
		t.Fatalf("a departure cancelled the decided series: phase %d", phase)
	}
	if !room.BombSeriesDecided() {
		t.Fatal("the series record lost its winner")
	}
}
