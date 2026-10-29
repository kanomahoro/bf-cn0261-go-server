package game

import (
	"fmt"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
)

// Chat channel numbers used on the wire by C501/C502.
const (
	chatNearby = 0
	chatTeam   = 1
	chatParty  = 2
	// chatServerIdentity is the slot value the original uses for server-side
	// module messages.
	chatServerIdentity = 0xFFFD
)

// chatThrottle reproduces ChatDirectory::Admit: at most five messages inside a
// two second window, and the same text at most three times in a row, otherwise
// the account is muted for a minute.
type chatThrottle struct {
	lastActivity time.Time
	blockedUntil time.Time
	sent         []time.Time
	previous     []uint16
	repeated     int
}

// admit reports whether the message may be relayed, updating the throttle.
func (s *Server) admitChat(accountID uint32, text []uint16, now time.Time) bool {
	s.chatMu.Lock()
	defer s.chatMu.Unlock()
	if s.chats == nil {
		s.chats = map[uint32]*chatThrottle{}
	}
	for id, state := range s.chats {
		if now.Sub(state.lastActivity) > 2*time.Minute {
			delete(s.chats, id)
		}
	}
	state := s.chats[accountID]
	if state == nil {
		state = &chatThrottle{}
		s.chats[accountID] = state
	}
	state.lastActivity = now
	if now.Before(state.blockedUntil) {
		return false
	}
	kept := state.sent[:0]
	for _, sent := range state.sent {
		if now.Sub(sent) < 2*time.Second {
			kept = append(kept, sent)
		}
	}
	state.sent = kept
	if len(state.sent) == 0 {
		state.repeated = 0
	}
	repeated := 1
	if len(state.previous) == len(text) && protocol.FromU16(state.previous) == protocol.FromU16(text) {
		repeated = state.repeated + 1
	}
	if len(state.sent) >= 5 || repeated > 3 {
		state.blockedUntil = now.Add(60 * time.Second)
		state.sent = nil
		state.repeated = 0
		return false
	}
	state.sent = append(state.sent, now)
	state.previous = append([]uint16(nil), text...)
	state.repeated = repeated
	return true
}

// sendChatNotice pushes a server-side module chat line, throttled to one notice
// every ten seconds like the original.
func (s *Server) sendChatNotice(session *Session, text string) {
	now := time.Now()
	if now.Sub(session.lastChatNotice) < 10*time.Second {
		return
	}
	session.lastChatNotice = now
	notice, err := protocol.CreateChatNotice(protocol.U16String(text))
	if err != nil {
		return
	}
	_ = session.Send(notice)
}

// chatRequest is the normalised form of C304/C501/C502.
type chatRequest struct {
	channel  uint32
	text     []uint16
	legacy   bool
	teamOnly bool
}

// readChat normalises the three chat packets.
func readChat(packet protocol.Packet) (chatRequest, error) {
	r := protocol.NewReader(packet.Payload)
	switch packet.ID {
	case 304:
		text, err := r.String(protocol.SessionStringLimit)
		if err != nil {
			return chatRequest{}, err
		}
		if err := r.RequireEnd(); err != nil {
			return chatRequest{}, err
		}
		return chatRequest{channel: chatParty, text: text}, nil
	case 501:
		teamOnly, err := r.Boolean()
		if err != nil {
			return chatRequest{}, err
		}
		text, err := r.String(protocol.SessionStringLimit)
		if err != nil {
			return chatRequest{}, err
		}
		if err := r.RequireEnd(); err != nil {
			return chatRequest{}, err
		}
		channel := uint32(chatNearby)
		if teamOnly {
			channel = chatTeam
		}
		return chatRequest{channel: channel, text: text, legacy: true, teamOnly: teamOnly}, nil
	case 502:
		channel, err := r.Unsigned()
		if err != nil {
			return chatRequest{}, err
		}
		text, err := r.String(protocol.SessionStringLimit)
		if err != nil {
			return chatRequest{}, err
		}
		if err := r.RequireEnd(); err != nil {
			return chatRequest{}, err
		}
		return chatRequest{channel: channel, text: text}, nil
	}
	return chatRequest{}, fmt.Errorf("%w: unsupported chat packet %d", protocol.ErrInvalidPacket, packet.ID)
}

// replyFor builds the per-recipient payload for a chat line.
func (s *Server) chatReply(session *Session, request chatRequest, slot uint16, nickname []uint16) (protocol.Packet, error) {
	if request.legacy {
		return protocol.CreateSessionChatReply(slot, request.teamOnly, request.text)
	}
	return protocol.CreateModuleChatReply(slot, nickname, request.channel, request.text)
}

// handleChat implements the original routing: nearby/team in a room, nearby in
// the plaza, party to self, everything else answered with a system notice.
func (s *Server) handleChat(session *Session, packet protocol.Packet) error {
	request, err := readChat(packet)
	if err != nil {
		return err
	}
	if err := protocol.ValidateChatText(request.text); err != nil {
		return err
	}
	nickname, _ := session.Profile()
	if !s.admitChat(session.Account.ID, request.text, time.Now()) {
		s.sendChatNotice(session, "发言过于频繁，请稍后再试。")
		return nil
	}

	switch request.channel {
	case chatParty:
		// S201 establishes a one-member party, so party chat echoes to the
		// sender with the original packet type.
		var reply protocol.Packet
		if packet.ID == 304 {
			reply, err = protocol.CreatePartyChatReply(session.Account.ID, request.text)
		} else {
			reply, err = protocol.CreateModuleChatReply(chatServerIdentity, protocol.U16String(nickname.Nickname), chatParty, request.text)
		}
		if err != nil {
			return err
		}
		return session.Send(reply)
	}

	if request.channel > chatTeam {
		s.sendChatNotice(session, "该聊天频道尚未开放，请使用附近、队内或私聊。")
		return nil
	}

	if session.inPlaza {
		if request.channel == chatNearby {
			plaza, slot, ok := s.plazas.find(session.ID)
			if ok && plaza.Members[slot].initialized {
				reply, err := s.chatReply(session, request, uint16(slot), protocol.U16String(nickname.Nickname))
				if err != nil {
					return err
				}
				s.broadcastPlaza(plaza, reply, 0)
				return nil
			}
		}
		if !request.legacy {
			s.sendChatNotice(session, "广场没有对战队伍，请使用附近聊天。")
		}
		return nil
	}

	room := session.room
	if room != nil && session.slot >= 0 {
		slot := uint16(session.slot)
		team := room.Team(session.slot)
		reply, err := s.chatReply(session, request, slot, protocol.U16String(nickname.Nickname))
		if err != nil {
			return err
		}
		for _, member := range room.View() {
			if !member.Occupied {
				continue
			}
			if request.channel == chatTeam && member.Team != team {
				continue
			}
			_ = member.Session.Send(reply)
		}
		return nil
	}

	if !request.legacy {
		s.sendChatNotice(session, "附近聊天需要先进入广场或房间。频道发言请用 #频道 内容。")
	}
	return nil
}
