package game

import (
	"fmt"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
)

// The pirate battle (mode 101, wgtHorde) keeps no rules in the server: the room
// master's own client runs the wave manager and the spawner, exactly as the
// single-player prologue does (see internal/game/tutorial.go). What the server
// owes the round is the bookkeeping the client cannot do itself:
//
//   - repeat the host's wave reports (903/904) to every seat, because each
//     seat's wave manager waits for the reply before it continues,
//   - repeat the host's wave table (908) as 909, which is what installs it,
//   - answer the host's "every wave cleared" report (905) with 906 and only to
//     the host, which releases its wave latch; the round itself settles on the
//     stage word (724) that follows,
//   - consume the host's mob life-point reports (32920): they must not reach the
//     generic player life handler, which refuses a mob's object id and drops the
//     connection (the reference forwards them to the other seats, never back to
//     the sender of a build that already applied them),
//   - settle the round on the host's stage report (724) or on the clock.
//
// Waves, mobs, spawn points, ammunition and the AI all stay in the client. The
// spawn frames themselves are not repeated here: cn-717 is one of the builds whose
// master expands its spawn groups locally, and the match relay carries the
// concrete gpp frames to the other seats. The mode is cooperative: one to four
// seats, all on the first battle team, with the wave table's own difficulty taken
// from the map row the room picked.

// pirateState is one pirate round. It is created when the round starts and
// dropped when it ends, and the generation it carries keeps a stale timer or a
// late report from another round out of this one.
type pirateState struct {
	generation int
	// wave is the last wave the host reported; 0 before the first one.
	wave uint32
	// difficulty is MapMeta's difficulty of the chosen map (1..3); the round-start
	// log states it, and the client is told the same value in GameSetting +164.
	difficulty uint32
	// cleared is set once the host reported every wave cleared (905). The round is not
	// settled by that report: the stage word every client sends right after it (724)
	// decides, so a failure can never be preceded by a clear (Fantasy CHANGELOG
	// 2026-11-04: "the host's all-waves report no longer ends the round by itself:
	// the stage word that every client sends right after it decides").
	cleared bool
	// stageDone is set once the stage word settled the round, so a second report
	// cannot settle it twice.
	stageDone bool
}

// pirateMaxNpcBody bounds the opaque spawn bodies the host relays.
const pirateMaxNpcBody = 64 * 1024

// beginPirateRound installs the pirate round state, or does nothing outside
// mode 101. Called from enterGame before the round's generation moves on, so the
// state carries the generation that follows.
func (r *Room) beginPirateRound() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Setting.Game.Mode != pirateModeID {
		r.pirate = nil
		return
	}
	difficulty := uint32(0)
	if entry, ok := mapEntryDifficulty(r.Setting.Game.Strings16[3]); ok {
		difficulty = entry
	}
	// The reference server's own pirate round puts every seat on team 2: its S503 seat
	// record carries team 2, while this server left the seats on team 1. The client copies
	// the local seat's team to the mission's friendly NPCs, so the team decides which NPCs
	// the player may damage and which the wave manager treats as its own.
	for slot := range r.slots {
		if r.slots[slot].session != nil {
			r.slots[slot].team = pirateSeatTeam
		}
	}
	r.pirate = &pirateState{generation: r.generation + 1, difficulty: difficulty}
}

// PirateRound reports whether the room runs the pirate battle. Its round is decided
// by its waves, never by a personal score limit: the mode's kill limit word belongs
// to the room card, and a seat's pang count can reach it long before the last wave.
func (r *Room) PirateRound() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Setting.Game.Mode == pirateModeID
}

// pirateSeatTeam is the team a pirate round's seats play on, and pirateOtherTeam the
// side that wins when the waves are not cleared.
const (
	pirateSeatTeam  uint8 = 2
	pirateOtherTeam uint8 = 1
)

// mapEntryDifficulty reads the difficulty suffix of a pirate map key
// (npc_*_01/02/03). It is the only place the server needs the map row once the
// round runs: the client already knows the wave table it belongs to.
func mapEntryDifficulty(name []uint16) (uint32, bool) {
	key := protocol.FromU16(name)
	if len(key) < 2 {
		return 0, false
	}
	switch key[len(key)-2:] {
	case "01":
		return 1, true
	case "02":
		return 2, true
	case "03":
		return 3, true
	}
	return 0, false
}

// The pirate battle's own values, taken from the reference server's capture of
// npc_pirate03_prison01 (2026-11-14): +168 states -1 and the wave table reply repeats the
// host's table back to it unchanged. The +164 difficulty is per map (the mission's own
// 1/2/3), not a constant.
//
// The round's length is the client's own rule record: its C503 carries the map's MapMeta
// words, and a pirate map's TimeAttackTimeLimit is 300 s (its DeathMatchTimeLimit is 240).
// The pirate family plays TimeAttack, so S704 states 300000. The map's wave schedule runs
// to 223 s, which the reference's 240 s fallback left almost no room for.
const (
	pirateRoundLimitMS uint32 = 300000
	// pirateBlockLimitMS is what the room setting's four limit words state. The client's
	// own block carries the map's DeathMatchTimeLimit there (240 s on every pirate map),
	// while the round itself plays the TimeAttack limit.
	pirateBlockLimitMS   uint32 = 240000
	pirateSettingWord168 uint32 = 0xFFFFFFFF
)

// piratePacketID reports whether the packet belongs to a pirate round's own
// traffic. The numbers are shared with other modes, so the room's mode decides
// whether they are claimed here.
func piratePacketID(id uint16) bool {
	switch id {
	case 903, 904, 905, 908, // the wave manager
		32857, 32862, 32892, // spawn group, member mob, one AI object
		32866, 32873, // a mob replay, a wave's spawn group assignment
		32920,               // a mob's life points, consumed without an echo
		724,                 // the stage result
		736, 2021, 735, 519: // drops and reward cards, consumed without a grant
		return true
	}
	return false
}

// handlePiratePacket answers a pirate round's traffic. It returns false when the
// session is not in a pirate room, so the ordinary dispatch stays in charge.
func (s *Server) handlePiratePacket(session *Session, packet protocol.Packet) bool {
	room := session.room
	if room == nil || room.SettingCopy().Game.Mode != pirateModeID {
		return false
	}
	switch packet.ID {
	case 903, 904:
		s.pirateWave(session, room, packet)
	case 908:
		s.pirateWaveTable(session, room, packet)
	case 905:
		s.pirateAllWaves(session, room, packet)
	case 32857:
		// The reference answers an ordinary round's spawn group with nothing: cn-717 is
		// in npc_local_spawn_group_builds, so the master's own queue already ran the
		// request (kr-812 sub_4620A0). The group word is logged because a run has to
		// tell the lua's own group (1 for one player) from the AI script's groups.
		group := uint32(0)
		if word, err := pirateWaveWord(packet); err == nil {
			group = word
		}
		s.log.Debug("[Pirate] spawn group from the host", "room", room.ID,
			"group", group, "bytes", len(packet.Payload),
			"hex", fmt.Sprintf("%x", packet.Payload))
	case 32920:
		// gppLifePoint: the host's mob life points. The reference forwards these to the
		// other seats and does NOT echo them back to their sender on this build
		// (SpawnGroupRelayTests: cn-717 asserts IncludeSender == false, because the sender
		// already applied its own copy), so this case consumes the report without an
		// echo. Without it the generic player life handler refuses a mob's object id with
		// ErrInvalidPacket, and that drops the connection.
		if len(packet.Payload) == 0 || len(packet.Payload) > pirateMaxNpcBody {
			s.log.Warn("[Pirate] life point out of bounds", "session", session.ID,
				"bytes", len(packet.Payload))
			return true
		}
		s.log.Debug("[Pirate] mob life point from the host", "room", room.ID,
			"bytes", len(packet.Payload))

	case 32862, 32892, 32866, 32873:
		// Concrete NPC frames are not repeated by the round on this build: the match
		// relay carries a gpp frame to the other seats itself, and echoing it here
		// would create every mob twice.
		s.log.Debug("[Pirate] spawn frame from the host", "room", room.ID,
			"id", packet.ID, "bytes", len(packet.Payload))
	case 724:
		s.pirateStageResult(session, room, packet)
	case 736, 2021, 735:
		// Drops are consumed and granted nothing: this server hands out no items and
		// no Lucci for the mode (2026-11-12 decision). The packets stay in the debug
		// stream so a client that waits on them is visible.
		s.log.Debug("[Pirate] drop traffic ignored", "session", session.ID,
			"id", packet.ID, "bytes", len(packet.Payload))
	case 519:
		// The reward-card selection is refused rather than ignored: the dialog only
		// opens from a packet this server never sends, so an answer here means the
		// client opened it on its own, and a fail word closes it.
		s.log.Debug("[Pirate] reward card refused", "session", session.ID,
			"bytes", len(packet.Payload))
		_ = session.Send(protocol.Packet{ID: 744})
	}
	return true
}

// pirateHostReady reports whether the packet comes from the room master in a
// round that is actually being played.
func (s *Server) pirateHostReady(session *Session, room *Room, id uint16) bool {
	if room.MasterSlot() != session.slot {
		s.log.Debug("[Pirate] the wave manager belongs to the room master", "session", session.ID,
			"id", id, "slot", session.slot, "master", room.MasterSlot())
		return false
	}
	// The host's wave manager starts as soon as the stage loads, which is before
	// the countdown handout reaches every seat, so the load and countdown phases
	// accept wave traffic too.
	room.mu.RLock()
	live := room.Started
	room.mu.RUnlock()
	if !live {
		s.log.Debug("[Pirate] wave report outside play", "session", session.ID, "id", id)
		return false
	}
	return true
}

// pirateWave repeats a wave start (903) or a wave end (904) to every seat. Each
// seat's wave manager latches until the matching reply arrives, so the host waits
// for its own echo as well.
func (s *Server) pirateWave(session *Session, room *Room, packet protocol.Packet) {
	if !s.pirateHostReady(session, room, packet.ID) {
		return
	}
	wave, err := pirateWaveWord(packet)
	if err != nil {
		s.log.Warn("[Pirate] unreadable wave report", "session", session.ID, "id", packet.ID,
			"error", err)
		return
	}
	reply := uint16(904)
	if packet.ID == 904 {
		reply = 905
	}
	room.mu.Lock()
	if room.pirate != nil {
		room.pirate.wave = wave
	}
	room.mu.Unlock()
	room.Broadcast(protocol.Packet{ID: reply, Payload: packet.Payload})
	s.log.Info("[Pirate] wave", "room", room.ID, "wave", wave, "reply", reply,
		"seats", room.Occupied())
}

// pirateWaveTable answers the host's spawn table the way the reference server does:
// an empty Unknown908 first, then gspRepWaveAllSpawnMobs carrying the host's own
// table back to every member (the host included).
func (s *Server) pirateWaveTable(session *Session, room *Room, packet protocol.Packet) {
	if !s.pirateHostReady(session, room, 908) {
		return
	}
	if len(packet.Payload) == 0 || len(packet.Payload) > pirateMaxNpcBody {
		s.log.Warn("[Pirate] wave table out of bounds", "session", session.ID,
			"bytes", len(packet.Payload))
		return
	}
	room.Broadcast(protocol.Packet{ID: 908})
	room.Broadcast(protocol.Packet{ID: 909, Payload: packet.Payload})
	s.log.Info("[Pirate] wave table", "room", room.ID, "bytes", len(packet.Payload),
		"seats", room.Occupied())
}

// pirateAllWaves answers the host's cleared report: 906 goes back to the host
// alone (the other seats never wait on it) and releases its wave latch. The round
// is settled by the stage word that follows, not here.
func (s *Server) pirateAllWaves(session *Session, room *Room, packet protocol.Packet) {
	if !s.pirateHostReady(session, room, 905) {
		return
	}
	if len(packet.Payload) != 0 {
		s.log.Debug("[Pirate] the all-waves report must be empty", "session", session.ID,
			"bytes", len(packet.Payload))
		return
	}
	room.mu.Lock()
	state := room.pirate
	duplicate := state == nil || state.cleared
	if state != nil {
		state.cleared = true
	}
	room.mu.Unlock()
	if duplicate {
		s.log.Debug("[Pirate] a repeated cleared report", "room", room.ID)
		return
	}
	// Every seat's wave manager holds its clear until this reply arrives, and only the
	// host reports the waves: a seat that never receives it keeps the round unfinished
	// and shows the failed banner on its own client (2026-11-14, two-seat round: the
	// reporting host showed the cleared result, the other seat the failed one).
	room.Broadcast(protocol.Packet{ID: 906})
	s.log.Info("[Pirate] every wave cleared", "room", room.ID, "wave", state.wave,
		"seats", room.Occupied())
}

// pirateStageResult handles the host's stage result (724): word 1 is a cleared
// stage, word 0 a failed one. This report settles the round, never 905 on its own.
func (s *Server) pirateStageResult(session *Session, room *Room, packet protocol.Packet) {
	if !s.pirateHostReady(session, room, 724) {
		return
	}
	completed, err := pirateWaveWord(packet)
	if err != nil {
		s.log.Warn("[Pirate] unreadable stage result", "session", session.ID, "error", err)
		return
	}
	// The seats play on team 2 in this mode, so a cleared stage is a win for team 2.
	// The result packets compare each member's own team with this word (S705/S732 and
	// the per-member outcome), which is why it has to name the seats' team and not 1.
	winner := pirateOtherTeam
	if completed == 1 {
		winner = pirateSeatTeam
	}
	room.mu.Lock()
	state := room.pirate
	duplicate := state == nil || state.stageDone
	if state != nil {
		state.stageDone = true
		room.pirateWinner = winner
	}
	room.mu.Unlock()
	if duplicate {
		s.log.Debug("[Pirate] a repeated stage result", "room", room.ID, "completed", completed)
		return
	}
	s.log.Info("[Pirate] stage reported", "room", room.ID, "completed", completed, "winner", winner)
	s.finishRound(room, false)
}

// pirateWaveWord reads the one-word wave or stage report.
func pirateWaveWord(packet protocol.Packet) (uint32, error) {
	r := protocol.NewReader(packet.Payload)
	word, err := r.Unsigned()
	if err != nil {
		return 0, err
	}
	if err := r.RequireEnd(); err != nil {
		return 0, err
	}
	return word, nil
}

// pirateRoundDifficulty reports the difficulty the running pirate round took from
// its map row, and 0 outside a pirate round. The round-start log states it so a
// test can tell which row the room resolved.
func (r *Room) pirateRoundDifficulty() uint32 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.pirate == nil {
		return 0
	}
	return r.pirate.difficulty
}

// pirateSettingDifficulty reports the difficulty the room states in GameSetting
// +164, which is the value the client copies into its horde manager.
func (r *Room) pirateSettingDifficulty() uint32 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Setting.Game.Values148[4]
}
