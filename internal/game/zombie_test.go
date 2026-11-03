package game

import (
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/store"
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
