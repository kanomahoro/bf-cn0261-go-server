package game

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/world"
)

// testMaps is a two-map catalog: one that runs both modes and one that runs the
// normal mode only.
func testMaps() world.Maps {
	return world.Maps{
		DefaultRulesMap: "rel_Test_Normal",
		Maps: []world.Map{
			{ResourceName: "rel_Test_Item", Theme: "Village", NormalMode: true, ItemMode: true,
				TimeLimitMilliseconds: 240000, ScoreLimit: 20, TeamSpawnCount: 8},
			{ResourceName: "rel_Test_Normal", Theme: "Village", NormalMode: true, ItemMode: false,
				TimeLimitMilliseconds: 240000, ScoreLimit: 20, TeamSpawnCount: 8},
		},
	}
}

func itemRequest(mapName string) protocol.GameSetting {
	var request protocol.GameSetting
	request.Mode = itemModeID
	request.Strings16[0] = protocol.U16String("items")
	request.Strings16[1] = protocol.U16String(mapName)
	request.Strings16[2] = protocol.U16String("Village")
	request.Strings16[3] = protocol.U16String(mapName)
	return request
}

func TestNormalizeGameSettingAcceptsItemMode(t *testing.T) {
	setting, err := normalizeGameSetting(itemRequest("rel_Test_Item"), testMaps())
	if err != nil {
		t.Fatalf("item mode rejected: %v", err)
	}
	if setting.Mode != itemModeID {
		t.Fatalf("mode = %d, want %d", setting.Mode, itemModeID)
	}
	if got := protocol.FromU16(setting.Strings16[1]); got != "rel_Test_Item" {
		t.Fatalf("map = %q", got)
	}
	if setting.Values128[2] != 240000 {
		t.Fatalf("time limit = %d", setting.Values128[2])
	}
}

func TestNormalizeGameSettingRejectsItemModeWithoutItemMap(t *testing.T) {
	if _, err := normalizeGameSetting(itemRequest("rel_Test_Normal"), testMaps()); err == nil {
		t.Fatal("a map without the item flag must be refused in mode 3")
	}
}

func TestNormalizeGameSettingStillRejectsOtherModes(t *testing.T) {
	request := itemRequest("rel_Test_Item")
	for _, mode := range []uint32{8, 13, 16, 101} {
		request.Mode = mode
		if _, err := normalizeGameSetting(request, testMaps()); err == nil {
			t.Fatalf("mode %d must stay refused until its round is ported", mode)
		}
	}
}

func TestMapPointKinds(t *testing.T) {
	for _, kind := range []uint32{0, 101, 102, 108, 109, 115, 120, 123, 127, 128, 130, 136, 137, 138, 139, 141} {
		if !mapPointKind(kind) {
			t.Fatalf("kind %d must be a valid map point kind", kind)
		}
	}
	for _, kind := range []uint32{103, 104, 116, 121, 133, 229, 1000} {
		if mapPointKind(kind) {
			t.Fatalf("kind %d is not in the cn-717 item sheet", kind)
		}
	}
}

// TestKnownCarriedKinds covers the diagnostic set, not a refusal: a pickup is never
// refused on its kind, but an unfamiliar one is logged.
func TestKnownCarriedKinds(t *testing.T) {
	for _, kind := range []uint32{101, 102, 108, 109, 115, 120, 123, 127, 128, 130, 136, 137, 138, 139, 141, 10, 11, 15, 16} {
		if !knownCarriedKind(kind) {
			t.Fatalf("kind %d must be known", kind)
		}
	}
	for _, kind := range []uint32{0, 104, 112, 201, 229, 1000} {
		if knownCarriedKind(kind) {
			t.Fatalf("kind %d is not in the sheets this server knows", kind)
		}
	}
}

func TestSupplyIntervalRespectsBounds(t *testing.T) {
	point := protocol.SupplyReportPoint{MinMs: 1000, MaxMs: 1200}
	for i := 0; i < 200; i++ {
		delay := supplyInterval(point)
		if delay < time.Second || delay > 1200*time.Millisecond {
			t.Fatalf("interval %v outside [1s, 1.2s]", delay)
		}
	}
	// A point that states no interval gets the reference defaults.
	if delay := supplyInterval(protocol.SupplyReportPoint{}); delay < defaultSupplyMinMs*time.Millisecond ||
		delay > defaultSupplyMaxMs*time.Millisecond {
		t.Fatalf("default interval %v", delay)
	}
}

func TestSupplyStartsAfterLoadingReport(t *testing.T) {
	room := &Room{Started: true, phase: phasePlaying, tracking: 5}
	state := &supplyRound{round: 4, points: []*supplyPoint{{}}}
	room.supplies = state
	if !supplyStartsLocked(room, state) {
		t.Fatal("a report that arrived one token behind must still start the loop")
	}
	// A stale state, a state that is not the room's, a token ahead, and an ended
	// round must all refuse to start.
	stale := &supplyRound{round: 3, points: []*supplyPoint{{}}}
	if supplyStartsLocked(room, stale) {
		t.Fatal("a two-token-old state must not start")
	}
	other := &supplyRound{round: 5, points: []*supplyPoint{{}}}
	if supplyStartsLocked(room, other) {
		t.Fatal("a state that is not the room's must not start")
	}
	room.supplies = state
	ahead := &supplyRound{round: 6, points: []*supplyPoint{{}}}
	room.supplies = ahead
	if supplyStartsLocked(room, ahead) {
		t.Fatal("a token ahead of the room must not start")
	}
	room.supplies = state
	room.Started = false
	if supplyStartsLocked(room, state) {
		t.Fatal("an ended round must not start the loop")
	}
}

func TestSeated(t *testing.T) {
	session := &Session{}
	for _, slot := range []int{-1, maxRoomSlots, 99} {
		session.slot = slot
		if seated(session) {
			t.Fatalf("slot %d must not be treated as seated", slot)
		}
	}
	for slot := 0; slot < maxRoomSlots; slot++ {
		session.slot = slot
		if !seated(session) {
			t.Fatalf("slot %d must be seated", slot)
		}
	}
}

func newTestServer() *Server {
	return &Server{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

// testItemRoom is a started item-mode room with room token 7.
func testItemRoom() *Room {
	room := &Room{Started: true, phase: phasePlaying, tracking: 7}
	room.Setting.Game.Mode = itemModeID
	return room
}

// TestClientBoxPickupIsGranted covers the fallback path: a box the host created
// over the relay is tracked by its own id, so a later pickup still reaches every
// member instead of being ignored as an unknown object.
func TestClientBoxPickupIsGranted(t *testing.T) {
	server := newTestServer()
	room := testItemRoom()
	room.supplies = &supplyRound{round: 7, lastID: supplyObjectIDBase,
		dropped: map[uint32]uint32{}, clientBoxes: map[uint32]clientBox{}, held: map[int]*heldItems{}}
	session := &Session{slot: 2, room: room}
	room.slots[2].session = session
	room.slots[2].team = 1

	server.noteClientBox(room, 0x1234, clientBox{kind: 138, team: 1})
	if box := room.supplies.clientBoxes[0x1234]; box.kind != 138 || box.team != 1 {
		t.Fatalf("client box = %+v, want kind 138 team 1", box)
	}

	body := protocol.NewWriter()
	body.Unsigned(0x1234)
	body.Unsigned(138)
	if err := server.handleEatItem(session, protocol.Packet{ID: protocol.CEatItem, Payload: body.Payload()}); err != nil {
		t.Fatalf("handleEatItem: %v", err)
	}
	if _, still := room.supplies.clientBoxes[0x1234]; still {
		t.Fatal("the picked box must be removed from the round")
	}
	if held := room.supplies.held[2]; held == nil || held.selectedKind() != 138 {
		t.Fatalf("held slot = %+v, want kind 138", held)
	}
}

// TestItemPickupRefusesOtherTeamBox keeps a team box out of the other team's
// hands even when the client asks for it.
func TestItemPickupRefusesOtherTeamBox(t *testing.T) {
	server := newTestServer()
	room := testItemRoom()
	point := &supplyPoint{SupplyReportPoint: protocol.SupplyReportPoint{Kind: 0, Team: 2},
		ActiveID: 0x0BAD}
	room.supplies = &supplyRound{round: 7, lastID: supplyObjectIDBase, points: []*supplyPoint{point},
		dropped: map[uint32]uint32{}, clientBoxes: map[uint32]clientBox{}, held: map[int]*heldItems{}}
	session := &Session{slot: 0, room: room}
	room.slots[0].session = session
	room.slots[0].team = 1

	body := protocol.NewWriter()
	body.Unsigned(0x0BAD)
	body.Unsigned(101)
	if err := server.handleEatItem(session, protocol.Packet{ID: protocol.CEatItem, Payload: body.Payload()}); err != nil {
		t.Fatalf("handleEatItem: %v", err)
	}
	if point.ActiveID == 0 {
		t.Fatal("a refused pickup must leave the box on the map")
	}
	if held, ok := room.supplies.held[0]; ok && held.selectedKind() == 101 {
		t.Fatal("the other team's box was granted")
	}
}

// TestHeldItemsTwoSlots covers the reference's HeldSupplyItems: two slots, the
// first empty one is filled, use spends the selected slot and keeps the
// selection, and switching only works when the other slot holds an item.
func TestHeldItemsTwoSlots(t *testing.T) {
	var held heldItems
	if slot := held.firstEmpty(); slot != 0 {
		t.Fatalf("first empty slot = %d, want 0", slot)
	}
	if held.selectedKind() != noItemKind {
		t.Fatal("an empty seat must report the empty kind word")
	}
	held.add(0, 138)
	if held.selected != 0 || held.selectedKind() != 138 {
		t.Fatalf("first pickup: selected %d kind %d", held.selected, held.selectedKind())
	}
	// The second pickup takes slot 1 and leaves the selection alone.
	held.add(held.firstEmpty(), 120)
	if held.kinds[1] != 120 || held.selected != 0 {
		t.Fatalf("second pickup: kinds %v selected %d", held.kinds, held.selected)
	}
	// Switching moves the selection to the occupied slot.
	if !held.trySwitch() || held.selected != 1 {
		t.Fatal("switch to the occupied slot must succeed")
	}
	// Using spends the selected slot only and keeps the selection.
	kind, slot, ok := held.use()
	if !ok || kind != 120 || slot != 1 {
		t.Fatalf("use = (%d, %d, %v), want (120, 1, true)", kind, slot, ok)
	}
	if held.kinds[1] != 0 || held.selected != 1 || held.kinds[0] != 138 {
		t.Fatalf("after use: kinds %v selected %d", held.kinds, held.selected)
	}
	// The other slot still holds an item, so switching back succeeds.
	if !held.trySwitch() || held.selected != 0 {
		t.Fatal("switch back to the remaining item must succeed")
	}
	if kind, slot, ok := held.use(); !ok || kind != 138 || slot != 0 {
		t.Fatalf("second use = (%d, %d, %v), want (138, 0, true)", kind, slot, ok)
	}
	// With every slot empty, neither switching nor using does anything.
	if held.trySwitch() {
		t.Fatal("switch with every slot empty must fail")
	}
	if _, _, ok := held.use(); ok {
		t.Fatal("an empty selected slot must not produce a use")
	}
}

// TestPickupReplacesTheOldestSlotWhenTheStateDisagrees covers a seat whose two
// slots this round already believes are full: the client asked for a third box, so
// it must be taken rather than refused, and the served slot is replaced.
func TestPickupReplacesTheOldestSlotWhenTheStateDisagrees(t *testing.T) {
	server := newTestServer()
	room := testItemRoom()
	state := &supplyRound{round: 7, lastID: supplyObjectIDBase,
		dropped: map[uint32]uint32{}, clientBoxes: map[uint32]clientBox{}, held: map[int]*heldItems{}}
	room.supplies = state
	first := &supplyPoint{SupplyReportPoint: protocol.SupplyReportPoint{Kind: 138}, ActiveID: 0x101}
	second := &supplyPoint{SupplyReportPoint: protocol.SupplyReportPoint{Kind: 120}, ActiveID: 0x102}
	third := &supplyPoint{SupplyReportPoint: protocol.SupplyReportPoint{Kind: 101}, ActiveID: 0x103}
	state.points = []*supplyPoint{first, second, third}
	session := &Session{slot: 0, room: room}
	room.slots[0].session = session

	pick := func(objectID, kind uint32) {
		body := protocol.NewWriter()
		body.Unsigned(objectID)
		body.Unsigned(kind)
		if err := server.handleEatItem(session, protocol.Packet{ID: protocol.CEatItem, Payload: body.Payload()}); err != nil {
			t.Fatalf("handleEatItem: %v", err)
		}
	}
	pick(0x101, 138)
	pick(0x102, 120)
	if held := state.held[0]; held == nil || held.kinds != [nativeItemSlots]uint32{138, 120} {
		t.Fatalf("held = %+v", held)
	}
	pick(0x103, 101)
	if third.ActiveID != 0 {
		t.Fatal("the third box must be taken, not left on the map")
	}
	if held := state.held[0]; held.kinds[0] != 101 {
		t.Fatalf("slot 0 was not replaced: %v", held.kinds)
	}
}

// TestUseAndSwitchAreRefusedOutsidePlay keeps the item handlers to a running
// round: a waiting room must not broadcast anything.
func TestUseAndSwitchAreRefusedOutsidePlay(t *testing.T) {
	server := newTestServer()
	room := testItemRoom()
	room.phase = phaseOpen
	room.supplies = &supplyRound{round: 7, lastID: supplyObjectIDBase,
		dropped: map[uint32]uint32{}, clientBoxes: map[uint32]clientBox{}, held: map[int]*heldItems{}}
	session := &Session{slot: 0, room: room}
	room.slots[0].session = session
	room.slots[0].team = 1

	if err := server.handleUseItem(session, protocol.Packet{ID: protocol.CUseItem}); err != nil {
		t.Fatalf("handleUseItem: %v", err)
	}
	if err := server.handleSwitchItemSlot(session, protocol.Packet{ID: protocol.CSwitchItemSlot}); err != nil {
		t.Fatalf("handleSwitchItemSlot: %v", err)
	}
	if held := room.supplies.held[0]; held != nil {
		t.Fatal("a waiting room must not touch the slot state")
	}
}

// TestSupplyStateAdoptsTheStartingRound covers the ordering the host can produce:
// a box or a drop creates the state while the room is still loading (one round
// token behind), and the map report then has to land in the same state instead of
// replacing it.
func TestSupplyStateAdoptsTheStartingRound(t *testing.T) {
	room := &Room{tracking: 5}
	early := &supplyRound{round: 4, dropped: map[uint32]uint32{0x11: 138},
		clientBoxes: map[uint32]clientBox{0x12: {kind: 120, team: 2}}, held: map[int]*heldItems{}}
	room.supplies = early

	state := supplyStateLocked(room)
	if state != early {
		t.Fatal("the loading-time state must be reused, not replaced")
	}
	if state.round != 5 {
		t.Fatalf("round token = %d, want 5", state.round)
	}
	if len(state.dropped) != 1 || len(state.clientBoxes) != 1 {
		t.Fatalf("boxes were lost: dropped %v client %v", state.dropped, state.clientBoxes)
	}

	// A state that claims a round ahead of the room is not this round's.
	room.supplies = &supplyRound{round: 9}
	if state := supplyStateLocked(room); state.round != 5 {
		t.Fatalf("a state ahead of the room must be replaced, got round %d", state.round)
	}
}

// TestUnknownBoxPickupIsIgnored keeps a stale or duplicate pickup from leaving a
// phantom item behind: a box the round never created is not granted.
func TestUnknownBoxPickupIsIgnored(t *testing.T) {
	server := newTestServer()
	room := testItemRoom()
	room.supplies = &supplyRound{round: 7, lastID: supplyObjectIDBase,
		dropped: map[uint32]uint32{}, clientBoxes: map[uint32]clientBox{}, held: map[int]*heldItems{}}
	session := &Session{slot: 1, room: room}
	room.slots[1].session = session
	room.slots[1].team = 2

	body := protocol.NewWriter()
	body.Unsigned(0x7777)
	body.Unsigned(138)
	if err := server.handleEatItem(session, protocol.Packet{ID: protocol.CEatItem, Payload: body.Payload()}); err != nil {
		t.Fatalf("handleEatItem: %v", err)
	}
	if held := room.supplies.held[1]; held != nil && held.kinds[0] != 0 {
		t.Fatalf("an unknown box must not be granted: %+v", held)
	}
}

// TestMapBoxScheduleIsAlwaysOn covers the schedule: a real 2015 round showed the
// client placing no map-point boxes while its C702 report states the points, so
// the server is the only source of them and always runs the schedule.
func TestMapBoxScheduleIsAlwaysOn(t *testing.T) {
	server := newTestServer()
	room := testItemRoom()
	state := &supplyRound{round: 7, lastID: supplyObjectIDBase,
		points:  []*supplyPoint{{SupplyReportPoint: protocol.SupplyReportPoint{Kind: 0, MinMs: 1000, MaxMs: 2000}}},
		dropped: map[uint32]uint32{}, clientBoxes: map[uint32]clientBox{}, held: map[int]*heldItems{}}
	room.supplies = state
	server.startSupplies(room, state)
	room.mu.RLock()
	running := state.running
	room.mu.RUnlock()
	if !running {
		t.Fatal("the map box schedule must run for every item round")
	}
}

// TestOtherColourPickupIsStillGranted keeps a colour word this server may read
// differently from costing a real pickup: the client already refuses the other
// team's box, so the server only notes it.
func TestOtherColourPickupIsStillGranted(t *testing.T) {
	server := newTestServer()
	room := testItemRoom()
	room.supplies = &supplyRound{round: 7, lastID: supplyObjectIDBase,
		dropped: map[uint32]uint32{}, held: map[int]*heldItems{},
		clientBoxes: map[uint32]clientBox{0x5151: {kind: 138, team: 2}}}
	session := &Session{slot: 0, room: room}
	room.slots[0].session = session
	room.slots[0].team = 1

	body := protocol.NewWriter()
	body.Unsigned(0x5151)
	body.Unsigned(138)
	if err := server.handleEatItem(session, protocol.Packet{ID: protocol.CEatItem, Payload: body.Payload()}); err != nil {
		t.Fatalf("handleEatItem: %v", err)
	}
	if held := room.supplies.held[0]; held == nil || held.kinds[0] != 138 {
		t.Fatalf("held = %+v, want kind 138", held)
	}
}

// dropBody builds the C2S gcpReqCreateItem body: six floats, the kind and two words.
func dropBody(kind, class uint32) []byte {
	w := protocol.NewWriter()
	for _, v := range []float32{1, 2, 3, 0, 0, 0} {
		w.Float(v)
	}
	w.Unsigned(kind)
	w.Unsigned(0)
	w.Unsigned(class)
	return w.Payload()
}

// TestKnockDownBoxRequestIgnored covers the request the client sends on every
// knock-down: a kind-0 random box, which the 2015 game does not place.
func TestKnockDownBoxRequestIgnored(t *testing.T) {
	server := newTestServer()
	room := testItemRoom()
	room.supplies = &supplyRound{round: 7, lastID: supplyObjectIDBase,
		dropped: map[uint32]uint32{}, clientBoxes: map[uint32]clientBox{}, held: map[int]*heldItems{}}
	session := &Session{slot: 0, room: room}
	room.slots[0].session = session
	room.slots[0].team = 1

	packet := protocol.Packet{ID: protocol.CCreateItem, Payload: dropBody(0, 10000)}
	if err := server.handleCreateItem(session, packet); err != nil {
		t.Fatalf("handleCreateItem: %v", err)
	}
	if len(room.supplies.dropped) != 0 {
		t.Fatalf("a knock-down box request must be refused, dropped=%v", room.supplies.dropped)
	}
	if room.supplies.lastID != supplyObjectIDBase {
		t.Fatal("a refused request must not consume a box id")
	}
}

// TestThrownItemLeavesTheSlot keeps the round's slot state in step with the
// client: throwing the held item clears the slot, otherwise every later pickup is
// refused as "every item slot is taken".
func TestThrownItemLeavesTheSlot(t *testing.T) {
	server := newTestServer()
	room := testItemRoom()
	held := &heldItems{}
	held.add(0, 138)
	room.supplies = &supplyRound{round: 7, lastID: supplyObjectIDBase,
		dropped: map[uint32]uint32{}, clientBoxes: map[uint32]clientBox{},
		held: map[int]*heldItems{0: held}}
	session := &Session{slot: 0, room: room}
	room.slots[0].session = session
	room.slots[0].team = 1

	packet := protocol.Packet{ID: protocol.CCreateItem, Payload: dropBody(138, 10000)}
	if err := server.handleCreateItem(session, packet); err != nil {
		t.Fatalf("handleCreateItem: %v", err)
	}
	if held.kinds[0] != 0 {
		t.Fatalf("the thrown item is still held: %v", held.kinds)
	}
	if len(room.supplies.dropped) != 1 {
		t.Fatalf("the thrown box was not tracked: %v", room.supplies.dropped)
	}
}

// TestUseFallsBackToTheOtherSlot covers the request that used to be dropped: the
// client keeps its own selection, so after spending the item in one slot it may
// ask to use the item still held in the other one.
func TestUseFallsBackToTheOtherSlot(t *testing.T) {
	var held heldItems
	held.add(0, 138)
	held.add(1, 115)
	if held.selected != 0 {
		t.Fatalf("selection = %d, want 0", held.selected)
	}
	kind, slot, ok := held.use()
	if !ok || kind != 138 || slot != 0 {
		t.Fatalf("first use = (%d,%d,%v)", kind, slot, ok)
	}
	// The selected slot is empty now; the client asks again for the other item.
	if held.selectedKind() != noItemKind {
		t.Fatalf("selected kind = %#x, want the empty word", held.selectedKind())
	}
	kind, slot, ok = held.use()
	if !ok || kind != 115 || slot != 1 {
		t.Fatalf("fallback use = (%d,%d,%v), want (115,1,true)", kind, slot, ok)
	}
	if _, _, ok := held.use(); ok {
		t.Fatal("an empty seat must not produce a use")
	}
}

// TestRespawnBoxIsAcceptedWithTeamColour covers the box the client asks for a few
// seconds after a respawn: it is the only kind-0 request that is honoured, and it
// carries the seat's team so the client draws it in that colour.
func TestRespawnBoxIsAcceptedWithTeamColour(t *testing.T) {
	server := newTestServer()
	room := testItemRoom()
	session := &Session{slot: 1, room: room}
	room.slots[1].session = session
	room.slots[1].team = 2
	server.noteRespawn(room, 1)

	if err := server.handleCreateItem(session, protocol.Packet{ID: protocol.CCreateItem, Payload: dropBody(0, 10000)}); err != nil {
		t.Fatalf("handleCreateItem: %v", err)
	}
	room.mu.RLock()
	tracked := len(room.supplies.dropped)
	room.mu.RUnlock()
	if tracked != 1 {
		t.Fatalf("the respawn box was not created, dropped=%d", tracked)
	}
}

// TestUnfamiliarKindIsStillGranted keeps a box whose kind no sheet in this server
// names: the client already took the box, so refusing would just lose the item.
func TestUnfamiliarKindIsStillGranted(t *testing.T) {
	server := newTestServer()
	room := testItemRoom()
	room.supplies = &supplyRound{round: 7, lastID: supplyObjectIDBase,
		dropped: map[uint32]uint32{}, held: map[int]*heldItems{},
		clientBoxes: map[uint32]clientBox{0x3131: {kind: 104, team: 0}}}
	session := &Session{slot: 0, room: room}
	room.slots[0].session = session
	room.slots[0].team = 1

	body := protocol.NewWriter()
	body.Unsigned(0x3131)
	body.Unsigned(104)
	if err := server.handleEatItem(session, protocol.Packet{ID: protocol.CEatItem, Payload: body.Payload()}); err != nil {
		t.Fatalf("handleEatItem: %v", err)
	}
	if held := room.supplies.held[0]; held == nil || held.kinds[0] != 104 {
		t.Fatalf("kind 104 must be granted: %+v", held)
	}
}

// TestImmediateItemTakesNoSlot covers the kinds that apply where they are picked
// up: the box goes away, no slot is used.
func TestImmediateItemTakesNoSlot(t *testing.T) {
	server := newTestServer()
	room := testItemRoom()
	room.supplies = &supplyRound{round: 7, lastID: supplyObjectIDBase,
		dropped: map[uint32]uint32{}, held: map[int]*heldItems{},
		clientBoxes: map[uint32]clientBox{0x4141: {kind: 10, team: 0}}}
	session := &Session{slot: 0, room: room}
	room.slots[0].session = session
	room.slots[0].team = 1

	body := protocol.NewWriter()
	body.Unsigned(0x4141)
	body.Unsigned(10)
	if err := server.handleEatItem(session, protocol.Packet{ID: protocol.CEatItem, Payload: body.Payload()}); err != nil {
		t.Fatalf("handleEatItem: %v", err)
	}
	if held := room.supplies.held[0]; held != nil && held.kinds[0] != 0 {
		t.Fatalf("an immediate item took a slot: %+v", held)
	}
	if _, still := room.supplies.clientBoxes[0x4141]; still {
		t.Fatal("the picked box must be removed")
	}
}

// TestRespawnClearsTheItemSlots pins the chosen rule: the item bar starts empty
// after a respawn, matching what the client shows.
func TestRespawnClearsTheItemSlots(t *testing.T) {
	server := newTestServer()
	room := testItemRoom()
	held := &heldItems{}
	held.add(0, 138)
	held.add(1, 141)
	room.supplies = &supplyRound{round: 7, lastID: supplyObjectIDBase,
		dropped: map[uint32]uint32{}, clientBoxes: map[uint32]clientBox{},
		held: map[int]*heldItems{0: held}}

	server.noteRespawn(room, 0)
	if held.kinds != [nativeItemSlots]uint32{} {
		t.Fatalf("a respawn must clear the slots: %v", held.kinds)
	}
	if held.selected != 0 {
		t.Fatalf("the selection must reset: %d", held.selected)
	}
}
