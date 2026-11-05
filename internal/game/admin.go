package game

import (
	"fmt"
	"sort"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/store"
)

// AdminOverview is the console's one-glance panel.
type AdminOverview struct {
	UptimeSeconds    int64 `json:"uptimeSeconds"`
	Online           int   `json:"online"`
	Rooms            int   `json:"rooms"`
	RelayConnections int   `json:"relayConnections"`
}

// AdminPlayer is one logged-in session as the console lists it.
type AdminPlayer struct {
	SessionID   uint64 `json:"sessionId"`
	AccountID   uint32 `json:"accountId"`
	AccountName string `json:"accountName"`
	CharacterID uint32 `json:"characterId"`
	Nickname    string `json:"nickname"`
	Level       uint32 `json:"level"`
	Where       string `json:"where"`
	RoomID      uint16 `json:"roomId"`
	Slot        int    `json:"slot"`
	Address     string `json:"address"`
	OnlineFor   int64  `json:"onlineForSeconds"`
}

// AdminRoom is one room as the console lists it.
type AdminRoom struct {
	ID       uint16 `json:"id"`
	Title    string `json:"title"`
	Mode     uint32 `json:"mode"`
	Map      string `json:"map"`
	Master   int    `json:"master"`
	Occupied int    `json:"occupied"`
	Capacity uint16 `json:"capacity"`
	Phase    string `json:"phase"`
	Locked   bool   `json:"locked"`
}

// Snapshot reports the live state the console shows.
func (s *Server) Snapshot() AdminOverview {
	s.mu.Lock()
	online := len(s.online)
	rooms := len(s.rooms)
	s.mu.Unlock()
	relay := 0
	if s.relay != nil {
		relay = s.relay.Active()
	}
	return AdminOverview{
		UptimeSeconds:    int64(time.Since(s.startedAt).Seconds()),
		Online:           online,
		Rooms:            rooms,
		RelayConnections: relay,
	}
}

// AdminPlayers lists every logged-in session. A seat is resolved from the room
// roster rather than from the session's own fields: those are written by the
// session's goroutine, and the console only needs a consistent snapshot.
func (s *Server) AdminPlayers() []AdminPlayer {
	s.mu.Lock()
	sessions := make([]*Session, 0, len(s.online))
	for _, session := range s.online {
		sessions = append(sessions, session)
	}
	rooms := make([]*Room, 0, len(s.rooms))
	for _, room := range s.rooms {
		rooms = append(rooms, room)
	}
	s.mu.Unlock()

	type seat struct {
		room uint16
		slot int
	}
	seats := make(map[*Session]seat, len(sessions))
	for _, room := range rooms {
		for _, member := range room.View() {
			if member.Occupied {
				seats[member.Session] = seat{room: room.ID, slot: member.Slot}
			}
		}
	}

	out := make([]AdminPlayer, 0, len(sessions))
	for _, session := range sessions {
		character, _ := session.Profile()
		entry := AdminPlayer{
			SessionID:   session.ID,
			AccountID:   session.Account.ID,
			AccountName: session.Account.Name,
			CharacterID: character.ID,
			Nickname:    character.Nickname,
			Level:       s.LevelOf(character.Experience),
			Address:     session.peerAddress,
			OnlineFor:   int64(time.Since(session.startedAt).Seconds()),
			Slot:        -1,
			Where:       "登录中",
		}
		if found, ok := seats[session]; ok {
			entry.Where = "房间"
			entry.RoomID = found.room
			entry.Slot = found.slot
		} else if session.inPlaza {
			entry.Where = "广场"
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AccountID < out[j].AccountID })
	return out
}

// AdminRooms lists every room with its live state.
func (s *Server) AdminRooms() []AdminRoom {
	s.mu.Lock()
	rooms := make([]*Room, 0, len(s.rooms))
	for _, room := range s.rooms {
		rooms = append(rooms, room)
	}
	s.mu.Unlock()

	out := make([]AdminRoom, 0, len(rooms))
	for _, room := range rooms {
		setting := room.SettingCopy()
		out = append(out, AdminRoom{
			ID:       room.ID,
			Title:    protocol.FromU16(setting.Title),
			Mode:     setting.Game.Mode,
			Map:      protocol.FromU16(setting.Game.Strings16[1]),
			Master:   room.MasterSlot(),
			Occupied: room.Occupied(),
			Capacity: setting.Capacity,
			Phase:    room.phaseName(),
			Locked:   setting.PasswordRequired,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// phaseName renders the room phase for the console.
func (r *Room) phaseName() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	switch r.phase {
	case phaseOpen:
		return "等待"
	case phaseLoading:
		return "加载"
	case phaseCountdown:
		return "倒计时"
	case phasePlaying:
		return "对局中"
	case phaseSettling:
		return "结算"
	case phaseEnding:
		return "结束演出"
	default:
		return "未知"
	}
}

// LevelOf resolves the level a total experience value gives.
func (s *Server) LevelOf(experience uint32) uint32 {
	level, _, _ := s.prog.Level(experience)
	return level
}

// AdminKickAccount disconnects the account's live session, if it has one.
func (s *Server) AdminKickAccount(accountID uint32) bool {
	s.mu.Lock()
	session := s.online[accountID]
	s.mu.Unlock()
	if session == nil {
		return false
	}
	s.Disconnect(session)
	if connection, ok := session.send.(closer); ok {
		_ = connection.Close("removed by the administrator")
	}
	return true
}

// AdminDisbandRoom empties a room: a running round is cancelled first, then every
// member is sent back to the plaza and the room closes with the last one.
func (s *Server) AdminDisbandRoom(roomID uint16) (int, bool) {
	s.mu.Lock()
	room := s.rooms[roomID]
	s.mu.Unlock()
	if room == nil {
		return 0, false
	}
	if room.StartedReport() {
		s.cancelRound(room)
	}
	var members []*Session
	for _, member := range room.View() {
		if member.Occupied {
			members = append(members, member.Session)
		}
	}
	for _, session := range members {
		if err := s.leaveRoom(session); err != nil {
			s.log.Debug("disband leave failed", "session", session.ID, "error", err)
			continue
		}
		if err := s.enterPlaza(session); err != nil {
			s.log.Debug("disband plaza return failed", "session", session.ID, "error", err)
		}
	}
	s.log.Info("[Admin] room disbanded", "room", roomID, "members", len(members))
	return len(members), true
}

// BroadcastNotice sends one system chat line to every logged-in session and
// reports how many received it. The text follows the chat rules (1..256 UTF-16
// units, no control characters), so an over-long notice is refused instead of
// silently reaching nobody.
func (s *Server) BroadcastNotice(text string) (int, error) {
	notice, err := protocol.CreateChatNotice(protocol.U16String(text))
	if err != nil {
		return 0, fmt.Errorf("公告内容不合法：需 1~256 个字符且不含控制字符")
	}
	s.mu.Lock()
	targets := make([]*Session, 0, len(s.online))
	for _, session := range s.online {
		targets = append(targets, session)
	}
	s.mu.Unlock()
	delivered := 0
	for _, session := range targets {
		if err := session.Send(notice); err == nil {
			delivered++
		}
	}
	s.log.Info("[Admin] notice broadcast", "delivered", delivered, "text", text)
	return delivered, nil
}

// AdminSetBalance writes a wallet through the store and reports whether the row
// existed. The live session is deliberately untouched: the server reloads a
// balance at login, at a purchase and after a match, so a write is enough.
func (s *Server) AdminSetBalance(characterID, lucci, cash uint32) error {
	if err := s.db.SetBalance(characterID, lucci, cash); err != nil {
		return err
	}
	s.log.Info("[Admin] balance written", "character", characterID, "lucci", lucci, "cash", cash)
	return nil
}

// AdminSetBan bans or unbans an account, optionally disconnecting it. A ban also
// burns the tickets it already holds, so the player cannot log in with one.
func (s *Server) AdminSetBan(accountID uint32, disabled bool) (bool, error) {
	if err := s.db.SetAccountDisabled(accountID, disabled); err != nil {
		return false, err
	}
	var revoked int64
	if disabled {
		count, err := s.db.RevokeLoginTickets(accountID)
		if err != nil {
			// The ban itself is stored; a ticket that could not be burnt is worth a
			// warning, because it still opens the game until it expires.
			s.log.Warn("[Admin] ticket revocation failed", "account", accountID, "error", err)
		}
		revoked = count
	}
	kicked := false
	if disabled {
		kicked = s.AdminKickAccount(accountID)
	}
	s.log.Info("[Admin] account ban changed", "account", accountID, "disabled", disabled,
		"ticketsRevoked", revoked, "kicked", kicked)
	return kicked, nil
}

// AdminSearchCharacters finds players for the console's search box.
func (s *Server) AdminSearchCharacters(term string, limit int) ([]store.AdminCharacter, error) {
	return s.db.SearchCharacters(term, limit)
}
