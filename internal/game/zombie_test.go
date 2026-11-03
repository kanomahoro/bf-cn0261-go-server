package game

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
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
// registry and a logger, with every broadcast going nowhere.
func zombieTestServer(room *Room) *Server {
	return &Server{
		log:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		rooms: map[uint16]*Room{room.ID: room},
	}
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
