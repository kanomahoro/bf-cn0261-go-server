package game

import (
	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
)

// The pirate battle (mode 101, wgtHorde) keeps no rules in the server: the room
// master's own client runs the wave manager and the spawner, exactly as the
// single-player prologue does (see internal/game/tutorial.go). What the server
// owes the round is the bookkeeping the client cannot do itself:
//
//   - repeat the host's wave reports (903/904/908) to every seat, because each
//     seat's wave manager waits for the reply before it continues,
//   - answer the host's "every wave cleared" report (905) with 906 and only to
//     the host, which is what ends the round,
//   - carry the spawn frames for the seats that are not the host,
//   - settle the round on the host's stage report (724) or on the clock.
//
// Waves, mobs, spawn points, ammunition and the AI all stay in the client. The
// mode is cooperative: one to four seats, all on the first battle team, with the
// wave table's own difficulty taken from the map row the room picked.

// pirateState is one pirate round. It is created when the round starts and
// dropped when it ends, and the generation it carries keeps a stale timer or a
// late report from another round out of this one.
type pirateState struct {
	generation int
	// wave is the last wave the host reported; 0 before the first one.
	wave uint32
	// difficulty is MapMeta's difficulty of the chosen map (1..3).
	difficulty uint32
	// cleared is set once the host reported every wave cleared, so a duplicate
	// report cannot settle the round twice.
	cleared bool
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
	r.pirate = &pirateState{generation: r.generation + 1, difficulty: difficulty}
}

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

// piratePacketID reports whether the packet belongs to a pirate round's own
// traffic. The numbers are shared with other modes, so the room's mode decides
// whether they are claimed here.
func piratePacketID(id uint16) bool {
	switch id {
	case 903, 904, 905, 908, // the wave manager
		32857, 32862, 32892, // spawn group, member mob, one AI object
		32866, 32873, // a mob replay, a wave's spawn group assignment
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
		// request (kr-812 sub_4620A0). Logged so a host that never names a real group
		// stays visible.
		s.log.Info("[Pirate] spawn group from the host", "room", room.ID,
			"bytes", len(packet.Payload))
	case 32862, 32892, 32866, 32873:
		// Concrete NPC frames are not repeated by the round on this build: the match
		// relay carries a gpp frame to the other seats itself (relayForwardable plus
		// RelayPeerFrameAllowed), and echoing it here would create every mob twice.
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

// pirateWaveTable answers the host's wave table (908) the way the client expects:
// the empty 908 clears the old mobs on every seat, the echoed 909 installs the
// table. The host's own latch clears on 909 too.
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
	s.log.Info("[Pirate] wave table", "room", room.ID, "bytes", len(packet.Payload))
}

// pirateAllWaves answers the host's cleared report: 906 goes back to the host
// alone (the other seats never wait on it), and the round is settled as a win.
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
		room.pirateWinner = 1
	}
	room.mu.Unlock()
	if duplicate {
		s.log.Debug("[Pirate] a repeated cleared report", "room", room.ID)
		return
	}
	_ = session.Send(protocol.Packet{ID: 906})
	s.log.Info("[Pirate] every wave cleared", "room", room.ID, "wave", state.wave)
	s.finishRound(room, false)
}

// pirateStageResult handles the host's stage result (724): word 1 is a cleared
// stage, word 0 a failed one.
func (s *Server) pirateStageResult(session *Session, room *Room, packet protocol.Packet) {
	if !s.pirateHostReady(session, room, 724) {
		return
	}
	completed, err := pirateWaveWord(packet)
	if err != nil {
		s.log.Warn("[Pirate] unreadable stage result", "session", session.ID, "error", err)
		return
	}
	winner := uint8(2)
	if completed == 1 {
		winner = 1
	}
	room.mu.Lock()
	state := room.pirate
	duplicate := state == nil || state.cleared
	if state != nil {
		state.cleared = true
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
