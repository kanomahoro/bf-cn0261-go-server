package game

import (
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/store"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/world"
)

// zombieTestRoom builds a room whose setting names the zombie mode, with the
// given seats occupied by throwaway sessions.
func zombieTestRoom(seats int) *Room {
	var setting protocol.SessionSetting
	setting.Game.Mode = zombieModeID
	setting.Game.Values128[2] = zombieRoundLimitMs
	setting.State = phasePlaying
	room := &Room{ID: 1, Setting: setting, zombieKingSlot: -1, phase: phasePlaying}
	room.Started = true
	for slot := 0; slot < seats; slot++ {
		room.slots[slot] = roomSlot{session: &Session{ID: uint64(slot + 1), slot: slot, room: room}, team: 1}
	}
	return room
}

// Every seat starts the zombie battle on the human team and the notices carry
// that team before the start's seat records are built.
func TestZombieRoundStartsEverySeatHuman(t *testing.T) {
	room := zombieTestRoom(3)
	room.slots[1].team = 2 // a waiting-room team switch must not survive the start
	notices := room.beginZombieRound()
	if len(notices) != 3 {
		t.Fatalf("notices = %d, want one per occupied seat", len(notices))
	}
	for slot := 0; slot < 3; slot++ {
		if team := room.Team(slot); team != zombieHumanTeam {
			t.Fatalf("slot %d team = %d, want %d", slot, team, zombieHumanTeam)
		}
		if notices[slot].ID != 504 {
			t.Fatalf("notice %d id = %d, want the team-change reply 504", slot, notices[slot].ID)
		}
	}
	if king := room.ZombieKingSlot(); king != -1 {
		t.Fatalf("king slot = %d, want none before the opening", king)
	}
}

// The zombie team's respawn markers are the fifteenth group: the native spawn
// offer validates the marker against the seat's team, so a converted seat must
// respawn on team 14's markers.
func TestZombieTeamRespawnsOnTeamFourteenMarkers(t *testing.T) {
	room := zombieTestRoom(2)
	life := NewBattleLife(func() time.Time { return time.Unix(0, 0) })
	if err := life.Bind(0, objectFor(0), zombieHumanTeam); err != nil {
		t.Fatalf("bind: %v", err)
	}
	room.life = life
	room.slots[0].team = zombieTeam
	if !life.MarkDownAndConvert(0, zombieTeam) {
		t.Fatal("the converted seat must enter the respawn wait")
	}
	if !life.BeginRespawn(0) {
		t.Fatal("a converted seat must be offerable")
	}
	index := room.NextSpawnIndex(0, 8)
	if index != 141 {
		t.Fatalf("spawn marker = %d, want the first team-14 marker 141", index)
	}
	if _, err := life.OfferRespawn(0, index); err != nil {
		t.Fatalf("team-14 marker refused: %v", err)
	}
}

// The opening turns exactly one human into the king once it elapses.
func TestZombieOpeningTurnsOneRandomHuman(t *testing.T) {
	room := zombieTestRoom(3)
	room.beginZombieRound()
	server := zombieTestServer(room)
	opening := zombieOpening
	zombieOpening = 10 * time.Millisecond
	defer func() { zombieOpening = opening }()

	server.scheduleZombieOpening(room, room.generation)
	deadline := time.Now().Add(2 * time.Second)
	for room.ZombieKingSlot() < 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	king := room.ZombieKingSlot()
	if king < 0 {
		t.Fatal("no king was picked")
	}
	zombies := 0
	for slot := 0; slot < 3; slot++ {
		if room.Team(slot) == zombieTeam {
			zombies++
		}
	}
	if zombies != 1 {
		t.Fatalf("zombies = %d, want exactly the king", zombies)
	}
	// Every client must be told, and the notice must really carry team 14. The
	// broadcast follows the pick, so it is waited for rather than assumed.
	seen := func() int {
		seen := 0
		for slot := 0; slot < 3; slot++ {
			for _, team := range teamsOf(room.slots[slot].session.send.(*recorder), 504) {
				if team == uint32(zombieTeam) {
					seen++
				}
			}
		}
		return seen
	}
	for seen() < 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := seen(); got != 3 {
		t.Fatalf("team-14 notices = %d, want one per member", got)
	}
}

// teamsOf lists the team word of every packet with the given id the recorder saw.
func teamsOf(rec *recorder, id uint16) []uint32 {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	var out []uint32
	for _, packet := range rec.packets {
		if packet.ID != id || len(packet.Payload) < 8 {
			continue
		}
		out = append(out, uint32(packet.Payload[4])|uint32(packet.Payload[5])<<8|
			uint32(packet.Payload[6])<<16|uint32(packet.Payload[7])<<24)
	}
	return out
}

// A dead human turns, a dead zombini comes back, and neither ends the round.
func TestZombieDeathReportConvertsAndRevives(t *testing.T) {
	room := zombieTestRoom(3)
	room.beginZombieRound()
	server := zombieTestServer(room)

	server.zombieDeathReport(room.slots[1].session)
	if team := room.Team(1); team != zombieTeam {
		t.Fatalf("the first dead human must become the king, team = %d", team)
	}
	if king := room.ZombieKingSlot(); king != 1 {
		t.Fatalf("king slot = %d, want 1", king)
	}

	server.zombieDeathReport(room.slots[2].session)
	if team := room.Team(2); team != zombieTeam {
		t.Fatalf("a dead human must become a zombini, team = %d", team)
	}

	// A zombini's death is a revive: the same notice goes out again.
	server.zombieDeathReport(room.slots[2].session)
	if team := room.Team(2); team != zombieTeam {
		t.Fatalf("a zombini must stay on the zombie team, team = %d", team)
	}
	if king := room.ZombieKingSlot(); king != 1 {
		t.Fatalf("king slot = %d, want the first one, 1", king)
	}
}

// zombieTestServer builds the smallest server the round logic needs: the room
// registry and a logger, with every broadcast going into the recorded packets.
func zombieTestServer(room *Room) *Server {
	server := &Server{
		log:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		rooms: map[uint16]*Room{room.ID: room},
	}
	for slot := range room.slots {
		if room.slots[slot].session != nil {
			room.slots[slot].session.send = &recorder{}
		}
	}
	return server
}

// recorder collects every packet a session sends.
type recorder struct {
	packets []protocol.Packet
	mu      sync.Mutex
}

// Send appends one packet.
func (r *recorder) Send(packet protocol.Packet) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.packets = append(r.packets, packet)
	return nil
}

// ids lists the packet ids the recorder saw.
func (r *recorder) ids() []uint16 {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]uint16, 0, len(r.packets))
	for _, packet := range r.packets {
		out = append(out, packet.ID)
	}
	return out
}

// The king's ultra request is confirmed to the king alone with gspPveItemUsed;
// the same request from a human gets nothing.
func TestZombieKingUltraIsConfirmedToTheKingAlone(t *testing.T) {
	room := zombieTestRoom(3)
	room.beginZombieRound()
	server := zombieTestServer(room)
	room.mu.Lock()
	room.slots[1].team = zombieTeam
	room.zombieKingSlot = 1
	room.mu.Unlock()

	request := protocol.Packet{ID: 707, Payload: ultraRequestPayload()}
	server.handleZombieItemRequest(room.slots[0].session, request)
	server.handleZombieItemRequest(room.slots[2].session, request)
	if ids := room.slots[0].session.send.(*recorder).ids(); len(ids) != 0 {
		t.Fatalf("a human got %v, want nothing", ids)
	}

	server.handleZombieItemRequest(room.slots[1].session, request)
	ids := room.slots[1].session.send.(*recorder).ids()
	if len(ids) != 1 || ids[0] != 715 {
		t.Fatalf("the king got %v, want one gspPveItemUsed", ids)
	}
}

// ultraRequestPayload is C707 with the no-object word and the ultra's item kind.
func ultraRequestPayload() []byte {
	writer := protocol.NewWriter()
	writer.Unsigned(zombieNoObject)
	writer.Unsigned(zombieUltraItemKind)
	return writer.Payload()
}

// A seat bound after the round started can join the zombie team: the life
// tracker refuses a third team through Bind, so the conversion path is the only
// way in.
func TestZombieConversionBypassesTheTwoTeamBind(t *testing.T) {
	life := NewBattleLife(nil)
	if err := life.Bind(0, objectFor(0), 3); err == nil {
		t.Fatal("bind accepted a third team")
	}
	if err := life.Bind(0, objectFor(0), zombieHumanTeam); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if err := life.Bind(0, objectFor(0), zombieTeam); err == nil {
		t.Fatal("bind must refuse a team change within the round")
	}
	if !life.MarkDownAndConvert(0, zombieTeam) {
		t.Fatal("conversion must be accepted")
	}
	if team := life.Team(0); team != zombieTeam {
		t.Fatalf("team = %d, want %d", team, zombieTeam)
	}
}

// A converted seat is alive again: its next death report is accepted, which is
// what the fallback path (a death in the combat stream) needs.
func TestZombieConvertedSeatCanDieAgain(t *testing.T) {
	life := NewBattleLife(nil)
	if err := life.Bind(0, objectFor(0), zombieHumanTeam); err != nil {
		t.Fatalf("bind: %v", err)
	}
	dead := protocol.PeerLifeEvent{Kind: protocol.LifeDead, VictimObject: objectFor(0), ActorObject: protocol.Unset}
	if accepted, err := life.Apply(0, dead, false); err != nil || !accepted {
		t.Fatalf("first death: accepted=%v err=%v", accepted, err)
	}
	if !life.ConvertTeam(0, zombieTeam) {
		t.Fatal("conversion refused")
	}
	if accepted, err := life.Apply(0, dead, false); err != nil || !accepted {
		t.Fatalf("second death after conversion: accepted=%v err=%v", accepted, err)
	}
	if team := life.Team(0); team != zombieTeam {
		t.Fatalf("team = %d, want %d", team, zombieTeam)
	}
}

// The settlement's wire rows keep the zombie team while the stored rows use the
// schema's two teams.
func TestZombieResultTeamIsMappedOnlyForStorage(t *testing.T) {
	if got := store.StoredTeam(zombieTeam); got != 2 {
		t.Fatalf("stored team = %d, want 2", got)
	}
	if got := store.StoredTeam(zombieHumanTeam); got != 1 {
		t.Fatalf("stored team = %d, want 1", got)
	}
	if got := store.StoredWinner(zombieTeam); got != 2 {
		t.Fatalf("stored winner = %d, want 2", got)
	}
	if got := store.StoredWinner(zombieHumanTeam); got != 1 {
		t.Fatalf("stored winner = %d, want 1", got)
	}
}

// A death reported in the combat stream names the victim, which need not be the
// reporter: the seat that died turns, the killer does not.
func TestZombieCombatDeathTurnsTheVictimNotTheReporter(t *testing.T) {
	room := zombieTestRoom(3)
	room.beginZombieRound()
	server := zombieTestServer(room)

	server.zombieDeathOfObject(room, objectFor(2))
	if team := room.Team(2); team != zombieTeam {
		t.Fatalf("victim team = %d, want the zombie team", team)
	}
	if team := room.Team(0); team != zombieHumanTeam {
		t.Fatalf("reporter team = %d, want it to stay human", team)
	}
	if king := room.ZombieKingSlot(); king != 2 {
		t.Fatalf("king slot = %d, want the first dead seat, 2", king)
	}
}

// A zombie-round seat that asks to respawn is answered on the zombie team's
// markers, so a client that waits for the server behaves like one that revives
// itself.
func TestZombieRespawnRequestIsAnsweredOnTeamFourteen(t *testing.T) {
	room := zombieTestRoom(2)
	room.beginZombieRound()
	server := zombieTestServer(room)
	// A one-millisecond wait keeps the offer from the round's own timer quick; the
	// catalog is keyed by the resolved character type, so every id is covered.
	server.respawn = world.Respawn{Characters: []world.RespawnCharacter{
		{ID: 0, DurationMilliseconds: 1}, {ID: 1, DurationMilliseconds: 1},
		{ID: 2, DurationMilliseconds: 1}, {ID: 3, DurationMilliseconds: 1},
	}}
	room.mu.Lock()
	room.slots[0].team = zombieTeam
	room.zombieKingSlot = 0
	room.mu.Unlock()
	life := NewBattleLife(nil)
	if err := life.Bind(0, objectFor(0), zombieHumanTeam); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if err := life.Bind(1, objectFor(1), zombieHumanTeam); err != nil {
		t.Fatalf("bind: %v", err)
	}
	room.life = life

	request := protocol.Packet{ID: 713, Payload: respawnPayload(objectFor(0))}
	if err := server.handleRespawn(room.slots[0].session, request); err != nil {
		t.Fatalf("respawn request: %v", err)
	}
	rec := room.slots[0].session.send.(*recorder)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		rec.mu.Lock()
		packets := append([]protocol.Packet(nil), rec.packets...)
		rec.mu.Unlock()
		for _, packet := range packets {
			if packet.ID != 708 || len(packet.Payload) < 8 {
				continue
			}
			marker := uint32(packet.Payload[4]) | uint32(packet.Payload[5])<<8 |
				uint32(packet.Payload[6])<<16 | uint32(packet.Payload[7])<<24
			if marker/10 != uint32(zombieTeam) {
				t.Fatalf("respawn marker = %d, want a team-14 marker", marker)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("no respawn offer was sent")
}

// respawnPayload is the C713 body: the player object and the client's marker.
func respawnPayload(objectID uint32) []byte {
	writer := protocol.NewWriter()
	writer.Unsigned(objectID)
	writer.Unsigned(0)
	return writer.Payload()
}

// The death report's body is not read: a build may send the seat and team words
// on the same request, and only the sender's own seat may turn.
func TestZombieDeathReportIgnoresTheBody(t *testing.T) {
	room := zombieTestRoom(2)
	room.beginZombieRound()
	server := zombieTestServer(room)

	writer := protocol.NewWriter()
	writer.Unsigned(uint32(0)) // the seat word a build may fill in
	writer.Unsigned(uint32(2)) // a team word naming the other seat
	packet := protocol.Packet{ID: 515, Payload: writer.Payload()}
	if err := server.handleChangeTeam(room.slots[1].session, packet); err != nil {
		t.Fatalf("death report refused: %v", err)
	}
	if team := room.Team(1); team != zombieTeam {
		t.Fatalf("the reporting seat's team = %d, want the zombie team", team)
	}
	if team := room.Team(0); team != zombieHumanTeam {
		t.Fatalf("the seat named in the body turned instead: team = %d", team)
	}
}

// The zombie battle's ammo box: either side may take one, the box leaves the map
// for every member, and nothing is granted (the ammo lives in the client).
func TestZombieAmmoBoxIsConsumedByEitherSide(t *testing.T) {
	room := zombieTestRoom(2)
	room.beginZombieRound()
	server := zombieTestServer(room)
	room.mu.Lock()
	room.slots[0].team = zombieTeam
	room.zombieKingSlot = 0
	room.mu.Unlock()

	point := &supplyPoint{SupplyReportPoint: protocol.SupplyReportPoint{Kind: 0, MinMs: 15000, MaxMs: 20000}}
	point.ActiveID = 0x3001
	state := &supplyRound{round: room.tracking, lastID: 0x3001,
		points:  []*supplyPoint{point},
		dropped: map[uint32]uint32{}, clientBoxes: map[uint32]clientBox{}, held: map[int]*heldItems{}}
	room.supplies = state

	body := protocol.NewWriter()
	body.Unsigned(0x3001)
	body.Unsigned(0)
	request := protocol.Packet{ID: protocol.CEatItem, Payload: body.Payload()}

	// The zombie takes the box: the box is consumed and every member is told.
	if err := server.handleEatItem(room.slots[0].session, request); err != nil {
		t.Fatalf("zombie pickup: %v", err)
	}
	if point.ActiveID != 0 {
		t.Fatalf("the box is still on the map: %d", point.ActiveID)
	}
	for slot := 0; slot < 2; slot++ {
		if ids := room.slots[slot].session.send.(*recorder).ids(); len(ids) == 0 || ids[len(ids)-1] != 712 {
			t.Fatalf("slot %d saw %v, want the box-removal broadcast 712", slot, ids)
		}
	}
	if held := state.held[0]; held != nil {
		t.Fatalf("the zombie battle must not grant a slotted item: %+v", held)
	}
}

// A pickup of a box the round never placed is ignored, as in the item battle.
func TestZombieUnknownBoxIsIgnored(t *testing.T) {
	room := zombieTestRoom(2)
	room.beginZombieRound()
	server := zombieTestServer(room)
	room.supplies = &supplyRound{round: room.tracking, lastID: 0x3001,
		points:  []*supplyPoint{},
		dropped: map[uint32]uint32{}, clientBoxes: map[uint32]clientBox{}, held: map[int]*heldItems{}}

	body := protocol.NewWriter()
	body.Unsigned(0x9999)
	body.Unsigned(0)
	if err := server.handleEatItem(room.slots[1].session, protocol.Packet{ID: protocol.CEatItem, Payload: body.Payload()}); err != nil {
		t.Fatalf("pickup: %v", err)
	}
	if ids := room.slots[1].session.send.(*recorder).ids(); len(ids) != 0 {
		t.Fatalf("an unknown box must send nothing, saw %v", ids)
	}
}

// zombieReportBody builds the host's C702 body (one word and fifteen counted
// vectors, the shape kr-812 sub_5312F0 reads) with the given points.
func zombieReportBody(points []protocol.SupplyReportPoint) []byte {
	w := protocol.NewWriter()
	w.Unsigned(0)
	w.Unsigned(0)
	w.Unsigned(uint32(len(points)))
	for _, point := range points {
		w.Float(point.X)
		w.Float(point.Y)
		w.Float(point.Z)
	}
	w.Unsigned(uint32(len(points)))
	for _, point := range points {
		w.Unsigned(point.Kind)
	}
	w.Unsigned(uint32(len(points)))
	for _, point := range points {
		w.Unsigned(point.Team)
	}
	for _, pick := range []func(protocol.SupplyReportPoint) uint32{
		func(p protocol.SupplyReportPoint) uint32 { return p.MaxMs },
		func(p protocol.SupplyReportPoint) uint32 { return p.MinMs },
	} {
		w.Unsigned(uint32(len(points)))
		for _, point := range points {
			w.Unsigned(pick(point))
		}
	}
	for i := 0; i < 9; i++ {
		w.Unsigned(0)
	}
	w.Unsigned(1)
	w.String(protocol.U16String("supply_point"))
	return w.Payload()
}

// The zombie battle accepts the host's map supply report and keeps its points,
// including the ammo kind the item sheet does not list.
func TestZombieMapReportInstallsAmmoPoints(t *testing.T) {
	room := zombieTestRoom(2)
	room.beginZombieRound()
	room.mu.Lock()
	room.phase = phasePlaying
	room.mu.Unlock()
	server := zombieTestServer(room)
	server.maps = testMaps()

	// The fixture's slot 0 is the master.
	points := []protocol.SupplyReportPoint{
		{X: 1, Y: 2, Z: 3, Kind: 0, MinMs: 15000, MaxMs: 30000},
		{X: 4, Y: 5, Z: 6, Kind: 777, MinMs: 20000, MaxMs: 20000},
	}
	packet := protocol.Packet{ID: protocol.CGameReportMapData, Payload: zombieReportBody(points)}
	if err := server.handleMapReport(room.slots[0].session, packet); err != nil {
		t.Fatalf("map report: %v", err)
	}
	state := room.supplies
	if state == nil || len(state.points) != 2 {
		t.Fatalf("points = %v, want the two reported points", state)
	}
	if state.points[1].Kind != 777 {
		t.Fatalf("an ammo kind outside the item sheet was dropped: %+v", state.points[1])
	}
}

// Supplies stay off outside the item battle and the zombie battle.
func TestSupplyModeCoversOnlyItemAndZombie(t *testing.T) {
	server := newTestServer()
	room := testItemRoom()
	if !server.supplyMode(room) {
		t.Fatal("the item battle must run supplies")
	}
	room.Setting.Game.Mode = zombieModeID
	if !server.supplyMode(room) {
		t.Fatal("the zombie battle must run supplies")
	}
	room.Setting.Game.Mode = 1
	if server.supplyMode(room) {
		t.Fatal("the ordinary battle must not run supplies")
	}
}
