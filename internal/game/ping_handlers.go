package game

import (
	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
)

// handlePingRequest answers C517: the requester's own slot is rebroadcast as
// S517 to its room or plaza, and only tracked responders are accepted.
func (s *Server) handlePingRequest(session *Session, packet protocol.Packet) error {
	slot, err := protocol.ReadSessionPingRequest(packet)
	if err != nil {
		return err
	}
	request, err := protocol.CreateSessionPingRequest(slot)
	if err != nil {
		return err
	}
	if room := session.room; room != nil {
		if room.requestPing(session.ID, slot) {
			room.Broadcast(request)
		}
		return nil
	}
	if session.inPlaza {
		if plaza, ok := s.plazas.requestPing(session.ID, slot); ok && plaza != nil {
			s.broadcastPlaza(plaza, request, 0)
		}
	}
	return nil
}

// handlePingResponse answers C518: a validated report is routed back to the
// requester only.
func (s *Server) handlePingResponse(session *Session, packet protocol.Packet) error {
	response, err := protocol.ReadSessionPingResponse(packet)
	if err != nil {
		return err
	}
	requesterID := uint64(0)
	found := false
	if room := session.room; room != nil {
		requesterID, found = room.completePing(session.ID, response.ResponderSlot, response.RequesterSlot)
	}
	if !found && session.inPlaza {
		requesterID, found = s.plazas.completePing(session.ID, response.ResponderSlot, response.RequesterSlot)
	}
	if !found {
		return nil
	}
	target := s.sessionByID(requesterID)
	if target == nil {
		return nil
	}
	reply, err := protocol.CreateSessionPingReply(response)
	if err != nil {
		return err
	}
	return target.Send(reply)
}

// sessionByID resolves a live session in a room or plaza.
func (s *Server) sessionByID(id uint64) *Session {
	if id == 0 {
		return nil
	}
	s.mu.Lock()
	rooms := make([]*Room, 0, len(s.rooms))
	for _, room := range s.rooms {
		rooms = append(rooms, room)
	}
	s.mu.Unlock()
	for _, room := range rooms {
		for _, member := range room.View() {
			if member.Occupied && member.Session.ID == id {
				return member.Session
			}
		}
	}
	s.plazas.mu.Lock()
	defer s.plazas.mu.Unlock()
	for _, plaza := range s.plazas.plazas {
		for _, member := range plaza.Members {
			if member.sessionID == id && member.session != nil {
				return member.session
			}
		}
	}
	return nil
}
