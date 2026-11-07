package game

import (
	"bytes"
	"crypto/rand"
	"fmt"
	mathrand "math/rand"
	"net"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/store"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/world"
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
// maxRoomSlots is the fixed roster size of a Bubble Fighter room: one player per
// object partition, so it is the protocol's own slot count.
const maxRoomSlots = protocol.PlayerSlots

type Room struct {
	mu sync.RWMutex

	ID           uint16
	Setting      protocol.SessionSetting
	Master       int
	Started      bool
	ChannelType  uint32
	ChannelIndex uint32

	slots [maxRoomSlots]roomSlot
	// password is the text the creator sent in C402, kept out of Setting so it is
	// never written into a lobby row or a room snapshot. Empty when the room is
	// open; a locked room with no text accepts any password (the wire allows it).
	password     []uint16
	life         *BattleLife
	pings        *PingTracker
	defenseSeeds map[int]defenseSeeds
	// supplies is the item-mode state of the running round, nil in other modes.
	supplies *supplyRound

	phase             int
	loadingTimer      *time.Timer
	defenseConfirmed  [maxRoomSlots]bool
	cancelConfirmed   [maxRoomSlots]bool
	characterReported [maxRoomSlots]bool
	seedsSent         bool
	published         [maxRoomSlots]protocol.BattleScore
	teamScores        [2]uint16
	// respawnCount rotates the team spawn markers the way the reference does.
	// It is reset at round start.
	respawnCount [maxRoomSlots]uint32
	// lastSpawn remembers the random marker an individual-round seat took last,
	// so the next respawn never lands on the same point.
	lastSpawn [maxRoomSlots]uint32
	// spawnUsedAt keeps a marker out of circulation for a moment after it is handed
	// out. Without it two seats the same blast killed come back on one point: three
	// of them took marker 26 within six seconds of each other on the first build.
	spawnUsedAt [29]time.Time
	// respawnAt records when each seat last came back, so the box the client
	// places a few seconds after a respawn can be told from a knock-down box.
	respawnAt [maxRoomSlots]time.Time

	tracking int
	// generation invalidates timers left over from a previous round.
	generation   int
	seedFallback *time.Timer
	startAt      time.Time
	endTimer     *time.Timer

	// The zombie battle's round state; every field is inert outside mode 8.
	// zombieKingSlot is -1 until a seat turns, zombieWinner is 0 while the round
	// is undecided and otherwise holds the winning team id (1 humans, 14 zombies).
	zombieKingSlot  int
	zombieKingTimer *time.Timer
	zombieWinner    uint8
	// forfeitWinner is the team that wins because the other side left the round:
	// 0 unless a mid-round departure left only one side seated. finishRound reads
	// it once, in place of a score comparison.
	forfeitWinner uint8
	// zombiePendingNotice marks the seats whose conversion has not been announced
	// yet: the notice waits for their respawn completion.
	zombiePendingNotice [maxRoomSlots]bool
}

// SetPassword stores the room password.
func (r *Room) SetPassword(password []uint16) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.password = append(r.password[:0], password...)
}

// PasswordCopy returns the stored room password.
func (r *Room) PasswordCopy() []uint16 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]uint16(nil), r.password...)
}

// View returns an immutable snapshot of the roster.
func (r *Room) View() []memberView {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]memberView, 0, maxRoomSlots)
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

	// game_results.end_reason: why the round stopped. The admin console renders the
	// column, and the CHECK constraint on the table keeps the pair as the only
	// legal values.
	endReasonScore = 1
	endReasonTime  = 2
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
	// Balance by count like the reference's ChooseTeam: the smaller team takes the
	// newcomer, so a four player room ends up 1,2,1,2 instead of 1,2,2,2.
	first, second := 0, 0
	for _, entry := range r.slots {
		if entry.session == nil {
			continue
		}
		if entry.team == 1 {
			first++
		} else if entry.team == 2 {
			second++
		}
	}
	team := uint8(1)
	if first > second {
		team = 2
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
	members := make([]uint64, maxRoomSlots)
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

// NextSpawnIndex returns the spawn marker for the next respawn of one slot and
// advances that slot's counter.
//
// The reference formula is team*10 + (slot/2 + respawn_count) % team_spawn_count
// + 1, which keeps teammates apart only because retail seats them on 0/2 and
// 1/3. This server also allows 0/1 and 2/3, where slot/2 is identical for both
// teammates and they would share a marker. The ordinal inside the team is used
// instead, so the first member of a team takes marker 1, the second marker 2,
// and each respawn advances by one.
// spawnHold keeps a marker out of circulation right after it is handed out, long
// enough for the seat that took it to walk away.
const spawnHold = 6 * time.Second

func (r *Room) NextSpawnIndex(slot int, teamSpawnCount uint32) uint32 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if slot < 0 || slot >= maxRoomSlots || teamSpawnCount == 0 {
		return 0
	}
	// The individual hot-blood battle drops every seat at a random point of the
	// map and never on the marker it used last: the retail mode moves the respawn
	// point each time. Both battle groups are ordinary points there, not a side,
	// and the client takes whatever marker it is given, so the spread is this
	// server's job.
	if r.Setting.Game.Mode == singlePvpModeID {
		span := teamSpawnCount
		if span > 8 {
			span = 8
		}
		// Only the first block is used. The second one (21-28) names the same
		// places under different numbers -- a seat brought back on 26 stood exactly
		// on 18 -- so mixing the two is what put two players on one spot. The first
		// block's eight points are distinct, which covers a full room.
		now := time.Now()
		free := make([]uint32, 0, span)
		var oldest uint32
		var oldestAt time.Time
		for within := uint32(1); within <= span; within++ {
			marker := 10 + within
			if marker == r.lastSpawn[slot] {
				continue
			}
			held := r.spawnUsedAt[marker]
			if !now.Before(held) {
				free = append(free, marker)
				continue
			}
			// Every point is held: the one whose hold ends first answers.
			if oldest == 0 || held.Before(oldestAt) {
				oldest, oldestAt = marker, held
			}
		}
		index := oldest
		if len(free) > 0 {
			index = free[mathrand.Intn(len(free))]
		}
		if index == 0 {
			index = 11
		}
		r.lastSpawn[slot] = index
		if int(index) < len(r.spawnUsedAt) {
			r.spawnUsedAt[index] = now.Add(spawnHold)
		}
		return index
	}
	team := r.slots[slot].team
	if team == 0 {
		return 0
	}
	// Each teammate owns a disjoint slice of the team's markers and rotates
	// inside it. The reference advances by one marker per respawn for the whole
	// team, so two teammates walk the same ring one step apart and eventually
	// respawn onto each other; with a private slice that cannot happen.
	ordinal, teamSize := uint32(0), uint32(0)
	for other := 0; other < maxRoomSlots; other++ {
		if r.slots[other].session == nil || r.slots[other].team != team {
			continue
		}
		teamSize++
		if other < slot {
			ordinal++
		}
	}
	if teamSize == 0 {
		return 0
	}
	stride := teamSpawnCount / teamSize
	if stride == 0 {
		stride = 1
	}
	base := ordinal * stride
	// The rotation stays inside the private slice: base is added after the
	// modulo, otherwise it would wrap back onto the first teammate's markers.
	index := uint32(team)*10 + base + r.respawnCount[slot]%stride + 1
	r.respawnCount[slot]++
	return index
}

// Life returns the life tracker.
func (r *Room) Life() *BattleLife {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.life
}

// masterNickname returns the name the lobby row shows, taken from the seat that
// currently owns the room.
func (r *Room) masterNickname() []uint16 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	master := r.Master
	if master < 0 || master >= maxRoomSlots {
		return nil
	}
	seated := r.slots[master].session
	if seated == nil {
		return nil
	}
	return protocol.U16String(seated.Character.Nickname)
}

// refreshSetting recomputes the derived room-card counters.
func (r *Room) refreshSetting() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Setting.MemberCount = uint16(r.occupiedLocked())
	if r.Setting.Capacity == 0 {
		r.Setting.Capacity = uint16(maxRoomSlots)
	}
}

// Members builds the wire roster from a snapshot.
func (s *Server) members(room *Room) []protocol.SessionMember {
	members := make([]protocol.SessionMember, maxRoomSlots)
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

// The equipment slot the rifles, shotguns and sniper rifles share; the item id's
// own segment says which kind it is.
const primaryWeaponSlot = 18

// starterSniperItemID is the practice sniper rifle, which every character owns.
const starterSniperItemID = 204001

// sniperItem reports whether an item id is a sniper rifle. The whole 204 segment is
// sniper rifles -- eighteen items, every one of them named so -- and no other
// segment carries one.
func sniperItem(itemID uint32) bool { return itemID/1000 == 204 }

// sniperLoadout returns the item list a seat brings into a sniper battle. The round
// offers nothing but the sniper rifle, and the slot reply is what tells the client
// which weapon a seat plays, so a seat that equipped something else is handed the
// practice rifle instead. The previous item id comes back for the log; zero means
// the seat already had a sniper.
func sniperLoadout(items []protocol.InvItem) ([]protocol.InvItem, uint32) {
	for index := range items {
		if items[index].EquippedSlot != primaryWeaponSlot {
			continue
		}
		if sniperItem(items[index].ItemID) {
			return items, 0
		}
		previous := items[index].ItemID
		out := make([]protocol.InvItem, len(items))
		copy(out, items)
		out[index] = protocol.InvItem{ItemID: starterSniperItemID, Quantity: 1, EquippedSlot: primaryWeaponSlot}
		return out, previous
	}
	// No primary weapon at all: the round still hands one out.
	out := make([]protocol.InvItem, len(items), len(items)+1)
	copy(out, items)
	return append(out, protocol.InvItem{ItemID: starterSniperItemID, Quantity: 1, EquippedSlot: primaryWeaponSlot}), 0
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
		card := room.SettingCopy()
		// The row's string after the lock flag is the master's name (cn-717
		// sub_6D3770 reads +12, +48 and the map name into the row's three columns),
		// so the list states it even after the room changed hands.
		card.MasterNickname = room.masterNickname()
		writer := protocol.NewWriter()
		if err := protocol.WriteSessionSetting(writer, card); err != nil {
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
		s.log.Info("[Room] creation rejected", "reason", "not in lobby",
			"character", session.Character.ID, "inRoom", session.room != nil, "inPlaza", session.inPlaza)
		return session.Send(protocol.CreateRoomResult(true, 1))
	}
	// The stored text is the room's only lock state, which is how Fantasy models it:
	// the lobby row's has_password is derived from `Password.Length > 0`
	// (RoomFlow.cs:243) and a joiner is refused only when the text is non-empty
	// (RoomFlow.cs:294). So a create that sets the flag but sends no text makes an
	// open room, not a locked door that everything walks through.
	if len(request.Setting.Title) > 64 || len(request.Setting.MasterNickname) > 64 {
		s.log.Info("[Room] creation rejected", "reason", "title",
			"title", len(request.Setting.Title), "locked", request.Setting.PasswordRequired)
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
	setting.Capacity = uint16(maxRoomSlots)
	// The password arrived outside the setting (C402 carries it after the last
	// field), so the client's own +46 flag is only a hint; the text decides.
	setting.PasswordRequired = len(request.Password) > 0
	// The lobby row shows the master's name in the string after the lock flag.
	setting.MasterNickname = protocol.U16String(session.Character.Nickname)

	s.mu.Lock()
	if len(s.rooms) >= 1024 {
		s.mu.Unlock()
		return session.Send(protocol.CreateRoomResult(true, 1))
	}
	room := &Room{
		ID:             s.nextRoom,
		Setting:        setting,
		Master:         0,
		ChannelType:    session.channelType,
		ChannelIndex:   session.channelIndex,
		phase:          phaseOpen,
		zombieKingSlot: -1,
	}
	room.SetPassword(request.Password)
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
	// The text decides, as in Fantasy RoomFlow.cs:294: a joiner is refused only
	// when the room has a non-empty password and this request's copy differs.
	// An empty room password means an open room.
	if stored := room.PasswordCopy(); len(stored) != 0 && !slices.Equal(stored, request.Password) {
		s.log.Debug("[Room] join refused, wrong password", "room", room.ID, "slot", session.slot,
			"password_chars", len(request.Password))
		return session.Send(protocol.CreateRoomResult(false, 4))
	}
	capacity := int(setting.Capacity)
	if capacity <= 0 || capacity > maxRoomSlots {
		capacity = maxRoomSlots
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
	// One departure at a time per session: the admin console, the player's own
	// request and a dying socket can all arrive together, and only the first may
	// run the sequence below.
	session.transitionMu.Lock()
	defer session.transitionMu.Unlock()
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
	// The seat leaves the round's life table too: its object must stop answering
	// events, or the round that keeps running could still score hits on it.
	if life := room.Life(); life != nil {
		life.Drop(uint8(slot))
	}
	// The departing member's relay transport goes with him; whatever happens to the
	// round afterwards is decided below.
	if s.relay != nil {
		s.relay.RevokeSlot(room.ID, uint8(slot))
	}
	// A member vanishing mid-match no longer aborts the round for everyone: the
	// seat is vacated and the rest keep playing, the way Fantasy's LeaveCoreAsync
	// sends RoomSeatVacated and returns. The zombie battle still lets a departure
	// decide its round when it can: the king's exit wins for the humans and the
	// last human's leaves it to the zombies.
	if started && !s.zombieLeave(room, slot) {
		s.roundAfterDeparture(room)
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

	// The leaver's own copy can fail — its socket is already gone in the disconnect
	// and relay paths — and that must not cost the room its departure notice: the
	// round goes on, so every other client has to learn that this seat is empty.
	if err := session.Send(departure); err != nil {
		s.log.Debug("departure reply failed", "session", session.ID, "error", err)
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
	if packet.ID != 515 {
		return fmt.Errorf("%w: expected RoomChangeTeam", protocol.ErrInvalidPacket)
	}
	room := session.room
	if room == nil {
		return nil
	}
	// In the zombie battle the same request is the client's death report
	// (kr-812 sub_76C090 sends gcpRoomChangeTeam on every death of the local
	// player), so a running round handles it instead of the waiting room's team
	// switch. Its body is not read: a build may send the seat and team words
	// there, and the sender's own seat is the only one that may turn.
	if room.StartedReport() && room.ZombieRound() {
		s.zombieDeathReport(session)
		return nil
	}
	if len(packet.Payload) != 0 {
		return fmt.Errorf("%w: expected empty RoomChangeTeam", protocol.ErrInvalidPacket)
	}
	if room.SettingCopy().State != 2 {
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
	// A team holds at most half of the room.
	if count >= maxRoomSlots/2 {
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

// roundMap resolves the map one round runs. A room whose master never chose a map
// draws a fresh one on every round, and the marker in the fourth setting string is
// what says so: the second string only holds the name the round before pinned for
// the client's relay report, so it cannot tell a choice from a leftover.
func roundMap(maps world.Maps, setting protocol.SessionSetting) string {
	if strings.HasPrefix(protocol.FromU16(setting.Game.Strings16[3]), "Random") {
		if candidates := maps.Candidates(setting.Game.Mode, protocol.FromU16(setting.Game.Strings16[2])); len(candidates) > 0 {
			return candidates[mathrand.Intn(len(candidates))].ResourceName
		}
		return maps.DefaultRulesMap
	}
	if name := protocol.FromU16(setting.Game.Strings16[1]); name != "" && name != "Random" {
		if _, ok := maps.Find(name); ok {
			return name
		}
	}
	return maps.DefaultRulesMap
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
		// The zombie battle starts every seat on the human team, and the individual
		// hot-blood battle has no teams at all, so neither is ever balanced.
		if settingNow.Game.Mode != zombieModeID && settingNow.Game.Mode != singlePvpModeID {
			return session.Send(protocol.CreateGameEntryReply(protocol.StartUnequal, nil, 2))
		}
	}

	mapName := roundMap(s.maps, settingNow)
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
	// The zombie battle puts every seat on the human team before the life tracker
	// binds its teams and before the start's seat records are built, so both carry
	// team 1; the notices themselves go out with the start.
	zombieNotices := room.beginZombieRound()
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
	room.defenseConfirmed = [maxRoomSlots]bool{}
	room.cancelConfirmed = [maxRoomSlots]bool{}
	room.characterReported = [maxRoomSlots]bool{}
	room.seedsSent = false
	room.respawnCount = [maxRoomSlots]uint32{}
	room.lastSpawn = [maxRoomSlots]uint32{}
	room.spawnUsedAt = [29]time.Time{}
	room.zombiePendingNotice = [maxRoomSlots]bool{}
	// A fresh round never inherits a decision from the previous one.
	room.zombieWinner = 0
	room.forfeitWinner = 0
	// A fresh round starts with no item-mode state; the host reports its map
	// points after the map loads.
	room.supplies = nil
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
	for _, notice := range zombieNotices {
		room.Broadcast(notice)
	}
	members := s.members(room)
	// The sniper battles offer no other weapon, and the slot reply is what tells a
	// client which weapon a seat plays. Substituting it here keeps the change inside
	// the round: nothing is written to the account, and the next round rebuilds the
	// list from the session's own inventory.
	if settingNow.Game.Mode == sniperModeID || settingNow.Game.Mode == sniperTeamSurvivalModeID {
		for slot := range members {
			if members[slot].Character == nil {
				continue
			}
			loadout, previous := sniperLoadout(members[slot].Items)
			members[slot].Items = loadout
			if previous != 0 {
				s.log.Debug("[Sniper] loadout forced", "room", room.ID, "slot", slot,
					"from", previous, "to", starterSniperItemID)
			}
		}
	}
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
		s.log.Warn("loading timed out", "room", room.ID)
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
	first, second := 0, 0
	for slot := range room.slots {
		if room.slots[slot].session == nil {
			continue
		}
		if room.slots[slot].team == 2 {
			second++
		} else {
			first++
		}
	}
	room.mu.Unlock()

	s.log.Info("[Round] started", "room", room.ID, "team1", first, "team2", second,
		"durationMs", duration)

	room.Broadcast(protocol.CreateGameStartReply(duration, uint32(tracking)))
	// The zombie battle's king is picked once the opening passed; the round's
	// generation makes a pick from an earlier round impossible.
	room.mu.RLock()
	generation := room.generation
	room.mu.RUnlock()
	s.scheduleZombieOpening(room, generation)

	// The item mode's box refresh starts with play and stops with the round.
	room.mu.Lock()
	supplies := room.supplies
	room.mu.Unlock()
	if supplies != nil {
		s.startSupplies(room, supplies)
	}

	room.mu.Lock()
	if room.endTimer != nil {
		room.endTimer.Stop()
	}
	room.endTimer = time.AfterFunc(time.Duration(duration)*time.Millisecond, func() {
		s.finishRound(room, true)
	})
	room.mu.Unlock()
}

// roundAfterDeparture keeps a round that is already being played alive after one
// seat vanished. A round that has not reached phasePlaying is dropped instead:
// its clients are still loading, so restarting costs nothing and a missing seat
// would strand the players on the ready screen.
func (s *Server) roundAfterDeparture(room *Room) {
	room.mu.Lock()
	phase := room.phase
	mode := room.Setting.Game.Mode
	remaining := room.occupiedLocked()
	// Count the seats each battle team still has: a round whose opponent side is
	// empty cannot be played to a result, however many players it still shows.
	seats := map[uint8]int{}
	for slot := range room.slots {
		if room.slots[slot].session != nil {
			seats[room.slots[slot].team]++
		}
	}
	room.mu.Unlock()
	if remaining == 0 || phase != phasePlaying {
		s.cancelRound(room)
		return
	}
	s.log.Info("[Round] a seat left mid-round", "room", room.ID, "remaining", remaining)
	// The zombie battle has no team 2 (humans are 1, zombies 14) and decides its
	// own round through zombieLeave, so the two-team test below must not read its
	// empty team 2 as a missing side.
	if mode == zombieModeID {
		return
	}
	// The individual battle has no sides to balance, so the two-team test cannot
	// decide it: one seat leaving must not hand the round to whatever team it sat
	// on. The last seat standing simply ends the round the way it always ends for
	// this mode -- with nobody left to play against, it would otherwise sit until
	// the clock ran out.
	if mode == singlePvpModeID {
		if remaining > 1 {
			return
		}
		s.finishRound(room, false)
		return
	}
	winner := uint8(0)
	switch {
	case seats[1] == 0 && seats[2] > 0:
		winner = 2
	case seats[2] == 0 && seats[1] > 0:
		winner = 1
	}
	if winner != 0 {
		// One side left the two-team round. Settle now and credit the side that
		// stayed, instead of running the clock down with nothing to fight.
		room.mu.Lock()
		room.forfeitWinner = winner
		room.mu.Unlock()
		s.finishRound(room, false)
	}
}

// cancelRound returns a started room to the open state and notifies everyone,
// matching GameRoundCoordinator::Cancel (S736 + S501 + S503).
func (s *Server) cancelRound(room *Room) {
	caller := "unknown"
	if _, file, line, ok := runtime.Caller(1); ok {
		caller = fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}
	s.log.Info("[Round] cancelled", "room", room.ID, "caller", caller,
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
	if room.zombieKingTimer != nil {
		room.zombieKingTimer.Stop()
		room.zombieKingTimer = nil
	}
	room.zombieKingSlot = -1
	room.zombieWinner = 0
	room.forfeitWinner = 0
	room.zombiePendingNotice = [maxRoomSlots]bool{}
	room.Started = false
	room.phase = phaseOpen
	room.Setting.State = phaseOpen
	// A room whose map was never chosen shows the random entry again: the pinned
	// name only existed for the round that just ended, and a room that did choose
	// one keeps it.
	if strings.HasPrefix(protocol.FromU16(room.Setting.Game.Strings16[3]), "Random") {
		room.Setting.Game.Strings16[1] = nil
	}
	room.life = nil
	room.pings = nil
	room.supplies = nil
	room.defenseSeeds = map[int]defenseSeeds{}
	room.defenseConfirmed = [maxRoomSlots]bool{}
	room.cancelConfirmed = [maxRoomSlots]bool{}
	room.characterReported = [maxRoomSlots]bool{}
	room.seedsSent = false
	for slot := range room.slots {
		room.slots[slot].ready = false
		room.slots[slot].loaded = false
		// The zombie battle's seats go back to the human team: the waiting room's
		// seat record only carries the two battle teams.
		if room.slots[slot].team == zombieTeam {
			room.slots[slot].team = zombieHumanTeam
		}
	}
	members := make([]memberView, 0, maxRoomSlots)
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
	if room.zombieKingTimer != nil {
		room.zombieKingTimer.Stop()
		room.zombieKingTimer = nil
	}
	teamScores := room.teamScores
	decided := room.zombieWinner
	if decided == 0 {
		decided = room.forfeitWinner
	}
	room.forfeitWinner = 0
	life := room.life
	room.life = nil
	room.supplies = nil
	room.published = [maxRoomSlots]protocol.BattleScore{}
	room.teamScores = [2]uint16{}
	setting := room.Setting
	members := make([]memberView, 0, maxRoomSlots)
	for slot := range room.slots {
		entry := room.slots[slot]
		members = append(members, memberView{
			Slot: slot, Session: entry.session, Team: entry.team,
			Ready: entry.ready, Active: entry.active, Loaded: entry.loaded,
			Occupied:  entry.session != nil,
			Character: entry.character, Items: entry.items,
		})
	}
	// The round's teams go back to the human team for the records that follow the
	// settlement: a waiting-room seat record only carries the two battle teams.
	// A conversion whose notice is still waiting for its respawn bar is announced
	// now: the client picks the mode's result banner from the seat teams it knows,
	// and a seat that already turned on the server would otherwise still look like a
	// surviving human.
	var flush []int
	if setting.Game.Mode == zombieModeID {
		for slot := range room.slots {
			if room.zombiePendingNotice[slot] {
				room.zombiePendingNotice[slot] = false
				flush = append(flush, slot)
			}
			if room.slots[slot].team == zombieTeam {
				room.slots[slot].team = zombieHumanTeam
			}
		}
	}
	room.mu.Unlock()
	for _, slot := range flush {
		if notice := zombieNotice(slot); notice != nil {
			s.log.Debug("[Zombie] conversion announced at the round end", "room", room.ID, "slot", slot)
			room.Broadcast(*notice)
		}
	}

	// Ask every member for its damage trace before settling. Diagnostics only:
	// the original never blocks settlement on the answer.
	for _, member := range members {
		if member.Occupied {
			_ = member.Session.Send(protocol.CreateHealthTraceRequest())
		}
	}
	winner := uint8(0)
	if decided != 0 {
		// The zombie battle is decided by its own rule, not by the team totals:
		// the winner is the human team (1) or the zombie team (14).
		winner = decided
	} else if setting.Game.Mode == zombieModeID && timeExpired {
		// A zombie round that runs its 180 s out goes to the humans; the round has
		// no team totals to compare.
		winner = zombieHumanTeam
	} else if teamScores[0] > teamScores[1] {
		winner = 1
	} else if teamScores[1] > teamScores[0] {
		winner = 2
	}
	reason := "score"
	if timeExpired {
		reason = "time"
	}
	if setting.Game.Mode == zombieModeID {
		// The mode's result banner is chosen by the client, so the round records what
		// it decided and how the seats stood when it decided it.
		humans, zombies := 0, 0
		for _, member := range members {
			if !member.Occupied {
				continue
			}
			if member.Team == zombieTeam {
				zombies++
			} else {
				humans++
			}
		}
		s.log.Info("[Zombie] result decided", "room", room.ID, "winner", winner,
			"decided", decided, "time_expired", timeExpired, "humans", humans, "zombies", zombies,
			"pending_notice", room.zombiePendingNotice)
	}
	s.log.Info("[Round] ended", "room", room.ID, "winner", winner,
		"first", teamScores[0], "second", teamScores[1], "reason", reason)

	// The wire's result rows keep the teams the round used (the zombie battle's
	// 14 included); the store maps them onto its two-team schema when it writes.
	players := make([]store.ResultPlayer, 0, maxRoomSlots)
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
		RoomID:     room.ID,
		TrackingID: uint32(roomIDTracking(room)),
		Mode:       setting.Game.Mode,
		SubMode:    setting.Game.SubMode,
		MapName:    protocol.FromU16(setting.Game.Strings16[1]),
		// 1 = the scores decided it, 2 = the clock ran out. The console shows this
		// column, so it has to be the round's own reason instead of a constant.
		EndReason:     endReasonScore,
		FirstScore:    teamScores[0],
		SecondScore:   teamScores[1],
		Winner:        winner,
		PolicyVersion: s.prog.Version,
		Players:       players,
	}
	if timeExpired {
		result.EndReason = endReasonTime
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
	// The result screen must report exactly what settlement stored, otherwise a
	// character at the level cap is shown more experience than the database keeps.
	maxExperience := s.prog.MaxExperience()
	for _, player := range players {
		before := uint32(0)
		for _, member := range members {
			if member.Occupied && uint8(member.Slot) == player.Slot {
				before = member.Character.Experience
			}
		}
		// Same helper settlement uses, so the screen and the database cannot drift.
		after, appliedGain := store.ClampGain(before, player.ExperienceGain, maxExperience)
		// Same rule for the wallet: report what settlement actually stored.
		lucciBefore := uint32(0)
		for _, member := range members {
			if member.Occupied && uint8(member.Slot) == player.Slot {
				lucciBefore = member.Character.Lucci
			}
		}
		lucciAfter, appliedLucci := store.ClampGain(lucciBefore, player.LucciGain, store.MaxLucci)
		if appliedGain != player.ExperienceGain || appliedLucci != player.LucciGain {
			s.log.Info("[Round] settlement clamped", "room", room.ID, "slot", player.Slot,
				"experienceBefore", before, "experienceAfter", after,
				"experienceGain", appliedGain, "lucciBefore", lucciBefore,
				"lucciAfter", lucciAfter, "lucciGain", appliedLucci)
		}
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
			ExperienceGain: appliedGain,
			LucciGain:      appliedLucci,
			LevelUp:        level > levelBefore,
			AwardScore:     award.Score,
			AwardGrade:     award.Grade,
			AwardMedal:     award.Medal,
		})
	}
	// S729 must reach every client before S705: the S705 handler reads the
	// frozen ScoreData snapshot.
	if resultReply, err := protocol.CreateGameResultReply(teamScores, winner, rows); err == nil {
		room.Broadcast(resultReply)
	} else {
		// Without this packet the client has no result screen at all, so it must
		// not disappear into the debug stream.
		s.log.Warn("game result packet rejected", "room", room.ID, "error", err)
	}
	if setting.Game.Mode == zombieModeID {
		// The zombie battle decides one winner for every seat, which the team
		// totals cannot express: every member gets its own outcome word (3 win,
		// 2 loss) instead of the round-end notice.
		for _, member := range members {
			if !member.Occupied || member.Session == nil {
				continue
			}
			_ = member.Session.Send(protocol.CreateGameOutcomeReply(member.Team == decided))
		}
	} else {
		room.Broadcast(protocol.CreateGameEndReply(timeExpired))
	}

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
						s.log.Warn("character reload failed", "session", member.Session.ID, "error", err)
						continue
					}
					member.Session.Character = fresh
					room.SetProfile(member.Slot, fresh, member.Items)
					if err := s.sendUserStatFor(member.Session, fresh, member.Items); err != nil {
						s.log.Warn("user stat refresh failed", "session", member.Session.ID, "error", err)
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
