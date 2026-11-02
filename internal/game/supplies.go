package game

import (
	"math/rand"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
)

// Item mode (client mode 3).
//
// The 2015 client keeps the rules of the mode itself: the boxes it sees on the
// map, their kinds, the pickup rules and the effects all live in the client's
// own ModeMgr (kr-812 sub_53AB30 builds a plain one for mode 3). The reference
// server therefore only owns two things: the box kinds the build's item sheet
// offers, and which team owns a box. This file implements that, plus the box
// refresh loop the host's map report drives, so every member sees the same
// boxes.
const (
	// itemModeID is the client's item battle (wgtItem).
	itemModeID = 3

	// supplyObjectIDBase keeps server-created boxes clear of character objects
	// (1000000 + 100000 per seat) and of the star range (0x64000000).
	supplyObjectIDBase = 0x63000000

	// maxSupplyPoints bounds one host map report.
	maxSupplyPoints = 256
	// maxDroppedSupplies bounds the thrown boxes one round keeps.
	maxDroppedSupplies = 256
	// maxClientBoxes bounds the boxes the clients created themselves that the
	// round still tracks.
	maxClientBoxes = 512
	// supplyPoll is the box refresh loop's resolution.
	supplyPoll = 250 * time.Millisecond
	// defaultSupplyMinMs and defaultSupplyMaxMs are the interval the original
	// host states for a point that names none (kr-812 sub_50C840 appends 30000
	// and 15000).
	defaultSupplyMinMs = 15000
	defaultSupplyMaxMs = 30000
	// ordinaryCreationClass is the word an ordinary map box carries.
	ordinaryCreationClass = 10000
	// maxSupplyIntervalMs bounds a host-stated refresh interval; the reference
	// host states 30000 at most.
	maxSupplyIntervalMs = 600000
)

// itemModeKinds is the set of box kinds the cn-717 item sheet offers. Every
// 2015 build loads ItemSpecItem for mode 3 (cn-717 sub_6920E0): booster,
// shield, ghost, needle, freezeCannon, medipack, camouflage, polymorph, banana,
// teamAngel, airBomb, turret, bazooka, machinegun and shieldField.
var itemModeKinds = map[uint32]bool{
	101: true, // booster
	102: true, // shield
	108: true, // ghost
	109: true, // needle
	115: true, // freezeCannon
	120: true, // medipack
	123: true, // camouflage
	127: true, // polymorph
	128: true, // banana
	130: true, // teamAngel
	136: true, // airBomb
	137: true, // turret
	138: true, // bazooka
	139: true, // machinegun
	141: true, // shieldField
}

// supplyPoint is one map supply point of the running round.
type supplyPoint struct {
	protocol.SupplyReportPoint
	ActiveID uint32
	NextAt   time.Time
}

// supplyRound is the item-mode state of one round. Every field is guarded by
// the room mutex.
type supplyRound struct {
	// round is the room's round token; a state from an older round is dead.
	round   int
	lastID  uint32
	points  []*supplyPoint
	dropped map[uint32]uint32
	// clientBoxes holds the boxes the clients created over the relay, keyed by
	// the id they chose, so a pickup of one still reaches every member.
	clientBoxes map[uint32]uint32
	held        map[int]uint32
	running     bool
	// refusal is set once a refused box is logged, so a client that keeps asking
	// cannot flood the log.
	refusal bool
}

// ItemMode reports whether the room runs the item battle.
func (s *Server) itemMode(room *Room) bool {
	return room.SettingCopy().Game.Mode == itemModeID
}

// seated reports whether a session owns a room slot, so the slot index can be
// used to read room.slots and to key the round's per-seat state.
func seated(session *Session) bool {
	return session.slot >= 0 && session.slot < maxRoomSlots
}

// itemModeAccepts reports whether the build's item mode offers a box kind. Kind
// 0 is a random box and is always offered.
func itemModeAccepts(kind uint32) bool {
	return kind == 0 || itemModeKinds[kind]
}

// handleMapReport accepts the host's map supply report (C702) and starts the box
// refresh loop. The body is tolerated: a host whose report does not match the
// audited layout must still be able to play, so a parse failure is logged and
// dropped instead of failing the connection.
func (s *Server) handleMapReport(session *Session, packet protocol.Packet) error {
	room := session.room
	if room == nil || !seated(session) || !s.itemMode(room) {
		return nil
	}
	report, err := protocol.ReadSupplyReport(packet, maxSupplyPoints)
	if err != nil {
		s.log.Warn("map supply report rejected", "room", room.ID, "slot", session.slot, "error", err)
		return nil
	}
	if session.slot != room.MasterSlot() {
		s.log.Debug("map supply report ignored: not the room master", "room", room.ID, "slot", session.slot)
		return nil
	}
	points := make([]*supplyPoint, 0, len(report.Points))
	refused := 0
	for _, entry := range report.Points {
		if !itemModeAccepts(entry.Kind) {
			refused++
			continue
		}
		if entry.Team > 2 {
			refused++
			continue
		}
		if entry.MinMs == 0 || entry.MaxMs < entry.MinMs || entry.MaxMs > maxSupplyIntervalMs {
			entry.MinMs, entry.MaxMs = defaultSupplyMinMs, defaultSupplyMaxMs
		}
		points = append(points, &supplyPoint{SupplyReportPoint: entry})
	}
	room.mu.Lock()
	round := room.tracking
	// A report that arrives while the round already runs and already has points is
	// a repeat; one that arrives during loading is the same round's first report,
	// because the round token only advances when the countdown starts.
	if existing := room.supplies; existing != nil && len(existing.points) > 0 &&
		(existing.round == round || room.Started) {
		room.mu.Unlock()
		s.log.Debug("map supply report ignored: already prepared", "room", room.ID)
		return nil
	}
	state := &supplyRound{
		round:       round,
		lastID:      supplyObjectIDBase,
		points:      points,
		dropped:     map[uint32]uint32{},
		clientBoxes: map[uint32]uint32{},
		held:        map[int]uint32{},
	}
	room.supplies = state
	playing := room.Started && room.phase == phasePlaying
	room.mu.Unlock()

	s.log.Info("[Supplies] map report", "room", room.ID, "points", len(points),
		"refused", refused, "intervals", report.IntervalsStated)
	if playing {
		s.startSupplies(room, state)
	}
	return nil
}

// handleEatItem accepts a box pickup (C707): the box must be a live map point of
// the running round, its kind must be one the build offers and a team box may
// only be eaten by that team.
func (s *Server) handleEatItem(session *Session, packet protocol.Packet) error {
	room := session.room
	if room == nil || !seated(session) || !s.itemMode(room) {
		return nil
	}
	objectID, kind, err := protocol.ReadSupplyPickup(packet)
	if err != nil {
		s.log.Warn("item pickup rejected", "room", room.ID, "slot", session.slot, "error", err)
		return nil
	}
	seat := session.slot

	var pickedKind uint32
	known := false
	room.mu.Lock()
	state := room.supplies
	if state == nil || state.round != room.tracking {
		room.mu.Unlock()
		return nil
	}
	// The slot may have been released by a concurrent leave; only the session that
	// still owns it may pick a box.
	if room.slots[seat].session != session {
		room.mu.Unlock()
		return nil
	}
	team := room.slots[seat].team
	if !itemModeAccepts(kind) {
		log := markSupplyRefusal(state)
		room.mu.Unlock()
		if log {
			s.log.Warn("item mode refused a box; further refusals this round are not logged",
				"room", room.ID, "reason", "a pickup of an unoffered kind")
		}
		return nil
	}
	for _, point := range state.points {
		if point.ActiveID == 0 || point.ActiveID != objectID {
			continue
		}
		if point.Kind != 0 && point.Kind != kind {
			continue
		}
		if point.Team != 0 && point.Team != uint32(team) {
			log := markSupplyRefusal(state)
			room.mu.Unlock()
			if log {
				s.log.Warn("item mode refused a box; further refusals this round are not logged",
					"room", room.ID, "reason", "a pickup of another team's box")
			}
			return nil
		}
		point.ActiveID = 0
		pickedKind, known = kind, true
		break
	}
	if !known {
		// A thrown box carries the kind it was thrown with.
		if thrown, ok := state.dropped[objectID]; ok && (thrown == 0 || thrown == kind) {
			delete(state.dropped, objectID)
			pickedKind, known = kind, true
		}
	}
	if !known {
		// A box the clients created themselves; the round tracked its id so the
		// pickup still reaches every member.
		if created, ok := state.clientBoxes[objectID]; ok && (created == 0 || created == kind) {
			delete(state.clientBoxes, objectID)
			pickedKind, known = kind, true
		}
	}
	if known {
		if previous, ok := state.held[seat]; ok && previous != pickedKind {
			s.log.Debug("held item replaced", "room", room.ID, "slot", seat,
				"was", previous, "now", pickedKind)
		}
		state.held[seat] = pickedKind
	}
	room.mu.Unlock()

	if !known {
		// Not a box this server created: the clients own it and report the
		// pickup between themselves.
		s.log.Debug("item pickup ignored: unknown box", "room", room.ID, "slot", seat, "object", objectID)
		return nil
	}
	room.Broadcast(protocol.CreateSupplyPicked(uint32(seat), objectID))
	room.Broadcast(protocol.CreateSupplyGranted(uint32(seat), pickedKind, 0))
	s.log.Debug("[Supplies] pickup", "room", room.ID, "slot", seat, "kind", pickedKind, "object", objectID)
	return nil
}

// handleUseItem answers the held-item use (C708, empty body) by telling every
// member which item the seat spent.
func (s *Server) handleUseItem(session *Session, packet protocol.Packet) error {
	room := session.room
	if room == nil || !seated(session) || !s.itemMode(room) {
		return nil
	}
	if len(packet.Payload) != 0 {
		return nil
	}
	seat := session.slot
	kind := uint32(0)
	found := false
	room.mu.Lock()
	if state := room.supplies; state != nil && state.round == room.tracking {
		kind, found = state.held[seat]
		delete(state.held, seat)
	}
	room.mu.Unlock()
	if !found {
		s.log.Debug("item use ignored: no held item", "room", room.ID, "slot", seat)
		return nil
	}
	room.Broadcast(protocol.CreateSupplyUsed(uint32(seat), kind, 0))
	return nil
}

// handleCreateItem accepts a thrown box (C728) and answers every member with the
// creation envelope the client's box handler expects.
func (s *Server) handleCreateItem(session *Session, packet protocol.Packet) error {
	room := session.room
	if room == nil || !seated(session) || !s.itemMode(room) {
		return nil
	}
	request, err := protocol.ReadSupplyDrop(packet)
	if err != nil {
		s.log.Warn("dropped item rejected", "room", room.ID, "slot", session.slot, "error", err)
		return nil
	}
	room.mu.Lock()
	state := room.supplies
	if state == nil || state.round != room.tracking {
		state = &supplyRound{round: room.tracking, lastID: supplyObjectIDBase,
			dropped: map[uint32]uint32{}, clientBoxes: map[uint32]uint32{},
			held: map[int]uint32{}}
		room.supplies = state
	}
	if len(state.dropped) >= maxDroppedSupplies {
		log := markSupplyRefusal(state)
		room.mu.Unlock()
		if log {
			s.log.Warn("item mode refused a box; further refusals this round are not logged",
				"room", room.ID, "reason", "more thrown boxes than the round keeps")
		}
		return nil
	}
	if !itemModeAccepts(request.Kind) {
		log := markSupplyRefusal(state)
		room.mu.Unlock()
		if log {
			s.log.Warn("item mode refused a box; further refusals this round are not logged",
				"room", room.ID, "reason", "a thrown box of an unoffered kind")
		}
		return nil
	}
	state.lastID++
	objectID := state.lastID
	state.dropped[objectID] = request.Kind
	room.mu.Unlock()

	class := request.Class
	if class == 0 {
		class = ordinaryCreationClass
	}
	room.Broadcast(protocol.CreateSupplyCreate(objectID, class, protocol.SupplyReportPoint{
		X: request.X, Y: request.Y, Z: request.Z, Kind: request.Kind, Team: 0,
	}))
	return nil
}

// handleSwitchItemSlot answers a slot switch (C744). The 2015 builds hold a
// single native slot, so the selection never changes; a nonzero result tells the
// client to keep its own.
func (s *Server) handleSwitchItemSlot(session *Session, packet protocol.Packet) error {
	room := session.room
	if room == nil || !seated(session) || !s.itemMode(room) {
		return nil
	}
	if len(packet.Payload) != 0 {
		return nil
	}
	kind := uint32(0)
	room.mu.Lock()
	if state := room.supplies; state != nil && state.round == room.tracking {
		kind = state.held[session.slot]
	}
	room.mu.Unlock()
	return session.Send(protocol.CreateSupplySlotChanged(uint32(session.slot), 0, kind, 1))
}

// markSupplyRefusal flags the first refused box of a round. The caller holds the
// room mutex and logs when it returns true.
func markSupplyRefusal(state *supplyRound) bool {
	if state.refusal {
		return false
	}
	state.refusal = true
	return true
}

// startSupplies begins the box refresh loop once play starts.
func (s *Server) startSupplies(room *Room, state *supplyRound) {
	room.mu.Lock()
	if !supplyStartsLocked(room, state) {
		room.mu.Unlock()
		return
	}
	state.running = true
	// Adopt the round token: a report that arrived during loading still belongs to
	// the round that is starting now.
	state.round = room.tracking
	now := time.Now()
	for _, point := range state.points {
		point.NextAt = now.Add(supplyInterval(point.SupplyReportPoint))
	}
	room.mu.Unlock()
	go s.runSupplies(room, state)
}

// supplyStartsLocked reports whether the refresh loop may start. The caller
// holds the room mutex.
//
// The state's round token may be one behind: the host reports the map points
// while the round is still loading, and the token only advances when the
// countdown starts. A state that is further behind, or that no longer belongs to
// the room, is stale.
func supplyStartsLocked(room *Room, state *supplyRound) bool {
	return !state.running && len(state.points) > 0 && room.supplies == state &&
		state.round <= room.tracking && room.Started && room.phase == phasePlaying
}

// supplyInterval picks the next refresh delay of one point.
func supplyInterval(point protocol.SupplyReportPoint) time.Duration {
	minimum, maximum := int(point.MinMs), int(point.MaxMs)
	if minimum <= 0 {
		minimum = defaultSupplyMinMs
	}
	if maximum < minimum {
		maximum = defaultSupplyMaxMs
	}
	if maximum <= minimum {
		return time.Duration(minimum) * time.Millisecond
	}
	return time.Duration(minimum+rand.Intn(maximum-minimum+1)) * time.Millisecond
}

// runSupplies creates one box per due point and stops with the round.
func (s *Server) runSupplies(room *Room, state *supplyRound) {
	ticker := time.NewTicker(supplyPoll)
	defer ticker.Stop()
	for range ticker.C {
		room.mu.Lock()
		if room.supplies != state || state.round != room.tracking ||
			!room.Started || room.phase != phasePlaying {
			state.running = false
			room.mu.Unlock()
			return
		}
		now := time.Now()
		var created []protocol.Packet
		var targets []*Session
		for _, point := range state.points {
			if now.Before(point.NextAt) {
				continue
			}
			point.NextAt = now.Add(supplyInterval(point.SupplyReportPoint))
			if point.ActiveID != 0 {
				continue
			}
			state.lastID++
			point.ActiveID = state.lastID
			created = append(created, protocol.CreateSupplyCreate(state.lastID,
				ordinaryCreationClass, point.SupplyReportPoint))
		}
		if len(created) > 0 {
			for slot := range room.slots {
				if entry := room.slots[slot]; entry.session != nil {
					targets = append(targets, entry.session)
				}
			}
		}
		room.mu.Unlock()
		for _, packet := range created {
			for _, target := range targets {
				_ = target.Send(packet)
			}
		}
	}
}

// forwardItemPacket relays one client-reported item packet to the other members
// over their relay connections when the round runs the item mode. The clients
// own these events; the server only keeps them inside the match.
func (s *Server) forwardItemPacket(room *Room, from int, packet protocol.Packet) {
	if s.relay == nil || !s.itemMode(room) {
		return
	}
	for _, member := range room.View() {
		if !member.Occupied || member.Slot == from {
			continue
		}
		s.relay.SendToSlot(room.ID, uint8(member.Slot), packet)
	}
}

// serverOwnsBoxes reports whether the running round creates the map boxes from
// the host's map report.
func (s *Server) serverOwnsBoxes(room *Room) bool {
	room.mu.RLock()
	defer room.mu.RUnlock()
	return room.supplies != nil && len(room.supplies.points) > 0
}

// itemRelayAllowed checks one client-reported item packet before it reaches the
// other members. The effect packets are the clients' own business; only a box
// creation states something the server can police, so only that one is decoded,
// and a body it cannot decode is still forwarded: a layout this server misreads
// must not cost the mode its boxes, and the item battle is client-authoritative
// either way.
func (s *Server) itemRelayAllowed(room *Room, packet protocol.Packet) (objectID, kind uint32, allowed bool) {
	if packet.ID != 32832 { // gppCreateMapItem
		return 0, 0, true
	}
	objectID, _, point, err := protocol.ReadSupplyCreate(packet, 0)
	if err != nil {
		s.log.Warn("client box could not be decoded; forwarding it unread",
			"room", room.ID, "error", err)
		return 0, 0, true
	}
	if !itemModeAccepts(point.Kind) {
		s.log.Debug("client box refused: kind not in the item sheet",
			"room", room.ID, "kind", point.Kind)
		return 0, 0, false
	}
	if point.Team > 2 {
		s.log.Debug("client box refused: team outside 0..2", "room", room.ID, "team", point.Team)
		return 0, 0, false
	}
	return objectID, point.Kind, true
}

// noteClientBox remembers a box a client announced over the relay, so a later
// pickup of that id can still be granted and shown to the other members.
func (s *Server) noteClientBox(room *Room, objectID, kind uint32) {
	if objectID == 0 {
		return
	}
	room.mu.Lock()
	state := room.supplies
	if state == nil || state.round != room.tracking {
		state = &supplyRound{round: room.tracking, lastID: supplyObjectIDBase,
			dropped: map[uint32]uint32{}, clientBoxes: map[uint32]uint32{},
			held: map[int]uint32{}}
		room.supplies = state
	}
	if len(state.clientBoxes) < maxClientBoxes {
		state.clientBoxes[objectID] = kind
	}
	room.mu.Unlock()
}

// isItemRelayPacket reports whether a relay packet belongs to the item family.
func isItemRelayPacket(id uint16) bool {
	switch id {
	case 32832, // gppCreateMapItem: the host scheduled a box
		32824, // gppItemUseResult: a seat used an item
		32888, // gppSyncUseItemSimply
		32894, // gppUseItemDirect
		32929: // gppUseItemDirectAtPos
		return true
	}
	return false
}
