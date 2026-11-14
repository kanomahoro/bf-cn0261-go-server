package game

import (
	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
)

// handlePortalMove answers a map's contact request (C948 gcpPortalMove): the client
// walked into a portal and asks whether it may play that portal's motion.
//
// The reference checks only that the round is playing and that the sender owns the
// seat it speaks for, then broadcasts the same reply to every member -- its own
// sender included. The client resolves the region in its own map data and runs the
// spring/portal motion locally, so the server keeps no position authority here and
// never refuses a portal it does not know about: an unknown region simply resolves
// to nothing on the clients that receive it.
func (s *Server) handlePortalMove(session *Session, packet protocol.Packet) error {
	move, err := protocol.ReadPortalMove(packet)
	if err != nil {
		return err
	}
	room := session.room
	if room == nil || !seated(session) {
		return nil
	}
	room.mu.RLock()
	playing := room.Started && room.phase == phasePlaying
	room.mu.RUnlock()
	if !playing || room.SessionAt(session.slot) != session {
		return nil
	}
	room.Broadcast(protocol.CreatePortalMoveReply(move.PortalID, uint32(session.slot)))
	s.log.Debug("portal move", "room", room.ID, "slot", session.slot, "portal", move.PortalID)
	return nil
}
