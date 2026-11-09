package game

import (
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
)

// bombModeID is the client's bomb mission (wgtBomb, round mode 19). The client
// owns the plant, the arming countdown and the defuse; the server owns which of
// those reports is allowed, the position they are pinned to, the sub-round record
// and the series.
const bombModeID = 19

// bombRoundsToWin is the legacy series' length: best of three, so two sub-round
// wins take the match (cn-717's rounds_to_win).
const bombRoundsToWin = 2

// bombRoundPause is how long a decided sub-round keeps its result banner before
// the next stage load starts. The reference waits out its result delay before
// reloading the series.
const bombRoundPause = 5 * time.Second

// bombSeries is the mode's whole state: the sub-round being played, the sides it
// is played between, the bomb itself and the wins counted so far.
type bombSeries struct {
	// roundIndex is the sub-round being played, zero based (kr-812 sub_4FB600
	// consumes 0..2). The client reads it out of the entry packet.
	roundIndex int
	// wins counts the decided sub-rounds per team; index 0 is team 1.
	wins [2]int
	// winner is the team that took the match, 0 until a side reaches
	// bombRoundsToWin.
	winner uint8
	// decided marks the current sub-round as counted, so a repeated mission-over
	// report cannot award a second win.
	decided bool

	// state is the bomb's own state word, 0..6 (see protocol.BombState*).
	state uint32
	// planter and defuser are the seats operating the bomb, -1 when nobody is.
	planter, defuser int
	// planterRef and defuserRef are the operator references those seats' clients
	// sent: the mission-over report is matched against them.
	planterRef, defuserRef uint32
	// position is the plant position. Only the plant writes it, and the client
	// repeats it in every later report.
	position [3]float32
}

// attackingTeam returns the team attacking the current sub-round: the sides swap
// every sub-round from round zero (attacking = round % 2 + 1).
func (b *bombSeries) attackingTeam() uint8 {
	if b.roundIndex%2 == 0 {
		return 1
	}
	return 2
}

// defendingTeam returns the other side.
func (b *bombSeries) defendingTeam() uint8 { return 3 - b.attackingTeam() }

// armed reports whether the bomb is counting down, defusing included: from here
// the round's own clock no longer ends the sub-round.
func (b *bombSeries) armed() bool {
	return b.state == protocol.BombStateCountdown || b.state == protocol.BombStateDefusing
}

// resetBomb clears the bomb while keeping the series counters.
func (b *bombSeries) resetBomb() {
	b.decided = false
	b.state = protocol.BombStateReset
	b.planter, b.defuser = -1, -1
	b.planterRef, b.defuserRef = 0, 0
	b.position = [3]float32{}
}

// BombRound reports whether the room runs the bomb mission.
func (r *Room) BombRound() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Setting.Game.Mode == bombModeID
}

// beginBombSeries starts a fresh match at sub-round zero. Outside the bomb
// mission it does nothing, so every other mode keeps no series state at all.
func (r *Room) beginBombSeries() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Setting.Game.Mode != bombModeID {
		return
	}
	r.bomb = &bombSeries{
		roundIndex: 0,
		planter:    -1,
		defuser:    -1,
		state:      protocol.BombStateReset,
	}
	r.seriesWinner = 0
	r.bombOperators = [maxRoomSlots]uint32{}
	r.bombObserved = [maxRoomSlots]bool{}
	r.applyBombSeriesSettingLocked()
}

// applyBombSeriesSettingLocked writes the two game setting words the client reads
// a series entry from: the first-round flag at +172 and the sub-round index at
// +176 (cn-717's settings_flags and settings_index, which the reference rewrites
// at every entry).
func (r *Room) applyBombSeriesSettingLocked() {
	if r.bomb == nil {
		return
	}
	if r.bomb.roundIndex == 0 {
		r.Setting.Game.Values148[6] |= 1
	} else {
		r.Setting.Game.Values148[6] &^= 1
	}
	r.Setting.Game.Values148[7] = uint32(r.bomb.roundIndex)
}

// bombOperator checks the operator reference a seat reports. The first report from
// a seat teaches the number its client uses and every later report must repeat it,
// so one client cannot report a transition as another player's operator. What the
// number itself means never matters to the server: the seat comes from the
// connection, and the value is echoed back as it arrived.
func bombOperator(room *Room, slot int, ref uint32) bool {
	if slot < 0 || slot >= maxRoomSlots {
		return false
	}
	if known := room.bombOperators[slot]; known != 0 {
		return known == ref
	}
	room.bombOperators[slot] = ref
	return true
}

// bombCanOperate reports whether a seat may act on the bomb for a team: it has to
// be seated on that team and free to act. A bubbled seat is not: the client blocks
// its own actions there, so an operation that arrives from one is a forged report.
func (s *Server) bombCanOperate(room *Room, slot int, team uint8) bool {
	if slot < 0 || slot >= maxRoomSlots {
		return false
	}
	entry := room.slots[slot]
	if entry.session == nil || entry.team != team {
		return false
	}
	life := room.life
	return life != nil && life.Active(uint8(slot))
}

// handleBombState relaying one gcpBombState transition: the report is checked
// against the state it claims to follow and, once accepted, broadcast to the room
// exactly as it arrived.
func (s *Server) handleBombState(session *Session, packet protocol.Packet) error {
	// The mode is checked before the body is: 2012 belongs to this mission alone, and
	// a room that does not run it must swallow the packet exactly as it did before
	// the mode existed, instead of failing on a body shape it never promised.
	room := session.room
	if room == nil || !room.BombRound() {
		return nil
	}
	report, err := protocol.ReadBombState(packet.Payload)
	if err != nil {
		return err
	}
	room.mu.Lock()
	series := room.bomb
	if room.Setting.Game.Mode != bombModeID || series == nil ||
		room.phase != phasePlaying || series.decided {
		room.mu.Unlock()
		return nil
	}
	reason := s.bombTransition(room, session.slot, report)
	generation, roundIndex := room.generation, series.roundIndex
	room.mu.Unlock()
	if reason != "" {
		s.log.Debug("[Bomb] report refused", "room", room.ID, "slot", session.slot,
			"state", protocol.BombStateName(report.State), "reason", reason)
		return nil
	}
	room.Broadcast(protocol.CreateBombStateReply(report))
	if report.State == protocol.BombStateCountdown {
		s.watchBombDetonation(room, generation, roundIndex)
	}
	return nil
}

// bombDetonationWatch is how long an armed bomb may stay armed before the server
// ends the sub-round itself: the client's own countdown is sixty seconds, so a
// quiet round is a lost report, not a live bomb.
const bombDetonationWatch = 70 * time.Second

// watchBombDetonation ends a sub-round whose armed bomb nobody ever reported as
// exploded. Without it a single lost report would hang the series until the room
// emptied: the round clock deliberately ignores an armed bomb.
func (s *Server) watchBombDetonation(room *Room, generation, roundIndex int) {
	time.AfterFunc(bombDetonationWatch, func() {
		room.mu.Lock()
		series := room.bomb
		stale := room.generation != generation || series == nil || series.roundIndex != roundIndex ||
			room.phase != phasePlaying || series.decided ||
			series.state != protocol.BombStateCountdown
		if stale {
			room.mu.Unlock()
			return
		}
		report := protocol.BombState{
			Operator: series.planterRef,
			State:    protocol.BombStateExploded,
			X:        series.position[0],
			Y:        series.position[1],
			Z:        series.position[2],
		}
		series.state = protocol.BombStateExploded
		series.defuser, series.defuserRef = -1, 0
		attacking := series.attackingTeam()
		room.mu.Unlock()
		s.log.Warn("[Bomb] the armed bomb was never reported; the server detonates it",
			"room", room.ID, "round", roundIndex)
		room.Broadcast(protocol.CreateBombStateReply(report))
		s.endBombSubRound(room, attacking)
	})
}

// bombTransition applies one transition to the series and returns the reason it
// was refused, or an empty string once it took effect. The rules are the client's
// own: only the attacker plants, only the defender defuses, only the operator
// finishes what it started, and only the master reports the detonation.
func (s *Server) bombTransition(room *Room, slot int, report protocol.BombState) string {
	series := room.bomb
	// Only the plant carries the position; every later report repeats it exactly,
	// so a client cannot move a bomb it already armed.
	if report.State != protocol.BombStatePlanting &&
		(series.position[0] != report.X || series.position[1] != report.Y || series.position[2] != report.Z) {
		return "position"
	}
	if !bombOperator(room, slot, report.Operator) {
		return "operator"
	}
	switch report.State {
	case protocol.BombStatePlanting:
		if series.state != protocol.BombStateReset {
			return "state"
		}
		if !s.bombCanOperate(room, slot, series.attackingTeam()) {
			return "team"
		}
		series.planter, series.planterRef = slot, report.Operator
		series.position = [3]float32{report.X, report.Y, report.Z}
	case protocol.BombStateReset:
		if series.state != protocol.BombStatePlanting || !bombOwns(series.planter, slot, series.planterRef, report.Operator) {
			return "owner"
		}
		series.resetBomb()
		return ""
	case protocol.BombStateCountdown:
		if series.state != protocol.BombStatePlanting || !bombOwns(series.planter, slot, series.planterRef, report.Operator) {
			return "owner"
		}
		if !s.bombCanOperate(room, slot, series.attackingTeam()) {
			return "standing"
		}
	case protocol.BombStateDefusing:
		if series.state != protocol.BombStateCountdown {
			return "state"
		}
		if !s.bombCanOperate(room, slot, series.defendingTeam()) {
			return "team"
		}
		series.defuser, series.defuserRef = slot, report.Operator
	case protocol.BombStateDefuseInterrupted:
		if series.state != protocol.BombStateDefusing || !bombOwns(series.defuser, slot, series.defuserRef, report.Operator) {
			return "owner"
		}
		// The interrupted defuse returns to the armed countdown without restarting
		// the client's timer (kr-812 sub_4245B0 case 5).
		series.defuser, series.defuserRef = -1, 0
		series.state = protocol.BombStateCountdown
		return ""
	case protocol.BombStateDefused:
		if series.state != protocol.BombStateDefusing || !bombOwns(series.defuser, slot, series.defuserRef, report.Operator) {
			return "owner"
		}
		if !s.bombCanOperate(room, slot, series.defendingTeam()) {
			return "standing"
		}
		// A completed defuse clears the planter and the position, which is what the
		// mission-over report that follows is matched against.
		series.planter, series.planterRef = -1, 0
		series.position = [3]float32{}
	case protocol.BombStateExploded:
		// Detonation belongs to the room master and names the bomb's own planter
		// (kr-812 sub_807260's master/local-slot comparison).
		if !series.armed() {
			return "state"
		}
		if room.Master != slot {
			return "master"
		}
		if report.Operator != series.planterRef {
			return "planter"
		}
		series.defuser, series.defuserRef = -1, 0
	default:
		return "state"
	}
	series.state = report.State
	return ""
}

// bombOwns reports whether the reporting seat is the one that started an
// operation, with the reference it started it under.
func bombOwns(owner int, slot int, ownerRef, ref uint32) bool {
	return owner == slot && ownerRef == ref
}

// handleBombMissionOver accepts the master's sub-round result once the bomb state
// or a wiped team agrees with it, and hands the sub-round over to the series.
func (s *Server) handleBombMissionOver(session *Session, packet protocol.Packet) error {
	// Same order as the state report: no bomb mission, no parsing.
	room := session.room
	if room == nil || !room.BombRound() {
		return nil
	}
	report, err := protocol.ReadBombMissionOver(packet.Payload)
	if err != nil {
		return err
	}
	room.mu.Lock()
	series := room.bomb
	if room.Setting.Game.Mode != bombModeID || series == nil ||
		room.phase != phasePlaying || series.decided {
		room.mu.Unlock()
		return nil
	}
	if room.Master != session.slot {
		room.mu.Unlock()
		s.log.Warn("[Bomb] mission over from a seat that is not the room master",
			"room", room.ID, "slot", session.slot, "master", room.Master)
		return nil
	}
	if !s.bombMatchesResult(room, series, report) {
		room.mu.Unlock()
		s.log.Warn("[Bomb] mission over does not match the bomb", "room", room.ID,
			"team", report.WinningTeam, "planter", report.Planter, "defuser", report.Defuser,
			"state", protocol.BombStateName(series.state))
		return nil
	}
	winner := uint8(report.WinningTeam)
	room.mu.Unlock()
	s.endBombSubRound(room, winner)
	return nil
}

// bombMatchesResult checks the master's report against the bomb state or a team
// that has nobody left standing.
func (s *Server) bombMatchesResult(room *Room, series *bombSeries, report protocol.BombMissionOver) bool {
	winner := report.WinningTeam
	if winner != 1 && winner != 2 {
		return false
	}
	attacking, defending := series.attackingTeam(), series.defendingTeam()
	switch {
	case series.state == protocol.BombStateExploded:
		return uint8(winner) == attacking && report.Planter == series.planterRef &&
			report.Defuser == protocol.BombOperatorNone
	case series.state == protocol.BombStateDefused:
		return uint8(winner) == defending && report.Planter == protocol.BombOperatorNone &&
			report.Defuser == series.defuserRef
	case series.armed():
		// An armed bomb outlives the round clock, but a defending team wiped while
		// it counts down still loses the sub-round (kr-812 sub_52A640).
		return uint8(winner) == attacking && report.Planter == series.planterRef &&
			report.Defuser == protocol.BombOperatorNone && s.bombTeamEliminated(room, defending)
	default:
		// Before the plant the report carries two cleared operators and names the
		// team the wiped side leaves behind.
		return report.Planter == protocol.BombOperatorNone && report.Defuser == protocol.BombOperatorNone &&
			s.bombTeamEliminated(room, uint8(3-winner))
	}
}

// bombTeamEliminated reports whether a team has nobody left standing, counting the
// bubbled seats that can still be rescued (the reference's Alive or Bubble).
func (s *Server) bombTeamEliminated(room *Room, team uint8) bool {
	life := room.life
	for slot := range room.slots {
		if room.slots[slot].session == nil || room.slots[slot].team != team {
			continue
		}
		if life != nil && life.Standing(uint8(slot)) {
			return false
		}
	}
	return true
}

// bombObserve turns a dead seat into an observer instead of bringing it back. The
// bomb mission has no respawn: the client asks for one and the server answers with
// the scene object of the seat that stays down, broadcast because the observation
// replaces the character for every client, not only its own.
func (s *Server) bombObserve(room *Room, slot int) {
	if slot < 0 || slot >= maxRoomSlots {
		return
	}
	room.mu.Lock()
	announce := room.Setting.Game.Mode == bombModeID && room.phase == phasePlaying &&
		!room.bombObserved[slot]
	if announce {
		room.bombObserved[slot] = true
	}
	room.mu.Unlock()
	if !announce {
		return
	}
	actor := objectFor(slot)
	for _, member := range room.View() {
		if !member.Occupied || member.Session == nil {
			continue
		}
		_ = member.Session.Send(protocol.CreateObserverReply(actor))
	}
	s.log.Debug("[Bomb] seat observes", "room", room.ID, "slot", slot, "actor", actor,
		"round", room.bombRoundIndex())
}

// bombRoundIndex reads the sub-round index for the log.
func (r *Room) bombRoundIndex() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.bomb == nil {
		return -1
	}
	return r.bomb.roundIndex
}

// endBombSubRound records one decided sub-round and either settles the whole match
// or reloads the next stage.
func (s *Server) endBombSubRound(room *Room, winner uint8) {
	if winner != 1 && winner != 2 {
		return
	}
	room.mu.Lock()
	series := room.bomb
	if room.Setting.Game.Mode != bombModeID || series == nil || !room.Started ||
		room.phase != phasePlaying || series.decided {
		room.mu.Unlock()
		return
	}
	series.decided = true
	series.wins[winner-1]++
	matchOver := series.wins[winner-1] >= bombRoundsToWin
	series.winner = 0
	if matchOver {
		series.winner = winner
		room.seriesWinner = winner
	}
	roundIndex, wins := series.roundIndex, series.wins
	armed := series.armed()
	generation := room.generation
	if room.endTimer != nil {
		room.endTimer.Stop()
		room.endTimer = nil
	}
	if room.loadingTimer != nil {
		room.loadingTimer.Stop()
		room.loadingTimer = nil
	}
	// Nothing may start or decide anything while the banner shows.
	room.phase = phaseSettling
	members := room.membersLocked()
	room.mu.Unlock()

	s.log.Info("[Bomb] sub-round decided", "room", room.ID, "round", roundIndex,
		"winner", winner, "wins", wins, "match_over", matchOver, "armed", armed)
	// The client draws the sub-round banner from its own outcome word, the same
	// packet the zombie battle uses for a result the team totals cannot express.
	for _, member := range members {
		if !member.Occupied || member.Session == nil {
			continue
		}
		_ = member.Session.Send(protocol.CreateGameOutcomeReply(member.Team == winner))
	}
	room.mu.Lock()
	if room.phase != phaseSettling || room.generation != generation || room.bomb == nil {
		room.mu.Unlock()
		return
	}
	// The banner plays first either way: the next stage load and the match result
	// both arrive after it, never on top of it.
	if matchOver {
		room.endTimer = time.AfterFunc(bombRoundPause, func() {
			s.finishBombMatch(room, generation)
		})
	} else {
		room.endTimer = time.AfterFunc(bombRoundPause, func() {
			s.continueBombRound(room, generation)
		})
	}
	room.mu.Unlock()
}

// finishBombMatch settles the series once its last sub-round's banner played. The
// settlement reads the series record, so the last sub-round's kill totals never
// decide the match.
func (s *Server) finishBombMatch(room *Room, generation int) {
	room.mu.RLock()
	stale := room.generation != generation || room.Setting.Game.Mode != bombModeID ||
		room.bomb == nil || room.phase != phaseSettling || !room.Started || room.bomb.winner == 0
	room.mu.RUnlock()
	if stale {
		return
	}
	s.finishRound(room, false)
}

// continueBombRound reloads the stage for the next sub-round: the sides swap, the
// bomb starts unplanted and every client is taken through the same stage load a
// fresh round uses, so its map load report drives the countdown again.
func (s *Server) continueBombRound(room *Room, generation int) {
	room.mu.Lock()
	series := room.bomb
	if room.generation != generation || room.Setting.Game.Mode != bombModeID || series == nil ||
		room.phase != phaseSettling || !room.Started || series.winner != 0 {
		room.mu.Unlock()
		return
	}
	series.roundIndex++
	series.resetBomb()
	room.applyBombSeriesSettingLocked()
	room.generation++
	room.phase = phaseLoading
	room.Setting.State = phaseLoading
	room.defenseConfirmed = [maxRoomSlots]bool{}
	room.cancelConfirmed = [maxRoomSlots]bool{}
	room.characterReported = [maxRoomSlots]bool{}
	room.seedsSent = false
	room.respawnCount = [maxRoomSlots]uint32{}
	room.lastSpawn = [maxRoomSlots]uint32{}
	room.spawnUsedAt = [29]time.Time{}
	room.bombOperators = [maxRoomSlots]uint32{}
	room.bombObserved = [maxRoomSlots]bool{}
	room.life = nil
	room.published = [maxRoomSlots]protocol.BattleScore{}
	room.teamScores = [2]uint16{}
	roundIndex, attacking := series.roundIndex, series.attackingTeam()
	setting := room.Setting
	members := room.membersLocked()
	room.mu.Unlock()

	for _, member := range members {
		if member.Occupied {
			room.SetLoaded(member.Slot, false)
		}
	}
	if err := s.bindLife(room); err != nil {
		s.log.Error("[Bomb] the next sub-round could not bind its life tracker",
			"room", room.ID, "error", err)
		s.cancelRound(room)
		return
	}
	s.log.Info("[Bomb] next sub-round", "room", room.ID, "round", roundIndex,
		"attacking", attacking, "defending", 3-attacking)
	s.broadcastRoundEntry(room, roundEntry{
		setting: setting,
		mapName: protocol.FromU16(setting.Game.Strings16[1]),
		series:  &protocol.RoundSeries{Round: uint32(roundIndex), Attacking: uint32(attacking)},
	})
	s.prepareDefenseSeeds(room)
	// The client reloads the stage for every sub-round, so the loading gate starts
	// over: its map load report is what drives the countdown to the next start.
	room.mu.Lock()
	if room.loadingTimer != nil {
		room.loadingTimer.Stop()
	}
	room.loadingTimer = time.AfterFunc(loadingTimeout, func() {
		s.log.Warn("loading timed out", "room", room.ID)
		s.cancelRound(room)
	})
	room.mu.Unlock()
	// Safety net: a client that reloads but never reports would strand the series
	// on the loading gate. Rather than lose the match, run the countdown anyway --
	// the entry packet already told every client to load, and a client that does
	// report later is harmless because the gate has moved on.
	armedGeneration := room.bombGeneration()
	time.AfterFunc(bombLoadFallback, func() {
		s.forceBombSubRoundStart(room, armedGeneration)
	})
}

// bombLoadFallback is how long a series' next sub-round waits for load reports
// before it starts without them.
const bombLoadFallback = 15 * time.Second

// forceBombSubRoundStart drives the countdown to a start when no client reported
// its load for the sub-round.
func (s *Server) forceBombSubRoundStart(room *Room, generation int) {
	room.mu.Lock()
	if room.generation != generation || room.Setting.Game.Mode != bombModeID || room.bomb == nil ||
		room.phase != phaseLoading || !room.Started {
		room.mu.Unlock()
		return
	}
	room.phase = phaseCountdown
	room.tracking++
	tracking := room.tracking
	if room.loadingTimer != nil {
		room.loadingTimer.Stop()
		room.loadingTimer = nil
	}
	room.mu.Unlock()
	s.log.Warn("[Bomb] no load report for the next sub-round; counting down anyway",
		"room", room.ID, "round", room.bombRoundIndex())
	room.Broadcast(protocol.CreateEverybodyLoadedReply())
	room.Broadcast(protocol.CreateGameCountdownReply(uint32(countdownLength / time.Millisecond)))
	s.forceDefenseSeeds(room)
	room.mu.Lock()
	if room.endTimer != nil {
		room.endTimer.Stop()
	}
	room.endTimer = time.AfterFunc(countdownLength, func() {
		s.startRound(room, tracking)
	})
	room.mu.Unlock()
}

// bombGeneration reads the round generation the fallback fences itself with.
func (r *Room) bombGeneration() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.generation
}

// bombCancelDepartedOperator cancels an unfinished plant or defuse whose owner
// left the round. The cancellation is broadcast because every client holds the
// operation until it arrives: a plant returns to the unplanted state, an
// interrupted defuse returns to the armed countdown without restarting it.
func (s *Server) bombCancelDepartedOperator(room *Room) {
	room.mu.Lock()
	series := room.bomb
	if room.Setting.Game.Mode != bombModeID || series == nil || series.decided {
		room.mu.Unlock()
		return
	}
	owner, state, operator := -1, uint32(0), uint32(0)
	switch series.state {
	case protocol.BombStatePlanting:
		owner, state, operator = series.planter, protocol.BombStateReset, series.planterRef
	case protocol.BombStateDefusing:
		owner, state, operator = series.defuser, protocol.BombStateDefuseInterrupted, series.defuserRef
	}
	if owner < 0 || room.slots[owner].session != nil {
		room.mu.Unlock()
		return
	}
	report := protocol.BombState{
		Operator: operator,
		State:    state,
		X:        series.position[0],
		Y:        series.position[1],
		Z:        series.position[2],
	}
	if state == protocol.BombStateReset {
		series.resetBomb()
	} else {
		series.defuser, series.defuserRef = -1, 0
		series.state = protocol.BombStateCountdown
	}
	room.mu.Unlock()
	room.Broadcast(protocol.CreateBombStateReply(report))
	s.log.Info("[Bomb] the operation owner left the round",
		"room", room.ID, "slot", owner, "state", protocol.BombStateName(state))
}

// bombState reads the bomb's state word for the log.
func bombState(room *Room) uint32 {
	room.mu.RLock()
	defer room.mu.RUnlock()
	if room.bomb == nil {
		return protocol.BombStateReset
	}
	return room.bomb.state
}
