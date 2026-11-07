package game

import (
	"fmt"
	"sync"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/store"
)

// plazaCapacity is the number of members per plaza instance.
const plazaCapacity = 16

// plazaMember is one presence slot. Profiles are read from the owning session,
// so only session identity and presence state live here.
type plazaMember struct {
	sessionID   uint64
	session     *Session
	initialized bool
	position    *protocol.PlazaPosition
	character   store.Character
	items       []store.Item
}

// Plaza is one lobby instance inside a channel.
type Plaza struct {
	InstanceID   uint64
	ChannelType  uint32
	ChannelIndex uint32
	MasterSlot   int
	Members      [plazaCapacity]plazaMember
}

// Occupied counts used slots.
func (p *Plaza) Occupied() int {
	count := 0
	for _, member := range p.Members {
		if member.sessionID != 0 {
			count++
		}
	}
	return count
}

// Setting builds the plaza room card (kind 3 = lobby stage, room number 0).
func (p *Plaza) Setting() protocol.SessionSetting {
	setting := protocol.DefaultSessionSetting()
	setting.Kind = 3
	setting.State = 2
	setting.Title = protocol.U16String("Plaza")
	setting.Capacity = plazaCapacity
	setting.MemberCount = uint16(p.Occupied())
	return setting
}

// PlazaDirectory holds every lobby instance.
type PlazaDirectory struct {
	mu           sync.Mutex
	plazas       map[uint64]*Plaza
	pings        map[uint64]*PingTracker
	nextInstance uint64
}

// NewPlazaDirectory builds the directory.
func NewPlazaDirectory() *PlazaDirectory {
	return &PlazaDirectory{
		plazas:       map[uint64]*Plaza{},
		pings:        map[uint64]*PingTracker{},
		nextInstance: 1,
	}
}

// enter admits a session, allocating an instance when the channel is full.
func (d *PlazaDirectory) enter(session *Session, channelType, channelIndex uint32) (*Plaza, int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if session == nil || session.Character.ID == 0 || session.Account.ID == 0 ||
		!ChannelContains(channelType, channelIndex) {
		return nil, 0, fmt.Errorf("plaza admission requires a character and channel")
	}
	if plaza, slot, ok := d.findLocked(session.ID); ok {
		if plaza.ChannelType != channelType || plaza.ChannelIndex != channelIndex {
			return nil, 0, fmt.Errorf("leave the previous plaza before changing channel")
		}
		return plaza, slot, nil
	}
	var target *Plaza
	for _, plaza := range d.plazas {
		if plaza.ChannelType == channelType && plaza.ChannelIndex == channelIndex &&
			plaza.Occupied() < plazaCapacity {
			target = plaza
			break
		}
	}
	if target == nil {
		target = &Plaza{
			InstanceID:   d.nextInstance,
			ChannelType:  channelType,
			ChannelIndex: channelIndex,
		}
		d.nextInstance++
		d.plazas[target.InstanceID] = target
	}
	for slot := range target.Members {
		if target.Members[slot].sessionID == 0 {
			target.Members[slot] = plazaMember{
				sessionID: session.ID, session: session,
				character: session.Character, items: session.Items,
			}
			if target.Occupied() == 1 {
				target.MasterSlot = slot
			}
			return target, slot, nil
		}
	}
	return nil, 0, fmt.Errorf("plaza instance is full")
}

func (d *PlazaDirectory) findLocked(sessionID uint64) (*Plaza, int, bool) {
	if sessionID == 0 {
		return nil, 0, false
	}
	for _, plaza := range d.plazas {
		for slot, member := range plaza.Members {
			if member.sessionID == sessionID {
				return plaza, slot, true
			}
		}
	}
	return nil, 0, false
}

// find reports the session's plaza and slot.
// count reports how many instances exist (metrics and test helper).
func (d *PlazaDirectory) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.plazas)
}

func (d *PlazaDirectory) find(sessionID uint64) (*Plaza, int, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.findLocked(sessionID)
}

// leave removes a session and reports whether the master changed.
func (d *PlazaDirectory) leave(sessionID uint64) (*Plaza, int, bool, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	plaza, slot, ok := d.findLocked(sessionID)
	if !ok {
		return nil, 0, false, false
	}
	previousMaster := plaza.MasterSlot
	plaza.Members[slot] = plazaMember{}
	if previousMaster == slot {
		for index, member := range plaza.Members {
			if member.sessionID != 0 {
				plaza.MasterSlot = index
				break
			}
		}
	}
	changed := plaza.MasterSlot != previousMaster
	if plaza.Occupied() == 0 {
		delete(d.plazas, plaza.InstanceID)
		delete(d.pings, plaza.InstanceID)
	} else if tracker, ok := d.pings[plaza.InstanceID]; ok {
		tracker.Remove(sessionID)
	}
	return plaza, slot, changed, true
}

// initialize marks a member as scene-ready.
func (d *PlazaDirectory) initialize(sessionID uint64) (*Plaza, int, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	plaza, slot, ok := d.findLocked(sessionID)
	if !ok {
		return nil, 0, false
	}
	changed := !plaza.Members[slot].initialized
	plaza.Members[slot].initialized = true
	return plaza, slot, changed
}

// move records a position broadcast.
func (d *PlazaDirectory) move(sessionID uint64, position protocol.PlazaPosition) (*Plaza, int, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	plaza, slot, ok := d.findLocked(sessionID)
	if !ok {
		return nil, 0, false
	}
	copied := position
	plaza.Members[slot].position = &copied
	return plaza, slot, true
}

// characterHolder returns the session that already occupies this character's
// plaza seat, if it is not the excluded one. The caller decides whether that
// session is still alive: a seat whose session was never cleaned up must not
// block the player from coming back.
func (d *PlazaDirectory) characterHolder(characterID uint32, exclude uint64) *Session {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, plaza := range d.plazas {
		for _, member := range plaza.Members {
			if member.sessionID != 0 && member.sessionID != exclude &&
				member.session != nil && member.session.CharacterID() == characterID {
				return member.session
			}
		}
	}
	return nil
}

// plazaSnapshot captures the roster and card under the directory lock so no
// caller walks Members while another goroutine leaves.
type plazaSnapshot struct {
	instanceID  uint64
	masterSlot  int
	setting     protocol.SessionSetting
	sessions    [plazaCapacity]*Session
	initialized [plazaCapacity]bool
	positions   [plazaCapacity]*protocol.PlazaPosition
	characters  [plazaCapacity]store.Character
	items       [plazaCapacity][]store.Item
}

// broadcastTargets fills dst with the sessions that should receive a plaza
// packet: initialized members other than exclude. Sends happen after the lock is
// released, so the directory is never held while writing to sockets.
func (d *PlazaDirectory) broadcastTargets(plaza *Plaza, exclude uint64, dst *[plazaCapacity]*Session) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	written := 0
	for slot := range plaza.Members {
		member := plaza.Members[slot]
		if member.session == nil || member.session.ID == exclude || !member.initialized {
			continue
		}
		dst[written] = member.session
		written++
	}
	return written
}

func (d *PlazaDirectory) snapshot(plaza *Plaza) plazaSnapshot {
	d.mu.Lock()
	defer d.mu.Unlock()
	snapshot := plazaSnapshot{
		instanceID: plaza.InstanceID,
		masterSlot: plaza.MasterSlot,
		setting:    plaza.Setting(),
	}
	for slot, member := range plaza.Members {
		snapshot.sessions[slot] = member.session
		snapshot.initialized[slot] = member.initialized
		snapshot.positions[slot] = member.position
		snapshot.characters[slot] = member.character
		snapshot.items[slot] = member.items
	}
	return snapshot
}

// members builds the wire roster from a snapshot.
func (s *Server) plazaMembers(snapshot plazaSnapshot) []protocol.SessionMember {
	members := make([]protocol.SessionMember, plazaCapacity)
	for slot, session := range snapshot.sessions {
		if session == nil {
			continue
		}
		character := s.svc.WireCharacter(snapshot.characters[slot])
		members[slot] = protocol.SessionMember{
			Character: &character,
			Items:     s.svc.WireItems(snapshot.items[slot]),
			Team:      1,
		}
	}
	return members
}

// broadcastPlaza sends a packet to every initialized member except the sender.
// The target list lives on the stack: a plaza position packet is broadcast at
// movement rate, so copying a full snapshot (roster, characters, inventory) per
// packet would be pure garbage.
func (s *Server) broadcastPlaza(plaza *Plaza, packet protocol.Packet, exclude uint64) {
	var targets [plazaCapacity]*Session
	count := s.plazas.broadcastTargets(plaza, exclude, &targets)
	for i := 0; i < count; i++ {
		_ = targets[i].Send(packet)
	}
}

// enterPlaza places the session in the lobby and announces it.
func (s *Server) enterPlaza(session *Session) error {
	if session.Character.ID == 0 {
		return nil
	}
	// A session that is being torn down must not take a plaza seat: its socket is
	// already dead, so nothing would ever drop that seat again. Disconnect removes
	// the plaza entry before it unregisters the account, which leaves a window
	// where a concurrent caller still sees the session as live.
	if session.closing.Load() {
		s.log.Debug("[Plaza] refused a seat for a closing session", "session", session.ID)
		return nil
	}
	if !ChannelContains(session.channelType, session.channelIndex) {
		session.channelType = lobbyChannelType
		session.channelIndex = 0
		if err := session.Send(protocol.CreateMoveIntoReply(true, session.channelType, session.channelIndex)); err != nil {
			return err
		}
	}
	if holder := s.plazas.characterHolder(session.Character.ID, session.ID); holder != nil {
		// A seat whose session is gone must not lock the player out: the console, a
		// dying socket or an aborted entry can leave one behind.
		if s.accountOnline(holder) {
			return fmt.Errorf("character is already present on another connection")
		}
		s.log.Warn("[Plaza] dropping a seat whose session is gone",
			"character", session.Character.ID, "holder", holder.ID)
		s.dropPlazaSeat(holder.ID)
	}
	plaza, slot, err := s.plazas.enter(session, session.channelType, session.channelIndex)
	if err != nil {
		return err
	}
	// The seat exists from here on, so the flag must be set before anything can
	// fail: a failure below rolls the seat back, and a disconnect cleans it up.
	session.inPlaza = true
	snapshot := s.plazas.snapshot(plaza)
	members := s.plazaMembers(snapshot)
	entry, err := protocol.CreateRoomEntryReply(snapshot.setting, uint32(snapshot.masterSlot), uint32(slot), members)
	if err != nil {
		s.leavePlaza(session)
		return err
	}
	if err := session.Send(entry); err != nil {
		s.leavePlaza(session)
		return err
	}
	if slotReply, err := protocol.CreateRoomSlotReply(uint32(slot), members[slot]); err == nil {
		s.broadcastPlaza(plaza, slotReply, session.ID)
	}
	if setting, err := protocol.CreateRoomSettingReply(snapshot.setting); err == nil {
		s.broadcastPlaza(plaza, setting, session.ID)
	}
	return nil
}

// accountOnline reports whether the session is still the account's live one.
func (s *Server) accountOnline(session *Session) bool {
	if session == nil {
		return false
	}
	accountID := session.Account.ID
	if accountID == 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.online[accountID] == session
}

// leavePlaza removes the session and announces the departure.
func (s *Server) leavePlaza(session *Session) {
	session.inPlaza = false
	s.dropPlazaSeat(session.ID)
}

// dropPlazaSeat removes one plaza seat by session id and announces it. A session
// that is not seated is a no-op, so callers can use it without checking.
func (s *Server) dropPlazaSeat(sessionID uint64) {
	plaza, slot, masterChanged, ok := s.plazas.leave(sessionID)
	if !ok {
		return
	}
	departure, err := protocol.CreateRoomDepartureReply(uint32(slot))
	if err == nil {
		s.broadcastPlaza(plaza, departure, sessionID)
	}
	if masterChanged {
		if master, err := protocol.CreateRoomMasterReply(uint32(s.plazas.snapshot(plaza).masterSlot)); err == nil {
			s.broadcastPlaza(plaza, master, sessionID)
		}
	}
	snapshot := s.plazas.snapshot(plaza)
	if setting, err := protocol.CreateRoomSettingReply(snapshot.setting); err == nil {
		s.broadcastPlaza(plaza, setting, sessionID)
	}
}

// handlePlazaInitialize answers C521 (SendSessionInit).
func (s *Server) handlePlazaInitialize(session *Session) error {
	plaza, slot, changed := s.plazas.initialize(session.ID)
	if plaza == nil {
		reply, err := protocol.CreateSessionInitializationReply(session.Account.ID)
		if err != nil {
			return err
		}
		return session.Send(reply)
	}
	if !changed {
		return nil
	}
	// Send the existing members' positions to the newcomer, then announce the
	// newcomer to everybody already in the scene.
	snapshot := s.plazas.snapshot(plaza)
	type presence struct {
		accountID uint32
		position  protocol.PlazaPosition
	}
	var existing []presence
	for index, session := range snapshot.sessions {
		if index == slot || session == nil || !snapshot.initialized[index] ||
			snapshot.positions[index] == nil {
			continue
		}
		existing = append(existing, presence{
			accountID: session.Account.ID,
			position:  *snapshot.positions[index],
		})
	}
	own := snapshot.positions[slot]

	for _, entry := range existing {
		reply, err := protocol.CreatePlazaPositionReply(entry.accountID, entry.position)
		if err != nil {
			return err
		}
		if err := session.Send(reply); err != nil {
			return err
		}
	}
	if own != nil {
		if reply, err := protocol.CreatePlazaPositionReply(session.Account.ID, *own); err == nil {
			s.broadcastPlaza(plaza, reply, session.ID)
		}
	}
	initialized, err := protocol.CreateSessionInitializationReply(session.Account.ID)
	if err != nil {
		return err
	}
	s.broadcastPlaza(plaza, initialized, session.ID)
	return nil
}

// handlePlazaMove answers C511 (SendSessionChrPos).
func (s *Server) handlePlazaMove(session *Session, position protocol.PlazaPosition) error {
	plaza, _, ok := s.plazas.move(session.ID, position)
	if !ok {
		return nil
	}
	reply, err := protocol.CreatePlazaPositionReply(session.Account.ID, position)
	if err != nil {
		return err
	}
	s.broadcastPlaza(plaza, reply, session.ID)
	return nil
}

// plazaPingMembers builds the slot-to-session array of initialized members.
func (d *PlazaDirectory) plazaPingMembers(plaza *Plaza) []uint64 {
	members := make([]uint64, plazaCapacity)
	for slot, member := range plaza.Members {
		if member.sessionID != 0 && member.initialized {
			members[slot] = member.sessionID
		}
	}
	return members
}

// requestPing starts a probe and reports whether it may be broadcast.
func (d *PlazaDirectory) requestPing(sessionID uint64, slot uint32) (*Plaza, bool) {
	d.mu.Lock()
	plaza, memberSlot, ok := d.findLocked(sessionID)
	if !ok {
		d.mu.Unlock()
		return nil, false
	}
	// The original requires the requester to have finished its scene setup;
	// otherwise the probe would be broadcast by a member nobody can see yet.
	if !plaza.Members[memberSlot].initialized {
		d.mu.Unlock()
		return nil, false
	}
	members := d.plazaPingMembers(plaza)
	tracker, exists := d.pings[plaza.InstanceID]
	if !exists {
		tracker = NewPingTracker()
		d.pings[plaza.InstanceID] = tracker
	}
	d.mu.Unlock()
	return plaza, tracker.Begin(sessionID, slot, members, time.Now())
}

// completePing validates a responder report and returns the requester session.
func (d *PlazaDirectory) completePing(sessionID uint64, responderSlot, requesterSlot uint32) (uint64, bool) {
	d.mu.Lock()
	plaza, _, ok := d.findLocked(sessionID)
	if !ok {
		d.mu.Unlock()
		return 0, false
	}
	members := d.plazaPingMembers(plaza)
	tracker, exists := d.pings[plaza.InstanceID]
	d.mu.Unlock()
	if !exists {
		return 0, false
	}
	return tracker.Complete(sessionID, responderSlot, requesterSlot, members, time.Now())
}

// updatePlazaPresence refreshes a lobby character after equipment or level
// changes. Callers load the data first so no database call happens under the
// plaza lock.
func (s *Server) updatePlazaPresence(characterID uint32, items []store.Item, character store.Character) {
	s.plazas.mu.Lock()
	defer s.plazas.mu.Unlock()
	for _, plaza := range s.plazas.plazas {
		for _, member := range plaza.Members {
			if member.session == nil || member.session.CharacterID() != characterID {
				continue
			}
			member.items = items
			member.character = character
		}
	}
}

// plazaPingDeadline is unused by the simplified tracker but keeps the timing
// constant documented next to the implementation.
var plazaPingDeadline = 5 * time.Second
