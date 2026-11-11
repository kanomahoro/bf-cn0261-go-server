package game

import (
	"errors"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
)

// HandleRelayPacket feeds one in-match packet that arrived over a member's relay
// connection into the game logic. The reference server routes every relay
// packet through the round coordinator before forwarding it, which is how
// character reports, defense confirmations and life events reach the server.
// RelayPeerFrameAllowed reports whether a relay packet is a peer frame a mode
// wants forwarded although the relay's own whitelist does not carry it. The 2015
// client sends its gpp traffic over the match relay whenever no direct peer link
// is available (kr-812 sub_80D4C0, cn-324 sub_6F7F10), which is every relay-only
// deployment: a horde round's spawn frames arrive there, and dropping them leaves
// the other seats with no mobs at all.
//
// 32857 (a spawn group) is deliberately absent: the master's client expands it
// into the concrete member and AI-object frames itself, so relaying the group as
// well would spawn every mob twice.
func (s *Server) RelayPeerFrameAllowed(roomID uint16, id uint16) bool {
	s.mu.Lock()
	room := s.rooms[roomID]
	s.mu.Unlock()
	if room == nil || room.SettingCopy().Game.Mode != pirateModeID {
		return false
	}
	room.mu.RLock()
	live := room.Started
	room.mu.RUnlock()
	if !live {
		return false
	}
	switch id {
	case 32862, 32892, // a spawn group member, one AI object
		32866, 32873: // a mob replay, a wave's spawn group assignment
		return true
	}
	return false
}

func (s *Server) HandleRelayPacket(roomID uint16, slot uint8, packet protocol.Packet) {
	s.mu.Lock()
	room := s.rooms[roomID]
	s.mu.Unlock()
	if room == nil {
		return
	}
	var session *Session
	if int(slot) < len(room.slots) {
		session = room.SessionAt(int(slot))
	}
	if session == nil {
		return
	}
	// In-match traffic runs over the relay, so it must also refresh the game
	// connection's idle timer.
	session.Touch()
	if err := s.handleRelayPacket(session, packet); err != nil {
		s.log.Debug("relay packet rejected", "session", session.ID, "id", packet.ID, "error", err)
	}
}

// handleRelayPacket validates the relay-side ids that carry game state. Packets
// without a server-side meaning are simply forwarded by the relay.
func (s *Server) handleRelayPacket(session *Session, packet protocol.Packet) error {
	room := session.room
	if room == nil {
		return nil
	}
	switch {
	case packet.ID == 3:
		// RelayUseReport: binds the character object for this round. The report
		// must name the map the round actually runs.
		setting := room.SettingCopy()
		resource := protocol.FromU16(setting.Game.Strings16[1])
		if _, err := protocol.ReadRelayReport(packet, uint8(session.slot), resource); err != nil {
			return err
		}
		room.mu.Lock()
		room.characterReported[session.slot] = true
		room.mu.Unlock()
		s.maybeSendDefenseSeeds(room)
		return nil
	case packet.ID == 20 || packet.ID == 21:
		room.mu.Lock()
		seeds, ok := room.defenseSeeds[session.slot]
		sent := room.seedsSent
		room.mu.Unlock()
		if !ok || !sent {
			return errors.New("defense seeds were not requested")
		}
		if err := protocol.ValidateDefenseSeedReport(packet, seeds.Defense, seeds.Cancel); err != nil {
			return err
		}
		room.mu.Lock()
		if packet.ID == 20 {
			room.defenseConfirmed[session.slot] = true
		} else {
			room.cancelConfirmed[session.slot] = true
		}
		room.mu.Unlock()
		return nil
	case packet.ID == 8 || packet.ID == 9:
		return s.handleRelayPeerUpdate(session, packet)
	case packet.ID == 32904 && room.SettingCopy().Game.Mode == pirateModeID:
		// The respawn request is a gpp frame (0x8088), so a relay-only client sends
		// it on the match relay, where the relay hands it to the round instead of
		// forwarding it (exactly as it treats the game connection's C713). Only the
		// pirate battle opts in: every other mode is verified with this packet
		// arriving on its game connection and keeps the behaviour it had.
		return s.handleRespawn(session, packet)
	case protocol.IsPeerLife(packet.ID):
		return s.handlePeerLife(session, packet, true)
	case protocol.IsPeerKick(packet.ID):
		return s.handlePeerKick(session, packet, true)
	case packet.ID == 32773:
		return s.handlePeerAction(session, packet)
	case packet.ID == 32813:
		return s.handleReturnToNormal(session, packet, true)
	case packet.ID == 2015 || packet.ID == 2017, packet.ID == 17:
		// Intrusion sync and the game-start report carry no server state here.
		return nil
	case packet.ID == 16:
		return s.handleHealthReport(session, packet)
	case packet.ID == 18:
		return s.handleHealthReport(session, packet)
	case packet.ID == 32840:
		// Latency samples are echoed straight back to the reporter.
		if s.relay != nil {
			s.relay.SendToSlot(room.ID, uint8(session.slot), packet)
		}
		return nil
	case isItemRelayPacket(packet.ID):
		// Item-mode traffic the clients report between themselves. The relay
		// whitelist does not carry these ids, so they are forwarded here once the
		// box envelope checks out and the round really runs the item battle.
		return s.handleItemPeerPacket(session, packet, true)
	case packet.ID == 32877 || packet.ID == 32776 || packet.ID == 32777 || packet.ID == 32923:
		// Explosion style packets are validated elsewhere and forwarded by the relay.
		return nil
	}
	// A round that runs the item battle or the zombie battle lets the clients own
	// the rules, including the packets the server does not model: a turret's state,
	// a character whose look an item changed, the zombie king's rage effect,
	// whatever else the client puts on this connection. Dropping them is what made
	// those effects visible only to the player who caused them: the zombie log
	// showed the king's ultra broadcasting 32772/32783/32828/32812 that never
	// reached the humans. Packets the relay itself forwards are skipped so nothing
	// reaches a peer twice, and the seat's own respawn request stays server-side.
	if packet.ID >= 32768 && s.supplyMode(room) && packet.ID != 32904 &&
		!relayAlreadyForwards(packet.ID) && !s.RelayPeerFrameAllowed(room.ID, packet.ID) {
		s.forwardItemPacket(room, session.slot, packet, true)
	}
	return nil
}

// forceDefenseSeeds publishes the tables even without every report; used as a
// last-resort fallback shortly before the countdown ends.
func (s *Server) forceDefenseSeeds(room *Room) {
	room.mu.Lock()
	if room.seedsSent || room.phase != phaseCountdown {
		room.mu.Unlock()
		return
	}
	room.seedsSent = true
	room.mu.Unlock()
	s.log.Info("defense seeds forced without every character report", "room", room.ID)
	s.sendDefenseSeeds(room)
}

// maybeSendDefenseSeeds publishes the defense tables once every member has
// reported its character and the countdown is running.
func (s *Server) maybeSendDefenseSeeds(room *Room) {
	room.mu.Lock()
	if room.seedsSent || room.phase != phaseCountdown {
		room.mu.Unlock()
		return
	}
	// The reference server waits until every member reported its character over
	// the relay; clients crash if the tables arrive before they finish loading.
	for slot := range room.slots {
		if room.slots[slot].session == nil {
			continue
		}
		if !room.characterReported[slot] {
			room.mu.Unlock()
			return
		}
	}
	room.seedsSent = true
	room.mu.Unlock()
	s.sendDefenseSeeds(room)
	s.log.Debug("defense seeds published", "room", room.ID)
}

// RelayDisconnected handles a member's relay transport disappearing while the
// game connection is still up. Every peer packet travels over that transport, so
// a member without one cannot take part any more: the seat is released at once
// and the round keeps running for the rest.
func (s *Server) RelayDisconnected(roomID uint16, slot uint8) {
	s.mu.Lock()
	room := s.rooms[roomID]
	s.mu.Unlock()
	if room == nil || !room.StartedReport() {
		return
	}
	var target *Session
	for _, member := range room.View() {
		if member.Occupied && member.Slot == int(slot) {
			target = member.Session
			break
		}
	}
	if target == nil {
		return
	}
	// A killed client drops both of its connections, and they are noticed by two
	// different goroutines. When the game connection is handled first, Disconnect
	// has already taken the seat and the account: this session is on its way out,
	// and putting it back into the plaza would seat a character whose socket is
	// closed. That seat has no owner left to remove it, so it stays in the plaza
	// list for good.
	if target.closing.Load() {
		s.log.Debug("relay transport lost while the session was closing", "room", roomID, "slot", slot,
			"session", target.ID)
		return
	}
	// Disconnect unregisters the account last, so accountOnline alone can still
	// report a session that is already closing; the flag above covers that.
	if !s.accountOnline(target) {
		s.log.Debug("relay transport lost after the session ended", "room", roomID, "slot", slot,
			"session", target.ID)
		s.Disconnect(target)
		return
	}
	// The bomb mission keeps a series alive across sub-rounds, and a client drops
	// its relay transport when it reloads the stage for the next one. That teardown
	// is not a member leaving: hold the seat for a grace window so the client's
	// re-registration -- the key is reserved for the whole series -- can re-attach.
	if room.BombSeriesActive() {
		s.log.Info("[Bomb] relay transport dropped during the series; holding the seat",
			"room", roomID, "slot", slot, "session", target.ID)
		go s.bombRelayGrace(room, slot, target)
		return
	}
	s.releaseSeatAfterRelayLoss(room, slot, target)
}

// bombRelayGraceWait is how long a bomb seat survives without a relay transport.
// A client that reloads the next sub-round reconnects well inside it, and one that
// does not is gone for good.
const bombRelayGraceWait = 20 * time.Second

// bombRelayGrace releases a bomb seat whose relay transport never came back.
func (s *Server) bombRelayGrace(room *Room, slot uint8, target *Session) {
	time.Sleep(bombRelayGraceWait)
	if s.relay != nil && s.relay.HasSlot(room.ID, slot) {
		s.log.Debug("[Bomb] relay transport came back", "room", room.ID, "slot", slot)
		return
	}
	if target.closing.Load() || !s.accountOnline(target) || !room.BombSeriesActive() {
		return
	}
	s.log.Warn("[Bomb] the relay transport did not come back; releasing the seat",
		"room", room.ID, "slot", slot, "session", target.ID)
	s.releaseSeatAfterRelayLoss(room, slot, target)
}

// releaseSeatAfterRelayLoss drops a seat whose relay transport is gone for good.
func (s *Server) releaseSeatAfterRelayLoss(room *Room, slot uint8, target *Session) {
	s.log.Warn("relay transport lost; releasing the seat", "room", room.ID, "slot", slot)
	if err := s.leaveRoom(target); err != nil {
		s.log.Debug("leave after relay loss failed", "session", target.ID, "error", err)
		return
	}
	if err := s.enterPlaza(target); err != nil {
		s.log.Debug("plaza return after relay loss failed", "session", target.ID, "error", err)
	}
}
