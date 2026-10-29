package game

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/store"
)

// roomSlot is the per-slot state owned by the room mutex.
type roomSlot struct {
	session   *Session
	team      uint8
	ready     bool
	active    [2]bool
	loaded    bool
	character store.Character
	items     []store.Item
}

// memberView is an immutable copy of one slot, safe to use outside the lock.
type memberView struct {
	Slot      int
	Session   *Session
	Team      uint8
	Ready     bool
	Active    [2]bool
	Loaded    bool
	Occupied  bool
	Character store.Character
	Items     []store.Item
}

// Room is one session (room) with up to eight slots. Every mutable field is
// guarded by mu; network writes always happen outside the lock on a snapshot.
// maxRoomSlots is the fixed roster size of a Bubble Fighter room.
const maxRoomSlots = 8

type Room struct {
	mu sync.RWMutex

	ID           uint16
	Setting      protocol.SessionSetting
	Master       int
	Started      bool
	ChannelType  uint32
	ChannelIndex uint32

	slots        [maxRoomSlots]roomSlot
	life         *BattleLife
	pings        *PingTracker
	defenseSeeds map[int]defenseSeeds

	phase             int
	loadingTimer      *time.Timer
	defenseConfirmed  [8]bool
	cancelConfirmed   [8]bool
	characterReported [8]bool
	seedsSent         bool
	published         [8]protocol.BattleScore
	teamScores        [2]uint16

	tracking int
	// generation invalidates timers left over from a previous round.
	generation   int
	seedFallback *time.Timer
	startAt      time.Time
	endTimer     *time.Timer
}

// View returns an immutable snapshot of the roster.
func (r *Room) View() []memberView {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]memberView, 0, 8)
	for slot := range r.slots {
		entry := r.slots[slot]
		out = append(out, memberView{
			Slot:      slot,
			Session:   entry.session,
			Team:      entry.team,
			Ready:     entry.ready,
			Active:    entry.active,
			Loaded:    entry.loaded,
			Occupied:  entry.session != nil,
			Character: entry.character,
			Items:     entry.items,
		})
	}
	return out
}

// errRoomFull reports that no slot could be claimed.
var errRoomFull = fmt.Errorf("room is full")

// Round timing, matching the original GameRoundTiming defaults.
const (
	loadingTimeout  = 90 * time.Second
	countdownLength = 4 * time.Second
	settlementTries = 3
	settlementRetry = 200 * time.Millisecond

	// Post-match stage delays, from GameRoundTiming (MapMeta ticks).
	endAnnouncement    = 3 * time.Second
	winnerAnnouncement = 6 * time.Second
	resultDisplay      = 11 * time.Second
)

// Room phases, matching the original state values.
const (
	phaseOpen     = 2
	phaseLoading  = 3
	phasePlaying  = 4
	phaseSettling = 5
	// phaseCountdown is internal: the client keeps state 3 until S704 arrives.
	phaseCountdown = 6
	// phaseEnding covers S729/S705 up to the S732 winner stage.
	phaseEnding = 7
)

// UnsetGrade marks the "no grade yet" value used by the result screen.
const UnsetGrade uint32 = 0xFFFFFFFF

// ReserveSlot atomically checks capacity, picks a team and claims a slot, so
// two concurrent joins cannot exceed the configured capacity.
func (r *Room) ReserveSlot(session *Session, capacity int) (int, uint8, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if capacity <= 0 || capacity > len(r.slots) {
		capacity = len(r.slots)
	}
	if r.occupiedLocked() >= capacity {
		return -1, 0, errRoomFull
	}
	slot := -1
	for index, entry := range r.slots {
		if entry.session == nil {
			slot = index
			break
		}
	}
	if slot < 0 {
		return -1, 0, errRoomFull
	}
	team := uint8(1)
	for _, entry := range r.slots {
		if entry.session != nil && entry.team == 1 {
			team = 2
		}
	}
	r.slots[slot] = roomSlot{session: session, team: team,
		character: session.Character, items: session.Items}
	return slot, team, nil
}

// Add places a session into a slot.
func (r *Room) Add(slot int, session *Session, team uint8) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if slot < 0 || slot >= len(r.slots) || r.slots[slot].session != nil {
		return fmt.Errorf("room slot %d is unavailable", slot)
	}
	r.slots[slot] = roomSlot{session: session, team: team,
		character: session.Character, items: session.Items}
	return nil
}

// Remove clears a slot and reports the session that was there.
func (r *Room) Remove(slot int) *Session {
	r.mu.Lock()
	defer r.mu.Unlock()
	if slot < 0 || slot >= len(r.slots) {
		return nil
	}
	session := r.slots[slot].session
	r.slots[slot] = roomSlot{}
	return session
}

// Occupied counts the used slots.
func (r *Room) Occupied() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.occupiedLocked()
}

func (r *Room) occupiedLocked() int {
	count := 0
	for _, entry := range r.slots {
		if entry.session != nil {
			count++
		}
	}
	return count
}

// FreeSlot returns the first free slot or -1.
func (r *Room) FreeSlot() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for slot, entry := range r.slots {
		if entry.session == nil {
			return slot
		}
	}
	return -1
}

// Team reports a slot's current team.
func (r *Room) Team(slot int) uint8 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if slot < 0 || slot >= len(r.slots) {
		return 0
	}
	return r.slots[slot].team
}

// SetProfile refreshes the cached roster profile of a slot. Callers load the
// data in the owner's goroutine, so rosters never read another session's fields.
func (r *Room) SetProfile(slot int, character store.Character, items []store.Item) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if slot < 0 || slot >= len(r.slots) {
		return
	}
	r.slots[slot].character = character
	r.slots[slot].items = items
}

// SetReady updates the ready and activity flags.
func (r *Room) SetReady(slot int, ready bool, activity [2]bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if slot < 0 || slot >= len(r.slots) {
		return
	}
	r.slots[slot].ready = ready
	r.slots[slot].active = activity
}

// SetTeam flips or assigns a slot's team.
func (r *Room) SetTeam(slot int, team uint8) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if slot < 0 || slot >= len(r.slots) {
		return
	}
	r.slots[slot].team = team
}

// SetLoaded marks a slot as having finished loading the map.
func (r *Room) SetLoaded(slot int, loaded bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if slot < 0 || slot >= len(r.slots) {
		return
	}
	r.slots[slot].loaded = loaded
}

// Each iterates a snapshot of the occupied slots.
func (r *Room) Each(fn func(slot int, session *Session)) {
	for _, member := range r.View() {
		if member.Occupied {
			fn(member.Slot, member.Session)
		}
	}
}

// sessionsInto copies the occupied sessions into dst and returns how many were
// written. Callers hand in a fixed array, so an in-match broadcast costs no heap
// allocation; the slice returned by View is only needed by handlers that also
// want teams, characters and inventory.
func (r *Room) sessionsInto(dst *[maxRoomSlots]*Session) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	written := 0
	for slot := range r.slots {
		if session := r.slots[slot].session; session != nil {
			dst[written] = session
			written++
		}
	}
	return written
}

// SessionAt returns the session occupying one slot, or nil.
func (r *Room) SessionAt(slot int) *Session {
	if slot < 0 || slot >= len(r.slots) {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.slots[slot].session
}

// Broadcast sends a packet to every member. Sends happen outside the room lock,
// exactly like the View based version, but without allocating a member slice.
func (r *Room) Broadcast(p protocol.Packet) {
	var targets [maxRoomSlots]*Session
	count := r.sessionsInto(&targets)
	for i := 0; i < count; i++ {
		_ = targets[i].Send(p)
	}
}

// pingMembers builds the slot-to-session array of occupied slots.
func (r *Room) pingMembers() []uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	members := make([]uint64, 8)
	for slot, entry := range r.slots {
		if entry.session != nil {
			members[slot] = entry.session.ID
		}
	}
	return members
}

// requestPing starts a probe for a room member.
func (r *Room) requestPing(sessionID uint64, slot uint32) bool {
	r.mu.RLock()
	tracker := r.pings
	r.mu.RUnlock()
	if tracker == nil {
		return false
	}
	return tracker.Begin(sessionID, slot, r.pingMembers(), time.Now())
}

// completePing validates a responder report and returns the requester session.
func (r *Room) completePing(sessionID uint64, responderSlot, requesterSlot uint32) (uint64, bool) {
	r.mu.RLock()
	tracker := r.pings
	r.mu.RUnlock()
	if tracker == nil {
		return 0, false
	}
	return tracker.Complete(sessionID, responderSlot, requesterSlot, r.pingMembers(), time.Now())
}

// MasterSlot returns the master slot under the lock.
func (r *Room) MasterSlot() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Master
}

// ReassignMasterIfNeeded picks the lowest occupied slot when the master left.
func (r *Room) ReassignMasterIfNeeded() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.slots[r.Master].session != nil {
		return r.Master
	}
	for slot, entry := range r.slots {
		if entry.session != nil {
			r.Master = slot
			return slot
		}
	}
	return r.Master
}

// SettingCopy returns a copy of the room card.
func (r *Room) SettingCopy() protocol.SessionSetting {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Setting
}

// SetSetting replaces the room card.
func (r *Room) SetSetting(setting protocol.SessionSetting) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Setting = setting
}

// BeginStart marks the room as started and prepares the life tracker.
func (r *Room) BeginStart() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Started {
		return fmt.Errorf("room already started")
	}
	r.Started = true
	r.Setting.State = 3
	return nil
}

// StartedReport reports whether the match is running.
func (r *Room) StartedReport() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Started
}

// Life returns the life tracker.
func (r *Room) Life() *BattleLife {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.life
}

// refreshSetting recomputes the derived room-card counters.
func (r *Room) refreshSetting() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Setting.MemberCount = uint16(r.occupiedLocked())
	if r.Setting.Capacity == 0 {
		r.Setting.Capacity = 8
	}
}

// Members builds the wire roster from a snapshot.
func (s *Server) members(room *Room) []protocol.SessionMember {
	members := make([]protocol.SessionMember, 8)
	for _, member := range room.View() {
		if !member.Occupied {
			continue
		}
		character := s.svc.WireCharacter(member.Character)
		members[member.Slot] = protocol.SessionMember{
			Character:     &character,
			Items:         s.svc.WireItems(member.Items),
			Team:          member.Team,
			Ready:         member.Ready,
			ActivityFlags: member.Active,
		}
	}
	return members
}

func (s *Server) handleMoveInto(session *Session, packet protocol.Packet) error {
	request, err := protocol.ReadMoveIntoRequest(packet)
	if err != nil {
		return err
	}
	if session.Account.ID == 0 || request.AccountID != session.Account.ID {
		return session.Send(protocol.CreateMoveIntoReply(false, 0, 0))
	}
	// A single-channel server has nothing to redeem: the client that skipped the
	// channel request is placed in the default channel, and a client that still
	// holds a prepared transfer from an older build lands here too.
	session.channelType = DefaultChannelType
	session.channelIndex = DefaultChannelIndex
	if err := session.Send(protocol.CreateMoveIntoReply(true, session.channelType, session.channelIndex)); err != nil {
		return err
	}
	if err := session.Send(protocol.CreateGotoLobbyReply()); err != nil {
		return err
	}
	return s.sendMainMenu(session)
}

func (s *Server) handleSessionList(session *Session, packet protocol.Packet) error {
	request, err := protocol.ReadSessionListRequest(packet)
	if err != nil {
		return err
	}
	s.mu.Lock()
	rooms := make([]*Room, 0, len(s.rooms))
	for _, room := range s.rooms {
		rooms = append(rooms, room)
	}
	s.mu.Unlock()

	// Stable order by room id, then filter like the original List(): every room
	// in the channel is listed, including rooms already in a match.
	filtered := make([]*Room, 0, len(rooms))
	for _, room := range rooms {
		if room.ChannelType == session.channelType && room.ChannelIndex == session.channelIndex {
			filtered = append(filtered, room)
		}
	}
	for i := 1; i < len(filtered); i++ {
		for j := i; j > 0 && filtered[j].ID < filtered[j-1].ID; j-- {
			filtered[j], filtered[j-1] = filtered[j-1], filtered[j]
		}
	}

	first := int(request.Page) * int(request.Count)
	hasMore := false
	var cards [][]byte
	for index, room := range filtered {
		if index < first {
			continue
		}
		if len(cards) == int(request.Count) {
			hasMore = true
			break
		}
		room.refreshSetting()
		writer := protocol.NewWriter()
		if err := protocol.WriteSessionSetting(writer, room.SettingCopy()); err != nil {
			return err
		}
		cards = append(cards, writer.Payload())
	}
	reply, err := protocol.CreateSessionListReply(uint32(request.Page), hasMore, cards)
	if err != nil {
		return err
	}
	return session.Send(reply)
}

func (s *Server) handleCreateRoom(session *Session, packet protocol.Packet) error {
	request, err := protocol.ReadCreateRoomRequest(packet)
	if err != nil {
		return err
	}
	if session.Character.ID == 0 || session.room != nil || !session.inPlaza {
		s.log.Info("room creation rejected", "reason", "not in lobby",
			"character", session.Character.ID, "inRoom", session.room != nil, "inPlaza", session.inPlaza)
		return session.Send(protocol.CreateRoomResult(true, 1))
	}
	if len(request.Setting.Title) > 64 ||
		len(request.Setting.Password) > 16 ||
		(!request.Setting.PasswordRequired && len(request.Setting.Password) != 0) ||
		(request.Setting.PasswordRequired && len(request.Setting.Password) == 0) {
		s.log.Info("room creation rejected", "reason", "title or password",
			"title", len(request.Setting.Title), "password", len(request.Setting.Password),
			"required", request.Setting.PasswordRequired)
		return session.Send(protocol.CreateRoomResult(true, 1))
	}
	game, err := normalizeGameSetting(request.Setting.Game, s.maps)
	if err != nil {
		s.log.Debug("room creation rejected", "error", err)
		return session.Send(protocol.CreateRoomResult(true, 1))
	}

	setting := request.Setting
	setting.State = 2
	setting.Kind = 2
	setting.Game = game
	setting.Capacity = 8

	s.mu.Lock()
	if len(s.rooms) >= 1024 {
		s.mu.Unlock()
		return session.Send(protocol.CreateRoomResult(true, 1))
	}
	room := &Room{
		ID:           s.nextRoom,
		Setting:      setting,
		Master:       0,
		ChannelType:  session.channelType,
		ChannelIndex: session.channelIndex,
	}
	s.nextRoom++
	if s.nextRoom == 0 {
		s.nextRoom = 1
	}
	room.Setting.RoomID = room.ID
	s.rooms[room.ID] = room
	s.mu.Unlock()

	if err := room.Add(0, session, 1); err != nil {
		s.mu.Lock()
		delete(s.rooms, room.ID)
		s.mu.Unlock()
		return session.Send(protocol.CreateRoomResult(true, 1))
	}
	s.leavePlaza(session)
	session.room = room
	session.slot = 0
	room.refreshSetting()

	if err := session.Send(protocol.CreateRoomResult(true, 0)); err != nil {
		return err
	}
	entry, err := protocol.CreateRoomEntryReply(room.SettingCopy(), 0, 0, s.members(room))
	if err != nil {
		return err
	}
	return session.Send(entry)
}

func (s *Server) handleJoinRoom(session *Session, packet protocol.Packet) error {
	request, err := protocol.ReadJoinRoomRequest(packet)
	if err != nil {
		return err
	}
	if session.Character.ID == 0 || session.room != nil || !session.inPlaza {
		return session.Send(protocol.CreateRoomResult(false, 5))
	}
	if (session.Character.CharacterType != 1 && session.Character.CharacterType != 2) ||
		protocol.CheckNickname(protocol.U16String(session.Character.Nickname)) != protocol.NicknameOK {
		return session.Send(protocol.CreateRoomResult(false, 5))
	}
	if s.accountInRoom(session.Account.ID) {
		return session.Send(protocol.CreateRoomResult(false, 5))
	}
	s.mu.Lock()
	room := s.rooms[uint16(request.RoomID)]
	s.mu.Unlock()
	if room == nil || room.StartedReport() || room.SettingCopy().State != 2 {
		return session.Send(protocol.CreateRoomResult(false, 5))
	}
	if room.ChannelType != session.channelType || room.ChannelIndex != session.channelIndex {
		return session.Send(protocol.CreateRoomResult(false, 5))
	}
	setting := room.SettingCopy()
	if setting.PasswordRequired && protocol.FromU16(setting.Password) != protocol.FromU16(request.Password) {
		return session.Send(protocol.CreateRoomResult(false, 4))
	}
	capacity := int(setting.Capacity)
	if capacity <= 0 || capacity > 8 {
		capacity = 8
	}
	slot, _, err := room.ReserveSlot(session, capacity)
	if err != nil {
		return session.Send(protocol.CreateRoomResult(false, 3))
	}
	s.leavePlaza(session)
	session.room = room
	session.slot = slot
	room.refreshSetting()

	if err := session.Send(protocol.CreateRoomResult(false, 0)); err != nil {
		return err
	}
	members := s.members(room)
	entry, err := protocol.CreateRoomEntryReply(room.SettingCopy(), uint32(room.MasterSlot()), uint32(slot), members)
	if err != nil {
		return err
	}
	if err := session.Send(entry); err != nil {
		return err
	}
	if slotReply, err := protocol.CreateRoomSlotReply(uint32(slot), members[slot]); err == nil {
		for _, member := range room.View() {
			if member.Occupied && member.Slot != slot {
				_ = member.Session.Send(slotReply)
			}
		}
	}
	if settingReply, err := protocol.CreateRoomSettingReply(room.SettingCopy()); err == nil {
		for _, member := range room.View() {
			if member.Occupied && member.Slot != slot {
				_ = member.Session.Send(settingReply)
			}
		}
	}
	return nil
}

func (s *Server) handleLeaveRoom(session *Session, packet protocol.Packet) error {
	if packet.ID != 404 || len(packet.Payload) != 0 {
		return fmt.Errorf("%w: expected empty ReqLeaveSession", protocol.ErrInvalidPacket)
	}
	if err := s.leaveRoom(session); err != nil {
		return err
	}
	// The original returns to the lobby scene after leaving a room.
	return s.enterPlaza(session)
}

func (s *Server) leaveRoom(session *Session) error {
	room := session.room
	if room == nil {
		return nil
	}
	slot := session.slot
	departure, err := protocol.CreateRoomDepartureReply(uint32(slot))
	if err != nil {
		return err
	}
	started := room.StartedReport()
	room.Remove(slot)
	// The departing member's relay transport goes with him; a round that was
	// already running is cancelled below and releases the rest.
	if s.relay != nil {
		s.relay.RevokeSlot(room.ID, uint8(slot))
	}
	// A member vanishing mid-match aborts the round for everyone else, exactly
	// like GameRoundCoordinator::CancelForSession (S736 + open state).
	if started {
		s.cancelRound(room)
	}
	room.mu.RLock()
	pings := room.pings
	room.mu.RUnlock()
	if pings != nil {
		pings.Remove(session.ID)
	}
	session.room = nil
	session.slot = -1

	if room.Occupied() == 0 {
		room.mu.Lock()
		if room.endTimer != nil {
			room.endTimer.Stop()
		}
		room.mu.Unlock()
		s.mu.Lock()
		delete(s.rooms, room.ID)
		s.mu.Unlock()
		if s.relay != nil {
			s.relay.ReleaseRoom(room.ID)
		}
		return nil
	}
	master := room.ReassignMasterIfNeeded()
	room.refreshSetting()

	if err := session.Send(departure); err != nil {
		return err
	}
	room.Broadcast(departure)
	if masterReply, err := protocol.CreateRoomMasterReply(uint32(master)); err == nil {
		room.Broadcast(masterReply)
	}
	return nil
}

func (s *Server) handleReady(session *Session, packet protocol.Packet) error {
	status, err := protocol.ReadReadyStatus(packet)
	if err != nil {
		return err
	}
	room := session.room
	if room == nil || room.SettingCopy().State != 2 {
		return nil
	}
	// The master is always treated as not-ready (original SetReadyStatus).
	ready := status.Ready && session.slot != room.MasterSlot()
	room.SetReady(session.slot, ready, status.ActivityFlags)
	slotReply, err := protocol.CreateRoomSlotReply(uint32(session.slot), s.members(room)[session.slot])
	if err != nil {
		return err
	}
	room.Broadcast(slotReply)
	return nil
}

func (s *Server) handleKick(session *Session, packet protocol.Packet) error {
	r := protocol.NewReader(packet.Payload)
	target, err := r.String(protocol.SessionStringLimit)
	if err != nil {
		return err
	}
	if err := r.RequireEnd(); err != nil {
		return err
	}
	room := session.room
	if room == nil {
		return nil
	}
	requester := protocol.U16String(session.Character.Nickname)
	if room.MasterSlot() != session.slot || room.SettingCopy().State != 2 {
		return session.Send(protocol.CreateRoomKickReply(requester, target, protocol.KickUnavailable))
	}
	nickname := protocol.FromU16(target)
	if nickname == session.Character.Nickname {
		return session.Send(protocol.CreateRoomKickReply(requester, target, protocol.KickSelf))
	}
	for _, member := range room.View() {
		if !member.Occupied || member.Slot == session.slot {
			continue
		}
		if member.Session.Nickname() != nickname {
			continue
		}
		reply := protocol.CreateRoomKickReply(requester, target, protocol.KickSuccess)
		// Both the requester and the kicked player see the result.
		_ = member.Session.Send(reply)
		_ = session.Send(reply)
		victim := member.Session
		if err := s.leaveRoom(victim); err != nil {
			return err
		}
		if settingReply, err := protocol.CreateRoomSettingReply(room.SettingCopy()); err == nil {
			room.Broadcast(settingReply)
		}
		// The kicked player returns to the lobby scene.
		return s.enterPlaza(victim)
	}
	return session.Send(protocol.CreateRoomKickReply(requester, target, protocol.KickNotInRoom))
}

func (s *Server) handleChangeTeam(session *Session, packet protocol.Packet) error {
	if packet.ID != 515 || len(packet.Payload) != 0 {
		return fmt.Errorf("%w: expected empty RoomChangeTeam", protocol.ErrInvalidPacket)
	}
	room := session.room
	if room == nil || room.SettingCopy().State != 2 {
		return nil
	}
	current := room.Team(session.slot)
	target := uint8(1)
	if current == 1 {
		target = 2
	}
	count := 0
	for _, member := range room.View() {
		if member.Occupied && member.Team == target {
			count++
		}
	}
	// A team holds at most half of the eight slots.
	if count >= 4 {
		return nil
	}
	room.SetTeam(session.slot, target)
	room.SetReady(session.slot, false, [2]bool{})
	reply, err := protocol.CreateRoomTeamReply(uint32(session.slot), target)
	if err != nil {
		return err
	}
	room.Broadcast(reply)
	slotReply, err := protocol.CreateRoomSlotReply(uint32(session.slot), s.members(room)[session.slot])
	if err != nil {
		return err
	}
	room.Broadcast(slotReply)
	return nil
}

func (s *Server) handleGameSetting(session *Session, packet protocol.Packet) error {
	r := protocol.NewReader(packet.Payload)
	var setting protocol.GameSetting
	for i := 0; i < 4; i++ {
		value, err := r.String(protocol.SessionStringLimit)
		if err != nil {
			return err
		}
		setting.Strings16[i] = value
	}
	var err error
	if setting.Flag4, err = r.Boolean(); err != nil {
		return err
	}
	if setting.Flag5, err = r.Boolean(); err != nil {
		return err
	}
	timeLimit, err := r.Unsigned()
	if err != nil {
		return err
	}
	scoreLimit, err := r.Unsigned()
	if err != nil {
		return err
	}
	setting.Values148[0] = timeLimit
	setting.Values148[1] = scoreLimit
	for i := 0; i < 6; i++ {
		if _, err := r.Unsigned(); err != nil {
			return err
		}
	}
	if setting.Mode, err = r.Unsigned(); err != nil {
		return err
	}
	if setting.SubMode, err = r.Unsigned(); err != nil {
		return err
	}
	if setting.Flag368, err = r.Boolean(); err != nil {
		return err
	}
	if err := r.RequireEnd(); err != nil {
		return err
	}
	room := session.room
	if room == nil || room.MasterSlot() != session.slot {
		return nil
	}
	normalized, err := normalizeGameSetting(setting, s.maps)
	if err != nil {
		return nil
	}
	current := room.SettingCopy()
	changed := !gameSettingEqual(current.Game, normalized)
	current.Game = normalized
	room.SetSetting(current)

	// A settings change clears every seat's ready flag in the reference server.
	// The seats must be republished with it: without S503 the clients keep
	// showing themselves ready while the server refuses the next start with
	// kNotReady, and a player has to toggle ready twice to recover.
	var wasReady []int
	if changed {
		for _, member := range room.View() {
			if !member.Occupied {
				continue
			}
			if member.Ready {
				wasReady = append(wasReady, member.Slot)
			}
			room.SetReady(member.Slot, false, member.Active)
		}
	}

	reply, err := protocol.CreateRoomGameSettingReply(normalized)
	if err != nil {
		return err
	}
	room.Broadcast(reply)
	for _, slot := range wasReady {
		if seat, err := protocol.CreateRoomSlotReply(uint32(slot), s.members(room)[slot]); err == nil {
			room.Broadcast(seat)
		}
	}
	return nil
}

// accountInRoom reports whether the account already occupies a room slot.
func (s *Server) accountInRoom(accountID uint32) bool {
	if accountID == 0 {
		return false
	}
	s.mu.Lock()
	rooms := make([]*Room, 0, len(s.rooms))
	for _, room := range s.rooms {
		rooms = append(rooms, room)
	}
	s.mu.Unlock()
	for _, room := range rooms {
		for _, member := range room.View() {
			if member.Occupied && member.Session.Account.ID == accountID {
				return true
			}
		}
	}
	return false
}

// gameSettingEqual compares two settings through their wire encoding, since the
// struct holds slices and is therefore not comparable.
func gameSettingEqual(a, b protocol.GameSetting) bool {
	left := protocol.NewWriter()
	if err := protocol.WriteGameSetting(left, a); err != nil {
		return false
	}
	right := protocol.NewWriter()
	if err := protocol.WriteGameSetting(right, b); err != nil {
		return false
	}
	return bytes.Equal(left.Payload(), right.Payload())
}

func (s *Server) handleSessionChat(session *Session, packet protocol.Packet) error {
	return s.handleChat(session, packet)
}

func (s *Server) handleModuleChat(session *Session, packet protocol.Packet) error {
	return s.handleChat(session, packet)
}

func (s *Server) handlePartyChat(session *Session, packet protocol.Packet) error {
	return s.handleChat(session, packet)
}

func (s *Server) handleEnterGame(session *Session, packet protocol.Packet) error {
	room := session.room
	if room == nil {
		return nil
	}
	start, err := protocol.ReadGameStartRequest(packet)
	if err != nil {
		return err
	}
	// The original rejects special-mode requests and event flags outright.
	if start.Flag4 {
		return session.Send(protocol.CreateGameEntryReply(protocol.StartUnavailable, nil, 2))
	}
	for _, flag := range start.EventFlags {
		if flag {
			return session.Send(protocol.CreateGameEntryReply(protocol.StartUnavailable, nil, 2))
		}
	}
	settingNow := room.SettingCopy()
	if room.MasterSlot() != session.slot || room.StartedReport() || settingNow.State != phaseOpen {
		return session.Send(protocol.CreateGameEntryReply(protocol.StartUnavailable, nil, 2))
	}
	teams := [2]int{}
	ready := true
	occupied := 0
	master := room.MasterSlot()
	for _, member := range room.View() {
		if !member.Occupied {
			continue
		}
		occupied++
		if member.Team == 1 {
			teams[0]++
		} else if member.Team == 2 {
			teams[1]++
		}
		if member.Slot != master && (!member.Ready || member.Active[0] || member.Active[1]) {
			ready = false
		}
	}
	if occupied < 2 {
		return session.Send(protocol.CreateGameEntryReply(protocol.StartAlone, nil, 2))
	}
	if !ready {
		return session.Send(protocol.CreateGameEntryReply(protocol.StartNotReady, nil, 2))
	}
	if teams[0] != teams[1] || teams[0]+teams[1] != occupied {
		return session.Send(protocol.CreateGameEntryReply(protocol.StartUnequal, nil, 2))
	}

	mapName := s.maps.DefaultRulesMap
	if name := protocol.FromU16(settingNow.Game.Strings16[1]); name != "" && name != "Random" {
		if _, ok := s.maps.Find(name); ok {
			mapName = name
		}
	} else if candidates := s.maps.Candidates(settingNow.Game.Mode, protocol.FromU16(settingNow.Game.Strings16[2])); len(candidates) > 0 {
		mapName = candidates[0].ResourceName
	}
	gameMap, ok := s.maps.Find(mapName)
	if !ok {
		gameMap = s.maps.Maps[0]
		mapName = gameMap.ResourceName
	}

	// Pin the resolved map into the setting: the client reports it back with its
	// relay character report, and S510 advertises the same value.
	room.mu.Lock()
	room.Setting.Game.Strings16[1] = protocol.U16String(mapName)
	room.mu.Unlock()
	if err := room.BeginStart(); err != nil {
		return session.Send(protocol.CreateGameEntryReply(protocol.StartUnavailable, nil, 2))
	}
	// Without a bound life tracker every peer life/kick event would be dropped
	// and no score would ever be published.
	if err := s.bindLife(room); err != nil {
		s.cancelRound(room)
		return session.Send(protocol.CreateGameEntryReply(protocol.StartUnavailable, nil, 2))
	}
	room.mu.Lock()
	room.generation++
	room.phase = phaseLoading
	room.Setting.State = phaseLoading
	room.defenseConfirmed = [8]bool{}
	room.cancelConfirmed = [8]bool{}
	room.characterReported = [8]bool{}
	room.seedsSent = false
	room.mu.Unlock()
	for _, member := range room.View() {
		if member.Occupied {
			room.SetLoaded(member.Slot, false)
		}
	}

	// Original order: room setting, every slot, the signature (game mode), then
	// S510 with state 3, then the peer/relay handoff.
	if setting, err := protocol.CreateRoomSettingReply(room.SettingCopy()); err == nil {
		room.Broadcast(setting)
	}
	members := s.members(room)
	for _, member := range room.View() {
		if !member.Occupied {
			continue
		}
		if slotReply, err := protocol.CreateRoomSlotReply(uint32(member.Slot), members[member.Slot]); err == nil {
			room.Broadcast(slotReply)
		}
	}
	if signature, err := protocol.CreateSessionSignatureReply(settingNow.Game.Mode); err == nil {
		room.Broadcast(signature)
	}
	room.Broadcast(protocol.CreateGameEntryReply(protocol.StartSuccess, protocol.U16String(mapName), phaseLoading))

	view := room.View()
	relayAddress, relayPort := s.relayAddress()
	for _, member := range view {
		if !member.Occupied {
			continue
		}
		_ = member.Session.Send(protocol.CreatePeerIdentityReply(objectFor(member.Slot)))
		for _, peer := range view {
			if !peer.Occupied {
				continue
			}
			if identity, err := protocol.CreatePeerTableReply(uint32(peer.Slot), objectFor(peer.Slot)); err == nil {
				_ = member.Session.Send(identity)
			}
		}
		_ = member.Session.Send(protocol.Packet{ID: 611})
		if !s.cfg.Relay.Enabled || relayPort == 0 {
			continue
		}
		key := newRelayKey()
		if s.relay != nil {
			s.relay.Register(key, room.ID, uint8(member.Slot), member.Session.peerAddress)
		}
		member.Session.relayKey = key
		member.Session.relayActive = true
		if endpoint, err := protocol.CreateRelayEndpointReply(relayAddress, relayPort, key); err == nil {
			_ = member.Session.Send(endpoint)
		}
	}
	s.prepareDefenseSeeds(room)

	// Loading must complete within the same timeout the original uses, otherwise
	// the room would stay locked forever.
	room.mu.Lock()
	if room.loadingTimer != nil {
		room.loadingTimer.Stop()
	}
	room.loadingTimer = time.AfterFunc(loadingTimeout, func() {
		s.log.Info("loading timed out", "room", room.ID)
		s.cancelRound(room)
	})
	room.mu.Unlock()
	return nil
}

// relayAddress resolves the advertised relay endpoint.
func (s *Server) relayAddress() ([4]byte, uint16) {
	var address [4]byte
	if parsed := net.ParseIP(s.cfg.Relay.AdvertisedAddress); parsed != nil {
		if v4 := parsed.To4(); v4 != nil {
			copy(address[:], v4)
		}
	}
	if address == ([4]byte{}) {
		address, _ = s.endpoint()
	}
	return address, s.cfg.Relay.ListenPort
}

// newRelayKey returns a fresh 20-byte relay session key.
func newRelayKey() protocol.RelayKey {
	var key protocol.RelayKey
	_, _ = rand.Read(key[:])
	return key
}

func (s *Server) handleMapLoaded(session *Session, packet protocol.Packet) error {
	// C701 carries four u32 fields and nothing else.
	r := protocol.NewReader(packet.Payload)
	for index := 0; index < 4; index++ {
		if _, err := r.Unsigned(); err != nil {
			return err
		}
	}
	if err := r.RequireEnd(); err != nil {
		return err
	}
	room := session.room
	if room == nil {
		return nil
	}
	room.SetLoaded(session.slot, true)
	reply, err := protocol.CreateMapLoadedReply(uint32(session.slot))
	if err != nil {
		return err
	}
	room.Broadcast(reply)

	all := true
	for _, member := range room.View() {
		if member.Occupied && !member.Loaded {
			all = false
		}
	}
	if !all {
		return nil
	}
	room.mu.Lock()
	if room.phase != phaseLoading {
		room.mu.Unlock()
		return nil
	}
	room.phase = phaseCountdown // guard: a repeated C701 must not restart it
	if room.loadingTimer != nil {
		room.loadingTimer.Stop()
	}
	room.mu.Unlock()

	room.Broadcast(protocol.CreateEverybodyLoadedReply())
	room.Broadcast(protocol.CreateGameCountdownReply(uint32(countdownLength / time.Millisecond)))
	s.maybeSendDefenseSeeds(room)
	// Fallback: if a client never reports, publish the tables 1.5s before the
	// countdown ends so the round can still start.
	fallback := time.AfterFunc(countdownLength-1500*time.Millisecond, func() {
		s.forceDefenseSeeds(room)
	})
	room.mu.Lock()
	room.seedFallback = fallback
	room.tracking++
	tracking := room.tracking
	room.mu.Unlock()

	room.mu.Lock()
	if room.endTimer != nil {
		room.endTimer.Stop()
	}
	room.endTimer = time.AfterFunc(countdownLength, func() {
		s.startRound(room, tracking)
	})
	room.mu.Unlock()
	return nil
}

// startRound begins play once the countdown elapsed and every member confirmed
// its character and defense seeds.
func (s *Server) startRound(room *Room, tracking int) {
	room.mu.Lock()
	if room.phase != phaseCountdown || !room.Started {
		room.mu.Unlock()
		return
	}
	missing := 0
	sent := room.seedsSent
	for slot := range room.slots {
		if room.slots[slot].session == nil {
			continue
		}
		if !room.defenseConfirmed[slot] || !room.cancelConfirmed[slot] {
			missing++
		}
	}
	if missing > 0 {
		// The reference server aborts the round here. Real clients do not always
		// echo the tables (they are advisory for damage prediction), so we only
		// warn and start: aborting leaves both players stuck on the ready screen.
		s.log.Warn("countdown ended without every defense seed confirmation",
			"room", room.ID, "missing", missing, "seedsSent", sent)
	}
	room.phase = phasePlaying
	room.Setting.State = phasePlaying
	duration := room.Setting.Game.Values128[2]
	if duration == 0 {
		duration = 240000
	}
	room.mu.Unlock()

	room.Broadcast(protocol.CreateGameStartReply(duration, uint32(tracking)))

	room.mu.Lock()
	if room.endTimer != nil {
		room.endTimer.Stop()
	}
	room.endTimer = time.AfterFunc(time.Duration(duration)*time.Millisecond, func() {
		s.finishRound(room, true)
	})
	room.mu.Unlock()
}

// cancelRound returns a started room to the open state and notifies everyone,
// matching GameRoundCoordinator::Cancel (S736 + S501 + S503).
func (s *Server) cancelRound(room *Room) {
	caller := "unknown"
	if _, file, line, ok := runtime.Caller(1); ok {
		caller = fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}
	s.log.Info("round cancelled", "room", room.ID, "caller", caller,
		"started", room.StartedReport(), "phase", room.phase)
	room.mu.Lock()
	room.generation++
	if room.endTimer != nil {
		room.endTimer.Stop()
		room.endTimer = nil
	}
	if room.loadingTimer != nil {
		room.loadingTimer.Stop()
		room.loadingTimer = nil
	}
	if room.seedFallback != nil {
		room.seedFallback.Stop()
		room.seedFallback = nil
	}
	room.Started = false
	room.phase = phaseOpen
	room.Setting.State = phaseOpen
	room.life = nil
	room.pings = nil
	room.defenseSeeds = map[int]defenseSeeds{}
	room.defenseConfirmed = [8]bool{}
	room.cancelConfirmed = [8]bool{}
	room.characterReported = [8]bool{}
	room.seedsSent = false
	for slot := range room.slots {
		room.slots[slot].ready = false
		room.slots[slot].loaded = false
	}
	members := make([]memberView, 0, 8)
	for slot := range room.slots {
		entry := room.slots[slot]
		members = append(members, memberView{
			Slot: slot, Session: entry.session, Team: entry.team,
			Occupied: entry.session != nil, Character: entry.character, Items: entry.items,
		})
	}
	setting := room.Setting
	room.mu.Unlock()

	if s.relay != nil {
		s.relay.ReleaseRoom(room.ID)
	}
	room.Broadcast(protocol.Packet{ID: 736})
	if reply, err := protocol.CreateRoomSettingReply(setting); err == nil {
		room.Broadcast(reply)
	}
	roster := s.members(room)
	for _, member := range members {
		if !member.Occupied || member.Slot < 0 || member.Slot >= len(roster) {
			continue
		}
		if slotReply, err := protocol.CreateRoomSlotReply(uint32(member.Slot), roster[member.Slot]); err == nil {
			room.Broadcast(slotReply)
		}
	}
}

func (s *Server) finishRound(room *Room, timeExpired bool) {
	room.mu.Lock()
	if !room.Started {
		room.mu.Unlock()
		return
	}
	room.Started = false
	room.Setting.State = phaseSettling
	if room.loadingTimer != nil {
		room.loadingTimer.Stop()
		room.loadingTimer = nil
	}
	if room.endTimer != nil {
		room.endTimer.Stop()
		room.endTimer = nil
	}
	teamScores := room.teamScores
	life := room.life
	room.life = nil
	room.published = [8]protocol.BattleScore{}
	room.teamScores = [2]uint16{}
	setting := room.Setting
	members := make([]memberView, 0, 8)
	for slot := range room.slots {
		entry := room.slots[slot]
		members = append(members, memberView{
			Slot: slot, Session: entry.session, Team: entry.team,
			Ready: entry.ready, Active: entry.active, Loaded: entry.loaded,
			Occupied:  entry.session != nil,
			Character: entry.character, Items: entry.items,
		})
	}
	room.mu.Unlock()

	// Ask every member for its damage trace before settling. Diagnostics only:
	// the original never blocks settlement on the answer.
	for _, member := range members {
		if member.Occupied {
			_ = member.Session.Send(protocol.CreateHealthTraceRequest())
		}
	}
	winner := uint8(0)
	if teamScores[0] > teamScores[1] {
		winner = 1
	} else if teamScores[1] > teamScores[0] {
		winner = 2
	}
	players := make([]store.ResultPlayer, 0, 8)
	for _, member := range members {
		if !member.Occupied {
			continue
		}
		session := member.Session
		reward := s.prog.Rewards.Loss
		if winner == 0 {
			reward = s.prog.Rewards.Draw
		} else if winner == member.Team {
			reward = s.prog.Rewards.Win
		}
		var score protocol.BattleScore
		if life != nil {
			if value, err := life.Score(uint8(member.Slot)); err == nil {
				score = value
			}
		}
		award := s.prog.EvaluateResult(score.BubbleCount, score.PangCount, score.RescueCount)
		players = append(players, store.ResultPlayer{
			Slot:           uint8(member.Slot),
			CharacterID:    member.Character.ID,
			AccountID:      session.Account.ID,
			Nickname:       member.Character.Nickname,
			Team:           member.Team,
			Bubbles:        score.BubbleCount,
			Pangs:          score.PangCount,
			Rescues:        score.RescueCount,
			ExperienceGain: reward.Experience,
			LucciGain:      reward.Lucci,
			Score:          award.Score,
			Grade:          award.Grade,
			Medal:          award.Medal,
		})
	}

	result := store.Result{
		RoomID:        room.ID,
		TrackingID:    uint32(roomIDTracking(room)),
		Mode:          setting.Game.Mode,
		SubMode:       setting.Game.SubMode,
		MapName:       protocol.FromU16(setting.Game.Strings16[1]),
		EndReason:     1,
		FirstScore:    teamScores[0],
		SecondScore:   teamScores[1],
		Winner:        winner,
		PolicyVersion: s.prog.Version,
		Players:       players,
	}
	if result.MapName == "" || result.MapName == "Random" {
		result.MapName = s.maps.DefaultRulesMap
	}
	// Relay keys stay valid through the end sequence; the original revokes them
	// only when the room returns to the waiting state (S736).
	// The original retries settlement up to three times, 200ms apart, then
	// cancels the round.
	var saveErr error
	for attempt := 1; attempt <= settlementTries; attempt++ {
		saveErr = s.svc.SaveResult(result)
		if saveErr == nil {
			break
		}
		s.log.Error("settlement failed", "room", room.ID, "attempt", attempt, "error", saveErr)
		if attempt < settlementTries {
			time.Sleep(settlementRetry)
		}
	}
	room.mu.Lock()
	room.phase = phaseEnding
	room.mu.Unlock()
	if saveErr != nil {
		s.cancelRound(room)
		return
	}
	// S729 result screen: computed from the committed progress of each slot.
	rows := make([]protocol.GameResultPlayer, 0, len(members))
	for _, player := range players {
		before := player.ExperienceGain
		for _, member := range members {
			if member.Occupied && uint8(member.Slot) == player.Slot {
				before = member.Character.Experience
			}
		}
		after := before + player.ExperienceGain
		award := s.prog.EvaluateResult(player.Bubbles, player.Pangs, player.Rescues)
		levelBefore, _, _ := s.prog.Level(before)
		level, levelStart, nextLevel := s.prog.Level(after)
		rows = append(rows, protocol.GameResultPlayer{
			Slot:           player.Slot,
			AccountID:      player.AccountID,
			Nickname:       protocol.U16String(player.Nickname),
			Team:           player.Team,
			Bubbles:        player.Bubbles,
			Pangs:          player.Pangs,
			Rescues:        player.Rescues,
			Level:          level,
			LevelStart:     levelStart,
			Experience:     after,
			NextLevel:      nextLevel,
			ExperienceGain: player.ExperienceGain,
			LucciGain:      player.LucciGain,
			LevelUp:        level > levelBefore,
			AwardScore:     award.Score,
			AwardGrade:     award.Grade,
			AwardMedal:     award.Medal,
		})
	}
	// S729 must reach every client before S705: the S705 handler reads the
	// frozen ScoreData snapshot.
	if resultReply, err := protocol.CreateGameResultReply(teamScores, rows); err == nil {
		room.Broadcast(resultReply)
	} else {
		s.log.Debug("game result packet rejected", "room", room.ID, "error", err)
	}
	room.Broadcast(protocol.CreateGameEndReply(timeExpired))

	// Staged end: winner (S732) -> result screen (S733) -> back to the waiting
	// room (S736), mirroring GameRoundCoordinator::AdvanceEndStage. Every stage
	// re-checks the round generation so a timer from an earlier match cannot
	// abort the next one.
	room.mu.Lock()
	generation := room.generation
	if room.endTimer != nil {
		room.endTimer.Stop()
	}
	room.endTimer = time.AfterFunc(endAnnouncement, func() {
		room.mu.Lock()
		if room.phase != phaseEnding || room.generation != generation {
			room.mu.Unlock()
			return
		}
		room.Setting.State = 6
		room.mu.Unlock()
		if winner, err := protocol.CreateGameWinnerReply(teamScores[0], teamScores[1]); err == nil {
			room.Broadcast(winner)
		}
		room.mu.Lock()
		if room.endTimer != nil {
			room.endTimer.Stop()
		}
		room.endTimer = time.AfterFunc(winnerAnnouncement, func() {
			room.mu.Lock()
			stale := room.phase != phaseEnding || room.generation != generation
			room.mu.Unlock()
			if stale {
				return
			}
			room.Broadcast(protocol.Packet{ID: 733})
			room.mu.Lock()
			if room.endTimer != nil {
				room.endTimer.Stop()
			}
			room.endTimer = time.AfterFunc(resultDisplay, func() {
				room.mu.Lock()
				stale := room.phase != phaseEnding || room.generation != generation
				room.mu.Unlock()
				if stale {
					return
				}
				for _, member := range members {
					if !member.Occupied {
						continue
					}
					// Settlement committed the rewards, so the roster snapshot taken
					// at round start still carries the pre-round experience and
					// Lucci. Reload the character before republishing it, otherwise
					// the client keeps showing the old totals until the next login.
					fresh, err := s.db.CharacterByAccount(member.Session.Account.ID)
					if err != nil {
						s.log.Debug("character reload failed", "session", member.Session.ID, "error", err)
						continue
					}
					member.Session.Character = fresh
					room.SetProfile(member.Slot, fresh, member.Items)
					if err := s.sendUserStatFor(member.Session, fresh, member.Items); err != nil {
						s.log.Debug("user stat refresh failed", "session", member.Session.ID, "error", err)
					}
				}
				s.cancelRound(room)
			})
			room.mu.Unlock()
		})
		room.mu.Unlock()
	})
	room.mu.Unlock()
}

func roomIDTracking(room *Room) int {
	room.mu.RLock()
	defer room.mu.RUnlock()
	return room.tracking
}
