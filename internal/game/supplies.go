package game

import (
	"fmt"
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
	// respawnBoxWindow is how long after a seat comes back its client's random-box
	// request counts as the respawn box. The 2015 client asks for it about five
	// seconds after the revive.
	respawnBoxWindow = 12 * time.Second
	// maxSupplyIntervalMs bounds a host-stated refresh interval (the reference
	// accepts up to one hour and states 30000 at most).
	maxSupplyIntervalMs = 3600000
)

// mapPointKinds is the set of kinds a map supply point may name. Every 2015 build
// loads ItemSpecItem for mode 3 (cn-717 sub_6920E0): booster, shield, ghost,
// needle, freezeCannon, medipack, camouflage, polymorph, banana, teamAngel,
// airBomb, turret, bazooka, machinegun and shieldField.
var mapPointKinds = map[uint32]bool{
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

// relayAlreadyForwards reports whether the relay itself already passes this id to
// the peers. It mirrors internal/net.reportForwardable's list; the game package
// cannot import the net package (the relay imports the game server), so an id
// added there has to be added here too.
func relayAlreadyForwards(id uint16) bool {
	if protocol.IsPeerAction(id) || protocol.IsPeerLife(id) ||
		protocol.IsPeerKick(id) || protocol.IsPeerExplosion(id) {
		return true
	}
	switch id {
	case 32898, 32901, 32924, // peer fire
		32773, 32777, 32900, 32906, 32920, // attacks, life points
		32884, 32885, 32922, // movement and watch target
		32925: // respawn complete
		return true
	}
	return false
}

// supplyCreateBodyBytes is the size of the ten-word box creation envelope.
const supplyCreateBodyBytes = 40

// clientBox is one box a client created and announced.
type clientBox struct {
	kind uint32
	// team is the colour word the creation envelope carried: 0 for any colour,
	// 1 for red and 2 for blue (kr-812 sub_50C840 classifies the point names, and
	// a box refuses a character of the other team in kr-812 sub_6378A0).
	team uint32
}

// suppliesColorName renders a colour word for the log.
func suppliesColorName(team uint32) string {
	switch team {
	case 1:
		return "red"
	case 2:
		return "blue"
	}
	return "any"
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
	// clientBoxes holds the boxes the clients created themselves, keyed by the id
	// they chose, with the kind and the colour word (0 any, 1 red, 2 blue) the
	// creation envelope stated.
	clientBoxes map[uint32]clientBox
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

// knownCarriedKind reports whether the kind appears in a sheet this server was told
// about. An unfamiliar kind is only logged: the client decides what a box contains
// and removes the box the moment it picks it, so refusing one loses the item (a
// real round showed the box disappear and the HUD stay empty).
func knownCarriedKind(kind uint32) bool {
	return mapPointKinds[kind] || immediateKinds[kind]
}

// immediateKinds are the kinds that apply the moment they are picked up instead of
// taking an item slot (the reference's immediate_supply_item_kinds).
var immediateKinds = map[uint32]bool{10: true, 11: true, 15: true, 16: true}

// mapPointKind reports whether a map point may name this kind. Kind 0 is a random
// box and is always allowed.
func mapPointKind(kind uint32) bool {
	return kind == 0 || mapPointKinds[kind]
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
		if !mapPointKind(entry.Kind) {
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

	// The kinds and colours decide what the boxes look like; keep them in the log so
	// a round can be checked against what the map is expected to place.
	kinds := make([]uint32, 0, len(points))
	teams := make([]uint32, 0, len(points))
	for _, point := range points {
		kinds = append(kinds, point.Kind)
		teams = append(teams, point.Team)
	}
	s.log.Info("[Supplies] map report", "room", room.ID, "points", len(points),
		"refused", refused, "intervals", report.IntervalsStated,
		"kinds", kinds, "teams", teams,
		"server_spawn", !s.cfg.ItemMode.DisableMapReportSpawns)
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
	// The round's state is created on demand: a client-driven round may deliver a
	// pickup before this server has seen any box announcement or map report, and
	// losing that pickup would leave the player unable to take anything all round.
	state := supplyStateLocked(room)
	// The slot may have been released by a concurrent leave; only the session that
	// still owns it may pick a box.
	if room.slots[seat].session != session {
		room.mu.Unlock()
		return nil
	}
	team := room.slots[seat].team
	if kind != 0 && !knownCarriedKind(kind) {
		if log := markSupplyRefusal(state); log {
			s.log.Warn("box carried a kind outside the known sheets; granted anyway",
				"room", room.ID, "slot", seat, "kind", kind)
		}
	}
	refusal := ""
	{
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
			} else if created, ok := state.clientBoxes[objectID]; ok && (created.kind == 0 || created.kind == kind) {
				found = true
				// The colour is bound to the team and the clients refuse the other
				// team's box themselves (kr-812 sub_6378A0). The server only makes it
				// authoritative when itemMode.enforceBoxColour is on: until the colour
				// word's numbering is confirmed on a real client, a wrong mapping would
				// stop a player from taking their own boxes.
				if created.team != 0 && created.team != uint32(team) {
					if s.cfg.ItemMode.EnforceBoxColour {
						found = false
						refusal = "a pickup of the other team's colour"
					} else {
						s.log.Warn("item mode: pickup of the other team's colour",
							"room", room.ID, "slot", seat, "box_team", suppliesColorName(created.team),
							"seat_team", suppliesColorName(uint32(team)))
					}
				}
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
		// A box this round never created or heard about. Granting it used to leave a
		// phantom item behind on a duplicate or stale request: the seat then could
		// not take anything else for the rest of the round.
		room.mu.Unlock()
		s.log.Info("item pickup of an unknown box ignored",
			"room", room.ID, "slot", seat, "object", objectID, "kind", kind)
		return nil
	}
	// An immediate item runs where it is picked up and never takes a slot.
	if immediateKinds[kind] {
		if target != nil {
			target.ActiveID = 0
		} else {
			delete(state.dropped, objectID)
			delete(state.clientBoxes, objectID)
		}
		room.mu.Unlock()
		room.Broadcast(protocol.CreateSupplyPicked(uint32(seat), objectID))
		s.log.Info("[Supplies] immediate item picked", "room", room.ID, "slot", seat,
			"kind", kind, "object", objectID)
		return nil
	}
	// The item goes into the first empty slot. The client only asks for a box it
	// believes it can take, so a pickup is never refused here: refusing one left the
	// seat unable to take anything else for the rest of the round whenever the two
	// sides disagreed about what was held. When this server thinks every slot is
	// taken, the oldest one is replaced and the desync is logged instead.
	held := state.held[seat]
	if held == nil {
		held = &heldItems{}
	}
	itemSlot = held.firstEmpty()
	if itemSlot < 0 {
		// The client asked for a box while this round believes both slots are taken.
		// Overwrite slot 0 and leave the other one alone: clearing both would throw
		// away an item the seat may really hold.
		if log := markSupplyRefusal(state); log {
			s.log.Warn("item slot state disagreed with the client; slot 0 overwritten",
				"room", room.ID, "slot", seat, "held", held.kinds)
		}
		itemSlot = 0
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
	// The client that picked the box may already have put the item in its own slot;
	// a second grant would leave a phantom item that blocks the slot and cannot be
	// used. itemMode.serverGrantsItem decides who owns the slot.
	if s.cfg.ItemMode.ServerGrantsItem {
		room.Broadcast(protocol.CreateSupplyGranted(uint32(seat), kind, uint32(itemSlot)))
		// One grant, no repeats.
		//
		// The client occasionally ignores a grant - the logs show it happening to the
		// second slot while the round's table already holds that item, with no death
		// and no use in between. Repeating the grant was tried and did not help: it
		// was sent again at 350ms and at 1.2s, both times guarded to the same slot and
		// kind, and the client still arrived at the next box believing the slot was
		// free. So the cause is a client-side acceptance condition, not a lost packet,
		// and sending more copies only risks a phantom item. If this is ever chased
		// further, look for the client state in which it accepts an S713 for slot 1
		// instead of repeating the packet.
	}
	s.log.Info("[Supplies] pickup", "room", room.ID, "slot", seat, "kind", kind,
		"item_slot", itemSlot, "object", objectID, "map_point", target != nil)
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
		s.log.Debug("item use carries a body this server does not read",
			"room", room.ID, "slot", session.slot, "bytes", len(packet.Payload))
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
		// The client asked to spend an item the round does not have for it. Either
		// the client owns the slot itself (see itemMode.serverGrantsItem) or the two
		// sides disagree about what that seat holds.
		s.log.Info("item use without a held item; nothing broadcast",
			"room", room.ID, "slot", seat, "grants_item", s.cfg.ItemMode.ServerGrantsItem)
		return nil
	}
	// The picker's own client applies the item from this packet: with it withheld a
	// whole round's items could not be used at all.
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
	// A kind-0 request is a random box. The client sends one on every knock-down,
	// which the 2015 game does not do, and one a few seconds after a respawn, which
	// it does: that is the box the player finds at the spawn point in its team's
	// colour. So a random box is only honoured inside the respawn window.
	boxTeam := uint32(0)
	if request.Kind == 0 {
		room.mu.RLock()
		back := room.respawnAt[session.slot]
		team := room.slots[session.slot].team
		room.mu.RUnlock()
		inside := !back.IsZero() && time.Since(back) <= respawnBoxWindow
		if !inside && !s.cfg.ItemMode.AcceptRandomBoxDrops {
			s.log.Debug("knock-down box request ignored", "room", room.ID, "slot", session.slot,
				"since_respawn_ms", time.Since(back).Milliseconds())
			return nil
		}
		boxTeam = uint32(team)
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
	if request.Kind != 0 && !knownCarriedKind(request.Kind) {
		if log := markSupplyRefusal(state); log {
			s.log.Warn("a thrown box carried a kind outside the known sheets; kept anyway",
				"room", room.ID, "kind", request.Kind)
		}
	}
	state.lastID++
	objectID := state.lastID
	state.dropped[objectID] = request.Kind
	// A thrown item leaves the hand: without this the round keeps believing the slot
	// is occupied, refuses every later pickup and the player cannot use what it does
	// not know about.
	thrownSlot := -1
	if request.Kind != 0 {
		if held := state.held[session.slot]; held != nil {
			if slot, ok := held.remove(request.Kind); ok {
				thrownSlot = slot
			}
		}
	}
	room.mu.Unlock()

	// The client states the class itself (10000 for an ordinary throw, 20000 for a
	// thrown box); it is passed through untouched.
	class := request.Class
	room.Broadcast(protocol.CreateSupplyCreate(objectID, class, protocol.SupplyReportPoint{
		X: request.X, Y: request.Y, Z: request.Z, Kind: request.Kind, Team: boxTeam,
	}))
	s.log.Info("[Supplies] box created", "room", room.ID, "slot", session.slot,
		"kind", request.Kind, "object", objectID, "class", class,
		"team", boxTeam, "thrown_slot", thrownSlot)
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
		s.log.Debug("item slot switch carries a body this server does not read",
			"room", room.ID, "slot", session.slot, "bytes", len(packet.Payload))
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

// noteRespawn records when a seat came back, so the box its client requests shortly
// afterwards can be told from a knock-down box.
func (s *Server) noteRespawn(room *Room, slot int) {
	if slot < 0 || slot >= maxRoomSlots {
		return
	}
	room.mu.Lock()
	room.respawnAt[slot] = time.Now()
	// The item bar starts empty after a respawn: the client shows nothing, so the
	// round keeps nothing either. Replaying the old grants was tried and the client
	// ignored them, which left the round holding items nobody could see and the
	// first use spending an invisible one. Clearing here keeps both sides equal and
	// lets the respawn box be taken straight away.
	if state := room.supplies; state != nil && state.round == room.tracking {
		if held := state.held[slot]; held != nil && held.kinds != [nativeItemSlots]uint32{} {
			s.log.Debug("item slots cleared on respawn", "room", room.ID, "slot", slot,
				"held", held.kinds)
			*held = heldItems{}
		}
	}
	room.mu.Unlock()
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
		clientBoxes: map[uint32]clientBox{},
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

// use spends the selected item and keeps the selection. When the selected slot is
// empty the other one is taken: the client keeps its own selection, and after a
// use it may ask for the item that is still in the other slot (that request used
// to be dropped, which looked like an item that could not be used).
func (h *heldItems) use() (kind uint32, slot int, ok bool) {
	if h.kinds[h.selected] == 0 {
		other := 1 - h.selected
		if h.kinds[other] == 0 {
			return 0, h.selected, false
		}
		h.selected = other
	}
	kind, slot = h.kinds[h.selected], h.selected
	h.kinds[h.selected] = 0
	return kind, slot, true
}

// remove clears the slot that holds the given kind and reports which one it was.
func (h *heldItems) remove(kind uint32) (int, bool) {
	for slot := range h.kinds {
		if h.kinds[slot] == kind {
			h.kinds[slot] = 0
			return slot, true
		}
	}
	return -1, false
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
	if s.cfg.ItemMode.DisableMapReportSpawns {
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
	// The map points carry a box when the round starts and then refresh on their
	// own interval, which is how the 2015 maps play.
	now := time.Now()
	for _, point := range state.points {
		point.NextAt = now
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

// handleItemPeerPacket takes one item packet a client reported, whichever
// connection it arrived on: the creation envelope is checked, a box is
// remembered by its own id and the packet is passed to the other members.
func (s *Server) handleItemPeerPacket(session *Session, packet protocol.Packet, viaRelay bool) error {
	room := session.room
	if room == nil || !seated(session) || !s.itemMode(room) {
		return nil
	}
	objectID, box, allowed := s.itemRelayAllowed(room, packet)
	if !allowed {
		return nil
	}
	if packet.ID == 32832 {
		s.noteClientBox(room, objectID, box)
	}
	s.forwardItemPacket(room, session.slot, packet, viaRelay)
	return nil
}

// forwardItemPacket relays one client-reported item packet to the other members
// over their relay connections when the round runs the item mode. The clients
// own these events; the server only keeps them inside the match.
func (s *Server) forwardItemPacket(room *Room, from int, packet protocol.Packet, viaRelay bool) {
	if !s.itemMode(room) {
		return
	}
	sent := 0
	for _, member := range room.View() {
		if !member.Occupied || member.Slot == from {
			continue
		}
		// The client dispatches these ids per connection, so the packet has to reach
		// the peers on the same transport the sender used: a game-connection report
		// that came back over the relay is parsed by a different handler (or not at
		// all), which is how an item effect ends up visible only to the player who
		// used it.
		if viaRelay && s.relay != nil && s.relay.SendToSlot(room.ID, uint8(member.Slot), packet) {
			sent++
			continue
		}
		if member.Session.Send(packet) == nil {
			sent++
		}
	}
	s.log.Debug("[Supplies] item packet forwarded", "room", room.ID, "id", packet.ID,
		"bytes", len(packet.Payload), "from", from, "to", sent, "relay", viaRelay)
}

// itemRelayAllowed checks one client-reported item packet before it reaches the
// other members. The effect packets are the clients' own business; only a box
// creation states something the server can police, so only that one is decoded,
// and a body it cannot decode is still forwarded: a layout this server misreads
// must not cost the mode its boxes, and the item battle is client-authoritative
// either way.
func (s *Server) itemRelayAllowed(room *Room, packet protocol.Packet) (objectID uint32, box clientBox, allowed bool) {
	if packet.ID != 32832 { // gppCreateMapItem
		return 0, clientBox{}, true
	}
	if len(packet.Payload) != supplyCreateBodyBytes {
		// The same opcode also carries the client's smaller map objects: a thrown
		// banana arrives as three words (object, kind, owner) and is nobody's box
		// envelope. Forward it untouched and do not track it as a box.
		preview := packet.Payload
		if len(preview) > 32 {
			preview = preview[:32]
		}
		s.log.Debug("client map object is not a box envelope; forwarding it unread",
			"room", room.ID, "bytes", len(packet.Payload), "hex", fmt.Sprintf("%x", preview))
		return 0, clientBox{}, true
	}
	objectID, _, point, err := protocol.ReadSupplyCreate(packet, 0)
	if err != nil {
		s.log.Warn("client box could not be decoded; forwarding it unread",
			"room", room.ID, "error", err)
		return 0, clientBox{}, true
	}
	if !mapPointKind(point.Kind) {
		s.log.Debug("client box refused: kind not in the item sheet",
			"room", room.ID, "kind", point.Kind)
		return 0, clientBox{}, false
	}
	if point.Team > 2 {
		s.log.Debug("client box refused: team outside 0..2", "room", room.ID, "team", point.Team)
		return 0, clientBox{}, false
	}
	return objectID, clientBox{kind: point.Kind, team: point.Team}, true
}

// noteClientBox remembers a box a client announced over the relay, so a later
// pickup of that id can still be granted and shown to the other members.
func (s *Server) noteClientBox(room *Room, objectID uint32, box clientBox) {
	if objectID == 0 {
		return
	}
	room.mu.Lock()
	state := supplyStateLocked(room)
	if len(state.clientBoxes) < maxClientBoxes {
		state.clientBoxes[objectID] = box
	}
	room.mu.Unlock()
}

// isItemRelayPacket reports whether a relay packet belongs to the item family.
// The 2015 round feed does not route any of these, so an item-mode round is the
// only place where they may reach another client.
func isItemRelayPacket(id uint16) bool {
	switch id {
	// Boxes.
	case 32832: // gppCreateMapItem: a client placed a box
		return true
	// Item use and its direct effects.
	case 32824, // gppItemUseResult: a seat used an item
		32888, // gppSyncUseItemSimply
		32894, // gppUseItemDirect
		32929: // gppUseItemDirectAtPos
		return true
	// Item hits and the status an item puts on a character. Each of these carries
	// the other side of an effect: without them the victim never learns it was
	// frozen, slid on a banana, was magnetised or turned invisible, and an effect
	// that starts can never end.
	case 32829, // gppHitByItem
		32830, // gppEndMagnet
		32831, // gppEndFreezeCannon
		32833, // gppStepOnBanana
		32882, // gppGhost (invisibility)
		32818, // gppStickGrenadeTo
		32889, // gppCharactersPlayData (an item that changes a character's look)
		32812, // gppResetChrAttr (the reset that ends such a change)
		32895: // gppChangeTurretState (a placed turret's state)
		return true
	// Area effects (air strike and its cleanup).
	case 32899, // gppStormDamager
		32909, // gppStormDamagerItemId
		32914, // gppEndDarkCircle
		32915: // gppDeleteStormDamagerAOE
		return true
	}
	return false
}
