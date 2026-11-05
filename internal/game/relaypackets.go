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
		!relayAlreadyForwards(packet.ID) {
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
	s.log.Warn("defense seeds forced without every character report", "room", room.ID)
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

// relaySeatGrace is how long a seat survives its relay transport dropping. The
// game connection is still up in that case, so the member may register a new
// relay connection; only when the grace period passes without one does the seat
// go, and the round keeps running for the rest either way.
const relaySeatGrace = 5 * time.Second

// RelayDisconnected handles a member's relay transport disappearing while the
// game connection is still up. A relay drop is not proof that the player left:
// the transport can be re-established, so the seat is held for relaySeatGrace
// and the round is never cancelled for it.
func (s *Server) RelayDisconnected(roomID uint16, slot uint8) {
	s.mu.Lock()
	room := s.rooms[roomID]
	s.mu.Unlock()
	if room == nil || !room.StartedReport() {
		return
	}
	s.log.Warn("relay transport lost; holding the seat",
		"room", roomID, "slot", slot, "grace", relaySeatGrace)
	time.AfterFunc(relaySeatGrace, func() { s.dropRelaySeat(roomID, slot) })
}

// dropRelaySeat removes a member whose relay never came back. The seat is
// vacated like any other departure: the rest of the round keeps playing.
func (s *Server) dropRelaySeat(roomID uint16, slot uint8) {
	if s.relay != nil && s.relay.HasSlot(roomID, slot) {
		return
	}
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
	s.log.Warn("relay seat dropped after the grace period", "room", roomID, "slot", slot)
	if err := s.leaveRoom(target); err != nil {
		s.log.Debug("leave after relay loss failed", "session", target.ID, "error", err)
		return
	}
	if err := s.enterPlaza(target); err != nil {
		s.log.Debug("plaza return after relay loss failed", "session", target.ID, "error", err)
	}
}
