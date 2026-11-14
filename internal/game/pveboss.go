package game

import (
	"fmt"
	"strconv"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
)

// The cooperative zombie raid (mode 16, wgtZombiePve) -- "僵尸袭击BOSS战".
//
// The map owns the fight. Every rel_zombie* row ships a <dir>_ZombiePve.ini that
// scripts the whole stage: the citizens and the two allied guardians spawn in the
// first seconds, the zombie groups run on their own clocks, the mission spawner
// groups G_0..G_5 exist for the rescue alerts, and the boss (NameID 31) arrives
// at clock 185000. None of that is the server's: the room master's client runs the
// script and the relay carries its NPC frames to the other seats, exactly as the
// pirate battle's wave manager does.
//
// What the client cannot do itself is the bookkeeping every seat has to agree on:
//
//   - the rescue mission list (S924) and each mission's start (S925), timeout
//     (S926) and success (S927), scheduled here because the alerts are the
//     round's shared clock,
//   - the PVE point table: the client reports the score unit of the NPC it killed
//     (C923) and every seat draws the same live ranking (S928 + S929),
//   - the three counted item slots (S930: heal 229, bazooka 146, air bomb 136)
//     and their confirmation (S715, to the user alone; the peers get the native
//     effect from the relay),
//   - the infection: a human who dies turns into a zombie (team 15) and the round
//     is lost once nobody is left on the human side; a zombie comes back on the
//     empty C925 revival request,
//   - the result: the master's stage word (C724) decides, the clock running out is
//     a failed run.
//
// Evidence for the numbers: BubbleFighter.Fantasy's audited cn-717 tables
// (packets/pve-rescue-missions.yaml, packets/pve-counted-items.yaml,
// packets/pve-points.yaml) and this client's own MapMeta rows / ZombiePve
// resources. The mission schedule, the points and the slot kinds are the legacy
// build's own configuration, not the modern one's.

// pveBossModeID is the client's wgtZombiePve game mode.
const pveBossModeID = 16

const (
	// pveHumanTeam is the side every player starts on: the map's citizens and
	// guardians spawn on team 2 as well, and its zombies and the boss on team 15.
	pveHumanTeam uint8 = 2
	// pveZombieTeam is the side a dead player is revived on.
	pveZombieTeam uint8 = 15
	// pveRoundLimitMs is the mode's own length: every raid map's MapMeta row
	// states TimeAttackTimeLimit 600 (seconds), and the mode plays TimeAttack.
	pveRoundLimitMs uint32 = 600000
	// pveMissionDurationMs is how long the team has to finish one rescue.
	pveMissionDurationMs uint32 = 25000
	// pveMissionCooldownMs is the quiet interval after a rescue before the next
	// alert; the legacy build's own configuration.
	pveMissionCooldownMs uint32 = 25000
	// pveMissionStopMs is the native clock at which the boss spawns and the
	// rescue schedule stops issuing new missions (the map's own CHR_SPAWNER_7
	// states 185000).
	pveMissionStopMs uint32 = 185000
)

// pveMissionStartMs holds each alert's start on the round clock, in list order.
var pveMissionStartMs = [6]uint32{20000, 45000, 70000, 95000, 120000, 145000}

// pveMissionIDs are the presentation rows a mission may pick
// (MissionInfo_Clt.xml's ids); the legacy corpus uses these eight.
var pveMissionIDs = []uint32{1, 4, 7, 10, 13, 16, 19, 22}

// pveCitizenNames are the CHR_PROPERTY names the map's mission spawners resolve,
// paired by index with pveMissionIDs.
var pveCitizenNames = []string{
	"ms_citizen", "ms_citizen1", "ms_citizen2", "ms_citizen3",
	"ms_citizen4", "ms_citizen5", "ms_citizen6", "ms_citizen7",
}

// pveMissionCounts is how many hostiles each map's mission group spawns per list
// index. Every row comes from the map's own ZombiePve resource (its
// CHR_MS_RAND_SPAWNER counts); rel_zombie04_1_Oneway ships no mission group at
// all, so it runs the native stage without alerts.
var pveMissionCounts = map[string][]uint32{
	"rel_zombie_01":          {2, 3, 3, 3, 4, 4},
	"rel_zombie02_Twobridge": {2, 3, 3, 3, 4, 4},
	"rel_zombie03_Circle":    {2, 3, 3, 4, 4, 4},
	"rel_zombie04_Oneway":    {2, 2, 3, 3, 4, 4},
	"rel_zombie04_1_Oneway":  {},
	"rel_zombie05_Tear":      {2, 2, 3, 3, 4, 4},
	"rel_zombie06_Frontal":   {2, 2, 3, 3, 4, 4},
}

// pveCountedItemKinds are the three UI slots, in slot order: the medipack, the
// bazooka and the air bomb the mode's ItemSpec enables (229/146/136). A pickup of
// one of these adds to the hub's count; it is not used on the spot.
var pveCountedItemKinds = []uint32{229, 146, 136}

// pveImmediateItemKinds are applied the moment they are picked up: the booster and
// the supply grenade (101 / 17), the reference's counted_immediate_item_kinds.
var pveImmediateItemKinds = []uint32{101, 17}

// pveCountedKind reports whether a dropped kind belongs to the item hub instead of
// being used on the spot.
func pveCountedKind(kind uint32) bool {
	for _, counted := range pveCountedItemKinds {
		if kind == counted {
			return true
		}
	}
	return false
}

// pvePointUnitValues is the score each reported unit index is worth. The table is
// the legacy build's own (FighterConfig's score units 0..15).
var pvePointUnitValues = [16]uint32{4, 3, 10, 11, 15, 15, 20, 20, 15, 15, 20, 20, 15, 15, 20, 20}

// pveCitizenCharacterType is the C927 character type of the rescued citizen: the
// report that says the mission's citizen died, which fails the alert no matter
// who killed it. The zombie types are 101 and 102.
const pveCitizenCharacterType = 100

// pveBossState is one raid round's shared state. It is created when the round
// starts and dropped when it ends; every timer it owns carries the generation it
// was armed for, so a stale one cannot write into the next round.
type pveBossState struct {
	generation int
	// mapKey is the resource directory whose mission groups the round plays.
	mapKey string
	// missions is the prepared list, empty for a map without mission groups.
	missions []protocol.PveMission
	// points are the PVE points each seat earned this round.
	points [maxRoomSlots]uint32
	// origin is when play began; the mission clock is offset from it.
	origin time.Time
	// active is the mission list index that is running, or -1.
	active int
	// next is the next list index to start.
	next int
	// killed counts the hostiles the human team already killed for the active
	// mission.
	killed uint32
	// nextStart is when the next alert is due.
	nextStart time.Time
	// deadline is when the active alert times out.
	deadline time.Time
	// timer is the single scheduler timer.
	timer *time.Timer
	// disposed stops the scheduler once the round ended or the schedule is over.
	disposed bool
	// stageDone records the master's stage word, so a repeated report cannot settle
	// the round twice.
	stageDone bool
	// unitReports counts the score-unit reports (C923) and unitScored those that paid
	// points. Both are printed when the stage settles: a run whose ranking stays at
	// zero is answered by these two numbers (a client that sends nothing looks
	// different from one whose units carry no value).
	unitReports int
	unitScored  int
	// unknownLogged counts the room-level packets a raid round did not claim and the
	// log has already printed, so a client that repeats one cannot flood the log.
	unknownLogged int
}

// beginPveBossRound installs the raid round state and puts every seat on the
// human team. The map key is the one the round resolved, because the mission
// counts belong to the stage the room actually loads.
//
// It returns the team notices the entry has to carry: the reference server sets
// every seat to team 2 and then announces that change to every member, the same
// way the zombie battle announces its human team. The seat records alone put the
// client's room view on team 2, but this client family's mode managers read the
// native team-change notice, so the round states it too.
func (r *Room) beginPveBossRound(mapKey string) []protocol.Packet {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Setting.Game.Mode != pveBossModeID {
		r.pveBoss = nil
		return nil
	}
	var notices []protocol.Packet
	for slot := range r.slots {
		if r.slots[slot].session == nil {
			continue
		}
		r.slots[slot].team = pveHumanTeam
		if notice, err := protocol.CreateRoomTeamReply(uint32(slot), pveHumanTeam); err == nil {
			notices = append(notices, notice)
		}
	}
	state := &pveBossState{generation: r.generation + 1, mapKey: mapKey, active: -1}
	if counts := pveMissionCounts[mapKey]; len(counts) > 0 {
		state.missions = buildPveMissions(counts)
	}
	r.pveBoss = state
	return notices
}

// buildPveMissions prepares a map's mission list. The mission id (and with it the
// citizen's property name) is drawn per entry, exactly like the reference server
// does; the counts come from the map in list order. The start time is on the
// client's match clock, which is ahead of play by the countdown.
func buildPveMissions(counts []uint32) []protocol.PveMission {
	missions := make([]protocol.PveMission, 0, len(counts))
	for index, count := range counts {
		role := pveRole(index, count)
		missions = append(missions, protocol.PveMission{
			ID:          pveMissionIDs[role],
			StartTimeMs: pveClockOffsetMs() + pveMissionStartMs[index],
			DurationMs:  pveMissionDurationMs,
			Arguments:   []string{"killmonster", "1", strconv.FormatUint(uint64(count), 10), pveCitizenNames[role]},
		})
	}
	return missions
}

// pveRole picks the presentation row for one mission. The choice is stable for a
// given (index, count) pair, which keeps a round's list reproducible for the log
// while still spreading the citizens across the eight rows.
func pveRole(index int, count uint32) int {
	return (index*3 + int(count)) % len(pveMissionIDs)
}

// PveBossRound reports whether the room plays the cooperative zombie raid.
func (r *Room) PveBossRound() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Setting.Game.Mode == pveBossModeID
}

// pveBossPrep returns the packets a member needs before the stage entry: the
// counted item slots and the rescue mission list. The client installs both when
// it builds its game UI, so they have to arrive before S510.
func (r *Room) pveBossPrep() []protocol.Packet {
	r.mu.RLock()
	state := r.pveBoss
	mode := r.Setting.Game.Mode
	r.mu.RUnlock()
	if mode != pveBossModeID || state == nil {
		return nil
	}
	var out []protocol.Packet
	if items, err := protocol.CreatePveItemsReply(pveCountedItemKinds); err == nil {
		out = append(out, items)
	}
	if len(state.missions) > 0 {
		if missions, err := protocol.CreateMapMissionInfoReply(state.missions); err == nil {
			out = append(out, missions)
		}
	}
	return out
}

// startPveBossSchedule publishes the initial ranking and arms the mission clock.
// Called once the round-start packet went out, so the client's own clock and the
// schedule share an origin.
func (s *Server) startPveBossSchedule(room *Room, generation int) {
	room.mu.Lock()
	state := room.pveBoss
	if state == nil || state.generation != generation || room.Setting.Game.Mode != pveBossModeID {
		room.mu.Unlock()
		return
	}
	state.origin = time.Now()
	if len(state.missions) == 0 {
		room.mu.Unlock()
		s.log.Info("[PveBoss] the map ships no mission groups; the native stage runs on its own",
			"room", room.ID, "map", state.mapKey)
		return
	}
	state.next = 0
	state.nextStart = state.origin.Add(time.Duration(pveMissionStartMs[0]) * time.Millisecond)
	rows := state.rankRowsLocked(room)
	missions := len(state.missions)
	mapKey := state.mapKey
	room.mu.Unlock()

	s.broadcastPveRanks(room, rows)
	s.log.Info("[PveBoss] the raid starts", "room", room.ID, "map", mapKey,
		"missions", missions, "points_units", len(pvePointUnitValues),
		"round_ms", pveRoundLimitMs)
	s.armPveBossTimer(room, generation, pveMissionStartMs[0])
}

// armPveBossTimer re-arms the single scheduler timer for the next decision.
func (s *Server) armPveBossTimer(room *Room, generation int, delayMs uint32) {
	room.mu.Lock()
	state := room.pveBoss
	if state == nil || state.generation != generation || state.disposed {
		room.mu.Unlock()
		return
	}
	if state.timer != nil {
		state.timer.Stop()
	}
	state.timer = time.AfterFunc(time.Duration(delayMs)*time.Millisecond, func() {
		s.pveBossTick(room, generation)
	})
	room.mu.Unlock()
}

// pveBossTick advances the mission schedule: it starts the next alert, times the
// active one out, or stops when the boss phase begins.
func (s *Server) pveBossTick(room *Room, generation int) {
	var (
		startIndex = -1
		failIndex  = -1
		missions   []protocol.PveMission
		nextDelay  uint32
		stepping   bool
	)
	room.mu.Lock()
	state := room.pveBoss
	if state == nil || state.generation != generation || state.disposed ||
		room.Setting.Game.Mode != pveBossModeID || !room.Started || room.phase != phasePlaying {
		room.mu.Unlock()
		return
	}
	now := time.Now()
	clock := uint32(now.Sub(state.origin)/time.Millisecond) + pveClockOffsetMs()
	// The active alert's own deadline comes first: a mission that is not finished
	// times out and the next one is announced.
	if state.active >= 0 && !now.Before(state.deadline) {
		failIndex = state.active
		state.active = -1
		state.nextStart = now.Add(time.Duration(pveMissionCooldownMs) * time.Millisecond)
	}
	if state.active < 0 {
		if state.next >= len(state.missions) {
			state.disposed = true
			room.mu.Unlock()
			s.log.Info("[PveBoss] every mission alert was played", "room", room.ID)
			if failIndex >= 0 {
				s.broadcastPveMissionTimeout(room, failIndex)
			}
			return
		}
		due := !now.Before(state.nextStart)
		// A mission must be finishable before the boss phase replaces the alert UI
		// (the map spawns the boss at clock 185000).
		if due && clock+pveMissionDurationMs >= pveMissionStopMs {
			state.disposed = true
			room.mu.Unlock()
			s.log.Info("[PveBoss] the boss phase is due; the alert schedule stops",
				"room", room.ID, "clock_ms", clock)
			if failIndex >= 0 {
				s.broadcastPveMissionTimeout(room, failIndex)
			}
			return
		}
		if due {
			startIndex = state.next
			state.next++
			state.active = startIndex
			state.killed = 0
			state.missions[startIndex].StartTimeMs = clock
			state.deadline = now.Add(time.Duration(pveMissionDurationMs) * time.Millisecond)
			missions = append([]protocol.PveMission(nil), state.missions...)
			nextDelay = pveMissionDurationMs
			stepping = true
		} else {
			nextDelay = uint32(state.nextStart.Sub(now) / time.Millisecond)
			stepping = true
		}
	} else {
		nextDelay = uint32(state.deadline.Sub(now) / time.Millisecond)
		stepping = true
	}
	room.mu.Unlock()

	if failIndex >= 0 {
		s.broadcastPveMissionTimeout(room, failIndex)
	}
	if startIndex >= 0 {
		for _, packet := range pveMissionPackets(missions, startIndex) {
			room.Broadcast(packet)
		}
		s.log.Info("[PveBoss] mission started", "room", room.ID, "index", startIndex,
			"mission_id", missions[startIndex].ID, "args", missions[startIndex].Arguments)
	}
	if stepping {
		if nextDelay < 1 {
			nextDelay = 1
		}
		s.armPveBossTimer(room, generation, nextDelay)
	}
}

// pveMissionPackets is the pair an alert start sends: the prepared list (its
// timing refreshed) and the index that begins.
func pveMissionPackets(missions []protocol.PveMission, index int) []protocol.Packet {
	var out []protocol.Packet
	if prepared, err := protocol.CreateMapMissionInfoReply(missions); err == nil {
		out = append(out, prepared)
	}
	return append(out, protocol.CreateResMissionStartReply(uint32(index)))
}

// pveClockOffsetMs is how far the client's match clock is ahead of the moment
// play begins: the countdown it spent loading the stage before S704 arrived. The
// mission times are on that clock, which is why the reference adds its own
// start_timer_ms to every configured time.
func pveClockOffsetMs() uint32 { return uint32(countdownLength / time.Millisecond) }

// broadcastPveMissionTimeout tells every seat that one alert failed; the word is
// the index the client moves on to.
func (s *Server) broadcastPveMissionTimeout(room *Room, index int) {
	room.Broadcast(protocol.CreateResMissionTimedOutReply(uint32(index + 1)))
	s.log.Info("[PveBoss] mission timed out", "room", room.ID, "index", index)
}

// broadcastPveRanks publishes the live ranking to every seat.
func (s *Server) broadcastPveRanks(room *Room, rows []protocol.PveRankRow) {
	packet, err := protocol.CreateNotifyRankInfoReply(rows)
	if err != nil {
		s.log.Warn("[PveBoss] rank rows rejected", "room", room.ID, "error", err)
		return
	}
	room.Broadcast(packet)
}

// rankRowsLocked builds the ranking rows for every occupied seat. The caller
// holds the room lock.
func (state *pveBossState) rankRowsLocked(room *Room) []protocol.PveRankRow {
	seats := make([]uint32, 0, maxRoomSlots)
	for slot := range room.slots {
		if room.slots[slot].session == nil {
			continue
		}
		seats = append(seats, uint32(slot))
	}
	rows := make([]protocol.PveRankRow, 0, len(seats))
	for _, seat := range seats {
		rank := uint32(1)
		for _, other := range seats {
			if state.points[other] > state.points[seat] {
				rank++
			}
		}
		rows = append(rows, protocol.PveRankRow{Seat: seat, Points: state.points[seat], Rank: rank})
	}
	return rows
}

// notePveUnknown prints a room-level packet a raid round did not claim. The mode's
// score-unit report is the one opcode this build's catalog only *infers* (gcpPvePangUnit
// at 923), so a run whose PVE ranking stays at zero has to show what the client
// actually sent. Bounded to the first few per round.
func (s *Server) notePveUnknown(room *Room, session *Session, packet protocol.Packet, preview []byte) {
	room.mu.Lock()
	state := room.pveBoss
	if state == nil || state.unknownLogged >= 30 {
		room.mu.Unlock()
		return
	}
	state.unknownLogged++
	count := state.unknownLogged
	room.mu.Unlock()
	s.log.Info("[PveBoss] unclaimed room packet", "room", room.ID, "session", session.ID,
		"id", packet.ID, "len", len(packet.Payload), "hex", fmt.Sprintf("%x", preview),
		"count", count)
}

// pveBossPacketID reports whether an id belongs to a raid round's traffic. The
// numbers are shared with other modes, so the room's mode decides.
func pveBossPacketID(id uint16) bool {
	switch id {
	case 710, 2023, // the client's live score reports (bubble / pop / rescue)
		921,                 // a counted item slot
		923,                 // the score unit of the NPC that died
		927,                 // a mission's progress report (C2S; the S2C reply of the same number is ours)
		925,                 // the empty PvE revival request (C2S)
		724,                 // the stage result
		32920,               // a mob's life points: consumed so the player path never sees them
		736, 2021, 735, 519, // drops and reward cards, consumed without a grant
		// Diagnostics only: these are the horde wave manager's numbers and the horde
		// spawn group. The reference server answers none of them in this mode, and the
		// round does not either -- but a raid whose client reports waves here has to be
		// visible in the log instead of vanishing into "unhandled packet".
		903, 904, 905, 908, 32857:
		return true
	}
	return false
}

// handlePveBossPacket answers a raid round's traffic. It returns false when the
// session is not in a raid room, so the ordinary dispatch stays in charge.
func (s *Server) handlePveBossPacket(session *Session, packet protocol.Packet) bool {
	room := session.room
	if room == nil || room.SettingCopy().Game.Mode != pveBossModeID {
		return false
	}
	switch packet.ID {
	case 710, 2023:
		// The client's own score reports. The live counters reach every seat through
		// the personal and team score rows this server already publishes on every life
		// event (S986), and the settlement table carries them again, so the report
		// needs no answer: claiming it only keeps it out of the "unclaimed" diagnostic.
		s.log.Debug("[PveBoss] score report", "room", room.ID, "slot", session.slot,
			"id", packet.ID, "bytes", len(packet.Payload))
	case 921:
		s.pveBossCountedItem(session, room, packet)
	case 923:
		s.pveBossPoints(session, room, packet)
	case 927:
		s.pveBossMissionProgress(session, room, packet)
	case 925:
		s.pveBossRevival(session, room, packet)
	case 724:
		s.pveBossStageResult(session, room, packet)
	case 32920:
		// gppLifePoint: the host's mob and boss life points. The relay forwards them
		// to the other seats; the generic player handler would refuse a mob's object
		// id and drop the connection, so the round consumes them without an echo.
		if len(packet.Payload) == 0 || len(packet.Payload) > 64*1024 {
			s.log.Warn("[PveBoss] life point out of bounds", "session", session.ID,
				"bytes", len(packet.Payload))
			return true
		}
		s.log.Debug("[PveBoss] mob life point", "room", room.ID, "bytes", len(packet.Payload))
	case 736, 2021, 735:
		// Drops and result detail are consumed and granted nothing: this server hands
		// out no items and no Lucci for the mode (the same decision the pirate battle
		// took on 2026-11-12). The packets stay in the debug stream so a client that
		// waits on them is visible.
		s.log.Debug("[PveBoss] drop traffic ignored", "session", session.ID,
			"id", packet.ID, "bytes", len(packet.Payload))
	case 903, 904, 905, 908:
		// A wave report from the horde's own manager. The raid's stage script runs on
		// the client, and the reference server answers no wave report in this mode, so
		// nothing is echoed; the line exists so a client that does report waves here
		// shows up in a log instead of in "unhandled packet".
		s.log.Debug("[PveBoss] wave report not part of this mode", "session", session.ID,
			"id", packet.ID, "bytes", len(packet.Payload))
	case 32857:
		// The horde spawn group. It is not repeated: cn-717 is one of the builds whose
		// master expands its spawn groups locally (the same finding the pirate battle
		// recorded), so an echo would create every mob twice.
		s.log.Debug("[PveBoss] spawn group logged", "session", session.ID,
			"bytes", len(packet.Payload))
	case 519:
		// The reward-card selection is refused rather than ignored: the dialog only
		// opens from a packet this server never sends, so an answer here means the
		// client opened it on its own, and a fail word closes it.
		s.log.Debug("[PveBoss] reward card refused", "session", session.ID,
			"bytes", len(packet.Payload))
		_ = session.Send(protocol.Packet{ID: 744})
	}
	return true
}

// pveBossPlaying reports whether a round that is actually being played is open
// for this session's report.
func (s *Server) pveBossPlaying(session *Session, room *Room) bool {
	room.mu.RLock()
	playing := room.Started && room.phase == phasePlaying
	room.mu.RUnlock()
	if !playing {
		s.log.Debug("[PveBoss] report outside play", "room", room.ID, "session", session.ID)
	}
	return playing
}

// pveBossCountedItem runs one counted item slot: the local client plays the
// effect itself and only needs the confirmation (S715 to the user alone); the
// peers receive the native effect over the relay.
func (s *Server) pveBossCountedItem(session *Session, room *Room, packet protocol.Packet) {
	if !s.pveBossPlaying(session, room) {
		return
	}
	slot, err := protocol.ReadPveItemUseRequest(packet)
	if err != nil || slot == 0 || slot > uint32(len(pveCountedItemKinds)) {
		s.log.Debug("[PveBoss] counted item refused", "room", room.ID, "slot", slot, "error", err)
		return
	}
	kind := pveCountedItemKinds[slot-1]
	_ = session.Send(protocol.CreatePveItemUsedReply(uint32(session.slot), kind))
	s.log.Debug("[PveBoss] counted item used", "room", room.ID, "slot", session.slot, "kind", kind)
}

// pveBossPoints credits one killed NPC's score unit and republishes the ranking.
func (s *Server) pveBossPoints(session *Session, room *Room, packet protocol.Packet) {
	if !s.pveBossPlaying(session, room) {
		return
	}
	unit, err := protocol.ReadPvePangUnit(packet)
	if err != nil {
		s.log.Debug("[PveBoss] unreadable score unit", "session", session.ID, "error", err)
		return
	}
	var (
		amount uint32
		seat   int
		rows   []protocol.PveRankRow
		logIt  bool
		reason string
	)
	room.mu.Lock()
	state := room.pveBoss
	slot := session.slot
	if state == nil || slot < 0 || slot >= maxRoomSlots || !room.Started ||
		room.phase != phasePlaying || room.slots[slot].session == nil {
		room.mu.Unlock()
		return
	}
	state.unitReports++
	// The first few reports are logged at Info so a single test run answers whether
	// the client reports units at all and what they carry; the rest stays in debug.
	logIt = state.unitReports <= 5
	switch {
	case unit == ^uint32(0) || unit >= uint32(len(pvePointUnitValues)):
		// A unit of -1 is an NPC spawned without a score row; it pays nothing.
		reason = "no value"
	case room.slots[slot].team != pveHumanTeam:
		// Converted seats fight the humans now; their kills do not score the raid.
		reason = "reporter turned"
	default:
		amount = pvePointUnitValues[unit]
		state.points[slot] += amount
		state.unitScored++
		seat = slot
		rows = state.rankRowsLocked(room)
	}
	room.mu.Unlock()

	if reason != "" {
		if logIt {
			s.log.Info("[PveBoss] score unit ignored", "room", room.ID, "slot", slot,
				"unit", unit, "reason", reason)
		} else {
			s.log.Debug("[PveBoss] score unit ignored", "room", room.ID, "slot", slot,
				"unit", unit, "reason", reason)
		}
		return
	}
	if logIt {
		s.log.Info("[PveBoss] score unit accepted", "room", room.ID, "slot", seat,
			"unit", unit, "amount", amount)
	}
	if point, err := protocol.CreateNotifyGamePointReply(uint32(seat), 0, amount); err == nil {
		room.Broadcast(point)
	}
	s.broadcastPveRanks(room, rows)
	s.log.Debug("[PveBoss] points", "room", room.ID, "slot", seat, "unit", unit, "amount", amount)
}

// pveBossMissionProgress answers C927 gcpMissionProcess: the character type that
// died under a mission. The citizen's death fails the alert at once, while a
// hostile's death counts toward the mission's own target.
func (s *Server) pveBossMissionProgress(session *Session, room *Room, packet protocol.Packet) {
	if !s.pveBossPlaying(session, room) {
		return
	}
	index, characterType, trailing, err := protocol.ReadMissionProgress(packet)
	if err != nil {
		s.log.Debug("[PveBoss] unreadable mission report", "session", session.ID, "error", err)
		return
	}
	if trailing != 0 {
		s.log.Debug("[PveBoss] mission report carries a non-zero tail", "session", session.ID,
			"tail", trailing)
	}
	var (
		succeeded  = false
		failed     = false
		killed     uint32
		generation int
		more       bool
	)
	room.mu.Lock()
	state := room.pveBoss
	slot := session.slot
	active := -1
	if state != nil {
		active = state.active
	}
	if state == nil || active < 0 || int(index) != active ||
		!room.Started || room.phase != phasePlaying || slot < 0 || slot >= maxRoomSlots {
		room.mu.Unlock()
		s.log.Debug("[PveBoss] mission report outside the active alert", "room", room.ID,
			"index", index, "active", active)
		return
	}
	counts := pveMissionCounts[state.mapKey]
	target := uint32(0)
	if active < len(counts) {
		target = counts[active]
	}
	switch characterType {
	case pveCitizenCharacterType:
		// The rescued citizen died: the alert fails no matter who killed it.
		failed = true
	case 101, 102:
		// A hostile died. Only the human side clears missions; a converted seat's
		// kill does not.
		if room.slots[slot].team != pveHumanTeam || target == 0 || state.killed >= target {
			room.mu.Unlock()
			return
		}
		state.killed++
		killed = state.killed
		if killed >= target {
			succeeded = true
		}
	default:
		room.mu.Unlock()
		s.log.Debug("[PveBoss] unsupported mission character type", "room", room.ID, "type", characterType)
		return
	}
	if failed || succeeded {
		state.active = -1
		state.nextStart = time.Now().Add(time.Duration(pveMissionCooldownMs) * time.Millisecond)
	}
	generation = state.generation
	more = state.next < len(state.missions)
	room.mu.Unlock()

	switch {
	case failed:
		room.Broadcast(protocol.CreateResMissionTimedOutReply(index + 1))
		s.log.Info("[PveBoss] mission failed: the citizen died", "room", room.ID, "index", index,
			"reporter", session.slot)
	case succeeded:
		room.Broadcast(protocol.CreateResMissionSucceededReply(index))
		s.log.Info("[PveBoss] mission cleared", "room", room.ID, "index", index,
			"hostiles", killed, "reporter", session.slot)
	}
	// The scheduler owns the next alert; wake it so the cooldown starts now. With no
	// alert left the timer it still holds is dropped instead of firing into nothing.
	if failed || succeeded {
		if more {
			s.armPveBossTimer(room, generation, pveMissionCooldownMs)
		} else {
			room.mu.Lock()
			if state := room.pveBoss; state != nil {
				state.stopLocked()
			}
			room.mu.Unlock()
		}
	}
}

// pveBossRevival answers the empty C925 gcpPveUseRevival: a converted seat asks
// to come back and the answer is the same team-15 notice the conversion used.
func (s *Server) pveBossRevival(session *Session, room *Room, packet protocol.Packet) {
	if err := protocol.ReadPveRevival(packet); err != nil {
		s.log.Debug("[PveBoss] revival with a body", "session", session.ID, "error", err)
		return
	}
	if !s.pveBossPlaying(session, room) {
		return
	}
	room.mu.RLock()
	team := uint8(0)
	if session.slot >= 0 && session.slot < maxRoomSlots {
		team = room.slots[session.slot].team
	}
	room.mu.RUnlock()
	if team != pveZombieTeam {
		s.log.Debug("[PveBoss] revival request from a human seat ignored", "room", room.ID,
			"slot", session.slot)
		return
	}
	// A zombie that fell comes back on the same team notice. The tracker has to go
	// with it, exactly as the conversion does: a seat left in the dead state would
	// have its damage refused and its next death report dropped, and the client
	// already shows it alive.
	if life := room.Life(); life != nil {
		life.ConvertTeam(uint8(session.slot), pveZombieTeam)
	}
	if notice, err := protocol.CreateRoomTeamReply(uint32(session.slot), pveZombieTeam); err == nil {
		room.Broadcast(notice)
	}
	s.log.Info("[PveBoss] a zombie seat came back", "room", room.ID, "slot", session.slot)
}

// pveBossDeathReport answers the client's native team-change request during a
// raid round: the local character died, so it turns. The request names no seat,
// and only the sender's own seat may turn.
func (s *Server) pveBossDeathReport(session *Session) {
	room := session.room
	if room == nil {
		return
	}
	room.mu.RLock()
	owner := session.slot >= 0 && session.slot < maxRoomSlots && room.slots[session.slot].session == session
	slot := session.slot
	room.mu.RUnlock()
	if !owner {
		return
	}
	s.pveBossDeathSlot(room, slot)
}

// pveBossDeathOfObject turns the victim a combat report named.
func (s *Server) pveBossDeathOfObject(room *Room, victimObject uint32) {
	if protocol.ValidateObject(victimObject) != nil {
		return
	}
	// A mob's object id may sit in the player partitions as well; only an object
	// this round actually bound may turn a seat.
	life := room.Life()
	if life == nil || !life.BoundObject(victimObject) {
		return
	}
	s.pveBossDeathSlot(room, int((victimObject-1000000)/100000))
}

// pveBossDeathSlot turns one dead human into a zombie. When the last present
// human turns, the raid is lost.
func (s *Server) pveBossDeathSlot(room *Room, slot int) {
	var (
		notice  *protocol.Packet
		convert bool
		humans  int
		failed  bool
	)
	room.mu.Lock()
	if !room.Started || room.phase != phasePlaying || room.Setting.Game.Mode != pveBossModeID ||
		room.pveBoss == nil || slot < 0 || slot >= maxRoomSlots || room.slots[slot].session == nil {
		room.mu.Unlock()
		return
	}
	if room.slots[slot].team == pveHumanTeam {
		room.slots[slot].team = pveZombieTeam
		convert = true
		if built, err := protocol.CreateRoomTeamReply(uint32(slot), pveZombieTeam); err == nil {
			notice = &built
		}
	}
	for index := range room.slots {
		if room.slots[index].session != nil && room.slots[index].team == pveHumanTeam {
			humans++
		}
	}
	if humans == 0 {
		failed = true
		room.pveWinner = pveOtherTeam
	}
	state := room.pveBoss
	if failed && state != nil {
		state.disposed = true
	}
	room.mu.Unlock()

	if convert {
		if life := room.Life(); life != nil {
			life.ConvertTeam(uint8(slot), pveZombieTeam)
		}
		if notice != nil {
			room.Broadcast(*notice)
		}
		s.log.Info("[PveBoss] seat turned", "room", room.ID, "slot", slot, "humans_left", humans)
	}
	// The last human is gone: whether this report is what turned them or a later
	// death of a seat that already turned, the raid is over.
	if failed {
		s.log.Info("[PveBoss] nobody is left on the human side", "room", room.ID,
			"turned", convert)
		s.finishRound(room, false)
	}
}

// pveOtherTeam is the side the result credits when the raid fails: the seats all
// play the human side (2), so a failed run belongs to the other one.
const pveOtherTeam uint8 = 1

// pveBossStageResult handles the master's stage word (C724): 1 a cleared stage,
// 0 a failed one. This report settles the round.
func (s *Server) pveBossStageResult(session *Session, room *Room, packet protocol.Packet) {
	if room.MasterSlot() != session.slot {
		s.log.Debug("[PveBoss] the stage result belongs to the room master", "room", room.ID,
			"slot", session.slot, "master", room.MasterSlot())
		return
	}
	completed, err := protocol.ReadTutorialEndStage(packet)
	if err != nil {
		s.log.Warn("[PveBoss] unreadable stage result", "session", session.ID, "error", err)
		return
	}
	winner := pveOtherTeam
	if completed {
		winner = pveHumanTeam
	}
	room.mu.Lock()
	state := room.pveBoss
	duplicate := state == nil || state.stageDone
	if state != nil {
		state.stageDone = true
		state.stopLocked()
	}
	room.pveWinner = winner
	room.mu.Unlock()
	if duplicate {
		s.log.Debug("[PveBoss] a repeated stage result", "room", room.ID, "completed", completed)
		return
	}
	room.mu.RLock()
	reports, scored := 0, 0
	if state := room.pveBoss; state != nil {
		reports, scored = state.unitReports, state.unitScored
	}
	room.mu.RUnlock()
	s.log.Info("[PveBoss] stage reported", "room", room.ID, "completed", completed, "winner", winner,
		"score_unit_reports", reports, "score_units_scored", scored)
	s.finishRound(room, false)
}

// stopPveBossTimer drops the scheduler; called from the round teardown paths.
// The caller holds the room lock.
func (state *pveBossState) stopLocked() {
	if state == nil {
		return
	}
	state.disposed = true
	if state.timer != nil {
		state.timer.Stop()
		state.timer = nil
	}
}
