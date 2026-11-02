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

func TestItemModeKinds(t *testing.T) {
	for _, kind := range []uint32{0, 101, 102, 108, 109, 115, 120, 123, 127, 128, 130, 136, 137, 138, 139, 141} {
		if !itemModeAccepts(kind) {
			t.Fatalf("kind %d must be offered", kind)
		}
	}
	for _, kind := range []uint32{103, 116, 133, 229, 1000} {
		if itemModeAccepts(kind) {
			t.Fatalf("kind %d is not in the cn-717 item sheet", kind)
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
		dropped: map[uint32]uint32{}, clientBoxes: map[uint32]uint32{}, held: map[int]uint32{}}
	session := &Session{slot: 2, room: room}
	room.slots[2].session = session
	room.slots[2].team = 1

	server.noteClientBox(room, 0x1234, 138)
	if kind := room.supplies.clientBoxes[0x1234]; kind != 138 {
		t.Fatalf("client box = %d, want 138", kind)
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
	if kind := room.supplies.held[2]; kind != 138 {
		t.Fatalf("held kind = %d, want 138", kind)
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
		dropped: map[uint32]uint32{}, clientBoxes: map[uint32]uint32{}, held: map[int]uint32{}}
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
	if kind, held := room.supplies.held[0]; held {
		t.Fatalf("the other team's box was granted as kind %d", kind)
	}
}
