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
	// (1000000 + 100000 per seat) and leaves 0x63000000 to the NPC-kill Lucci
	// items the reference reserves for it.
	supplyObjectIDBase = 0x62000000

	// maxSupplyPoints bounds one host map report.
	maxSupplyPoints = 256
	// maxDroppedSupplies bounds the thrown boxes one round keeps, matching the
	// reference's max_dropped_supplies.
	maxDroppedSupplies = 512
	// maxClientBoxes bounds the boxes the clients created themselves that the
	// round still tracks.
	maxClientBoxes = 512
	// nativeItemSlots is how many items a seat may hold. The 2015 builds hold two
	// (the reference's native_item_slot_count), and the client switches between
	// them with C744.
	nativeItemSlots = 2
	// noItemKind is the kind word that means "this slot is empty" on the wire.
	noItemKind = 0xFFFFFFFF
	// supplyPoll is the box refresh loop's resolution.
	supplyPoll = 250 * time.Millisecond
	// defaultSupplyMinMs and defaultSupplyMaxMs are the interval the original
	// host states for a point that names none (kr-812 sub_50C840 appends 30000
	// and 15000).
	defaultSupplyMinMs = 15000
	defaultSupplyMaxMs = 30000
	// ordinaryCreationClass is the word an ordinary map box carries.
	ordinaryCreationClass = 10000
	// maxSupplyIntervalMs bounds a host-stated refresh interval (the reference
	// accepts up to one hour and states 30000 at most).
	maxSupplyIntervalMs = 3600000
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
	// held is the per-seat item slots; a seat holds at most nativeItemSlots.
	held    map[int]*heldItems
	running bool
	// refusal is set once a refused box is logged, so a client that keeps asking
	// cannot flood the log.
	refusal bool
}

// itemMode reports whether the room runs the item battle.
func (s *Server) itemMode(room *Room) bool {
	return room.SettingCopy().Game.Mode == itemModeID
}

// suppliesPlaying reports whether the room is in play, which is when the item
// battle accepts pickups, uses, slot switches and thrown boxes. The map report is
// the only one accepted earlier, because the host sends it while the map loads.
func suppliesPlaying(room *Room) bool {
	room.mu.RLock()
	defer room.mu.RUnlock()
	return room.Started && room.phase == phasePlaying
}

// suppliesReportable reports whether a round is loading or running, the two
// states the reference accepts a map report in.
func suppliesReportable(room *Room) bool {
	room.mu.RLock()
	defer room.mu.RUnlock()
	if !room.Started {
		return false
	}
	switch room.phase {
	case phaseLoading, phaseCountdown, phasePlaying:
		return true
	}
	return false
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
	if !suppliesReportable(room) {
		s.log.Debug("map supply report ignored: the round is not loading or running",
			"room", room.ID, "slot", session.slot)
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
	// A report that arrives while the round already runs and already has points is
	// a repeat; one that arrives during loading is the same round's first report,
	// because the round token only advances when the countdown starts.
	if existing := room.supplies; existing != nil && len(existing.points) > 0 &&
		(existing.round == room.tracking || room.Started) {
		room.mu.Unlock()
		s.log.Debug("map supply report ignored: already prepared", "room", room.ID)
		return nil
	}
	state := supplyStateLocked(room)
	state.points = points
	playing := room.Started && room.phase == phasePlaying
	room.mu.Unlock()

	s.log.Info("[Supplies] map report", "room", room.ID, "points", len(points),
		"refused", refused, "intervals", report.IntervalsStated,
		"server_spawn", s.cfg.ItemMode.SpawnFromMapReport)
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
	if room == nil || !seated(session) || !s.itemMode(room) || !suppliesPlaying(room) {
		return nil
	}
	objectID, kind, err := protocol.ReadSupplyPickup(packet)
	if err != nil {
		s.log.Warn("item pickup rejected", "room", room.ID, "slot", session.slot, "error", err)
		return nil
	}
	seat := session.slot

	var target *supplyPoint
	found := false
	itemSlot := -1
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
	refusal := ""
	switch {
	case !itemModeAccepts(kind):
		refusal = "a pickup of an unoffered kind"
	default:
		for _, point := range state.points {
			if point.ActiveID == 0 || point.ActiveID != objectID {
				continue
			}
			if point.Kind != 0 && point.Kind != kind {
				continue
			}
			if point.Team != 0 && point.Team != uint32(team) {
				refusal = "a pickup of another team's box"
				break
			}
			target, found = point, true
			break
		}
		if !found && refusal == "" {
			// A thrown box or a box the clients created carries its own kind.
			if thrown, ok := state.dropped[objectID]; ok && (thrown == 0 || thrown == kind) {
				found = true
			} else if created, ok := state.clientBoxes[objectID]; ok && (created == 0 || created == kind) {
				found = true
			}
		}
	}
	if refusal != "" {
		log := markSupplyRefusal(state)
		room.mu.Unlock()
		if log {
			s.log.Warn("item mode refused a box; further refusals this round are not logged",
				"room", room.ID, "reason", refusal)
		}
		return nil
	}
	if !found {
		// The 2015 clients create the map boxes themselves, so a box this server
		// never saw announced is still a real one: accept the pickup and let every
		// member remove it. The item sheet check above still applies, the two item
		// slots bound how much a client can take and the refusal below needs a known
		// team box, which only a reported map point can be.
		s.log.Debug("item pickup of a box the round did not see created",
			"room", room.ID, "slot", seat, "object", objectID, "kind", kind)
		found = true
	}
	// The item goes into the first empty slot. A seat that already holds every
	// slot cannot take the box, and the box stays where it is for somebody else.
	held := state.held[seat]
	if held == nil {
		held = &heldItems{}
	}
	if itemSlot = held.firstEmpty(); itemSlot < 0 {
		room.mu.Unlock()
		s.log.Debug("item pickup ignored: every item slot is taken",
			"room", room.ID, "slot", seat, "object", objectID)
		return nil
	}
	// Commit: the box leaves the map and the item enters the slot.
	if target != nil {
		target.ActiveID = 0
	} else {
		delete(state.dropped, objectID)
		delete(state.clientBoxes, objectID)
	}
	state.held[seat] = held
	held.add(itemSlot, kind)
	room.mu.Unlock()

	room.Broadcast(protocol.CreateSupplyPicked(uint32(seat), objectID))
	room.Broadcast(protocol.CreateSupplyGranted(uint32(seat), kind, uint32(itemSlot)))
	s.log.Info("[Supplies] pickup", "room", room.ID, "slot", seat, "kind", kind,
		"item_slot", itemSlot, "object", objectID, "tracked", target != nil)
	return nil
}

// handleUseItem answers the held-item use (C708, empty body) by telling every
// member which item the seat spent.
func (s *Server) handleUseItem(session *Session, packet protocol.Packet) error {
	room := session.room
	if room == nil || !seated(session) || !s.itemMode(room) || !suppliesPlaying(room) {
		return nil
	}
	if len(packet.Payload) != 0 {
		return nil
	}
	seat := session.slot
	kind := uint32(0)
	itemSlot := 0
	found := false
	room.mu.Lock()
	if state := room.supplies; state != nil && state.round == room.tracking &&
		room.slots[seat].session == session {
		if held := state.held[seat]; held != nil {
			kind, itemSlot, found = held.use()
		}
	}
	room.mu.Unlock()
	if !found {
		s.log.Debug("item use ignored: no held item in the selected slot", "room", room.ID, "slot", seat)
		return nil
	}
	room.Broadcast(protocol.CreateSupplyUsed(uint32(seat), kind, uint32(itemSlot)))
	s.log.Info("[Supplies] item used", "room", room.ID, "slot", seat, "kind", kind, "item_slot", itemSlot)
	return nil
}

// handleCreateItem accepts a thrown box (C728) and answers every member with the
// creation envelope the client's box handler expects.
func (s *Server) handleCreateItem(session *Session, packet protocol.Packet) error {
	room := session.room
	if room == nil || !seated(session) || !s.itemMode(room) || !suppliesPlaying(room) {
		return nil
	}
	request, err := protocol.ReadSupplyDrop(packet)
	if err != nil {
		s.log.Warn("dropped item rejected", "room", room.ID, "slot", session.slot, "error", err)
		return nil
	}
	room.mu.Lock()
	state := supplyStateLocked(room)
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

	// The client states the class itself (10000 for an ordinary throw, 20000 for a
	// thrown box); it is passed through untouched.
	class := request.Class
	room.Broadcast(protocol.CreateSupplyCreate(objectID, class, protocol.SupplyReportPoint{
		X: request.X, Y: request.Y, Z: request.Z, Kind: request.Kind, Team: 0,
	}))
	s.log.Info("[Supplies] box dropped", "room", room.ID, "slot", session.slot,
		"kind", request.Kind, "object", objectID, "class", class)
	return nil
}

// handleSwitchItemSlot answers a slot switch (C744). The 2015 builds hold two
// native slots: a switch to the occupied one is accepted and told to every
// member, and a switch that has nothing to select is answered to the requester
// alone with a nonzero result, which leaves its selection unchanged.
func (s *Server) handleSwitchItemSlot(session *Session, packet protocol.Packet) error {
	room := session.room
	if room == nil || !seated(session) || !s.itemMode(room) || !suppliesPlaying(room) {
		return nil
	}
	if len(packet.Payload) != 0 {
		return nil
	}
	seat := session.slot
	kind := uint32(noItemKind)
	itemSlot := 0
	switched := false
	room.mu.Lock()
	if state := room.supplies; state != nil && state.round == room.tracking &&
		room.slots[seat].session == session {
		if held := state.held[seat]; held != nil {
			switched = held.trySwitch()
			itemSlot = held.selected
			kind = held.selectedKind()
		}
	}
	room.mu.Unlock()
	changed := protocol.CreateSupplySlotChanged(uint32(seat), uint32(itemSlot), kind, 0)
	if !switched {
		// Nothing to switch to: only the requester is told, and it keeps its own
		// selection (a nonzero result leaves it unchanged).
		return session.Send(protocol.CreateSupplySlotChanged(uint32(seat), uint32(itemSlot), kind, 1))
	}
	room.Broadcast(changed)
	s.log.Info("[Supplies] item slot switched", "room", room.ID, "slot", seat,
		"item_slot", itemSlot, "kind", kind)
	return nil
}

// supplyStateLocked returns the round's item-mode state, creating it when there
// is none. The caller holds the room mutex. A state whose round token is behind
// the room's belongs to the round that is starting now, so it is adopted instead
// of replaced: replacing it would drop the map points the host already reported
// or the boxes the clients already announced.
func supplyStateLocked(room *Room) *supplyRound {
	if state := room.supplies; state != nil && state.round <= room.tracking {
		state.round = room.tracking
		return state
	}
	state := &supplyRound{
		round:       room.tracking,
		lastID:      supplyObjectIDBase,
		dropped:     map[uint32]uint32{},
		clientBoxes: map[uint32]uint32{},
		held:        map[int]*heldItems{},
	}
	room.supplies = state
	return state
}

// heldItems is one seat's native item slots. The 2015 client keeps two: a
// pickup fills the first empty one, use spends the selected one and C744 switches
// the selection to the other, exactly like the reference's HeldSupplyItems.
type heldItems struct {
	kinds    [nativeItemSlots]uint32
	selected int
}

// firstEmpty returns the first slot without an item, or -1 when every slot is
// taken.
func (h *heldItems) firstEmpty() int {
	for slot := range h.kinds {
		if h.kinds[slot] == 0 {
			return slot
		}
	}
	return -1
}

// add stores one item and selects its slot when the previous selection was
// empty.
func (h *heldItems) add(slot int, kind uint32) {
	if slot < 0 || slot >= len(h.kinds) {
		return
	}
	if h.kinds[h.selected] == 0 {
		h.selected = slot
	}
	h.kinds[slot] = kind
}

// selectedKind returns the kind in the selected slot, or noItemKind.
func (h *heldItems) selectedKind() uint32 {
	if kind := h.kinds[h.selected]; kind != 0 {
		return kind
	}
	return noItemKind
}

// use spends the selected item and keeps the selection.
func (h *heldItems) use() (kind uint32, slot int, ok bool) {
	if h.kinds[h.selected] == 0 {
		return 0, h.selected, false
	}
	kind, slot = h.kinds[h.selected], h.selected
	h.kinds[h.selected] = 0
	return kind, slot, true
}

// trySwitch selects the other slot when it holds an item.
func (h *heldItems) trySwitch() bool {
	other := 1 - h.selected
	if h.kinds[other] == 0 {
		return false
	}
	h.selected = other
	return true
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
	if !s.cfg.ItemMode.SpawnFromMapReport {
		return
	}
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
		// Same transport it arrived on, with the game connection as the fallback
		// for a member whose relay never joined.
		if s.relay.SendToSlot(room.ID, uint8(member.Slot), packet) {
			continue
		}
		_ = member.Session.Send(packet)
	}
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
	state := supplyStateLocked(room)
	if len(state.clientBoxes) < maxClientBoxes {
		state.clientBoxes[objectID] = kind
	}
	room.mu.Unlock()
}

// isItemRelayPacket reports whether a relay packet belongs to the item family.
// The 2015 round feed does not route any of these, so an item-mode round is the
// only place where they may reach another client.
func isItemRelayPacket(id uint16) bool {
	switch id {
	case 32832, // gppCreateMapItem: the host scheduled a box
		32824, // gppItemUseResult: a seat used an item
		32829, // gppHitByItem: an item hit a character
		32888, // gppSyncUseItemSimply
		32894, // gppUseItemDirect
		32909, // gppStormDamagerItemId: an item's area effect
		32929: // gppUseItemDirectAtPos
		return true
	}
	return false
}
