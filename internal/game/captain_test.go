package game

import (
	"io"
	"log/slog"
	"testing"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
)

func captainTestServer() *Server {
	return &Server{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

// captainRecorder captures what the round sends to one seat.
type captainRecorder struct{ sent []protocol.Packet }

func (r *captainRecorder) Send(packet protocol.Packet) error {
	r.sent = append(r.sent, packet)
	return nil
}

// captainAnnouncements counts the seat announcements a recorder saw.
func (r *captainRecorder) captainAnnouncements() int {
	count := 0
	for _, packet := range r.sent {
		if packet.ID == 982 {
			count++
		}
	}
	return count
}

// captainTestRoom builds a room whose seats own the given teams; a team of 0
// leaves the seat empty.
func captainTestRoom(teams [maxRoomSlots]uint8) *Room {
	room, _ := captainTestRoomWithRecorders(teams)
	return room
}

func captainTestRoomWithRecorders(teams [maxRoomSlots]uint8) (*Room, []*captainRecorder) {
	room := &Room{
		Setting:  protocol.SessionSetting{Game: protocol.GameSetting{Mode: captainModeID}},
		phase:    phasePlaying,
		Started:  true,
		captains: [3]int{captainNone, captainNone, captainNone},
	}
	recorders := make([]*captainRecorder, 0, maxRoomSlots)
	for slot, team := range teams {
		if team == 0 {
			continue
		}
		recorder := &captainRecorder{}
		recorders = append(recorders, recorder)
		room.slots[slot].session = &Session{slot: slot, send: recorder}
		room.slots[slot].team = team
	}
	life := NewBattleLife(nil, false)
	for slot := range room.slots {
		if room.slots[slot].session == nil {
			continue
		}
		if err := life.Bind(uint8(slot), objectFor(slot), room.slots[slot].team); err != nil {
			panic(err)
		}
	}
	room.life = life
	return room, recorders
}

// A team without a Captain gets one of its own standing seats, and the pick
// never crosses to the other team.
func TestPickCaptainStaysOnItsTeam(t *testing.T) {
	room := captainTestRoom([maxRoomSlots]uint8{1, 2, 2})
	life := room.Life()
	for attempt := 0; attempt < 32; attempt++ {
		pick, ok := pickCaptain(room, life, 2, captainNone)
		if !ok {
			t.Fatal("team 2 has two standing seats")
		}
		if room.slots[pick].team != 2 {
			t.Fatalf("picked slot %d of team %d", pick, room.slots[pick].team)
		}
	}
	if _, ok := pickCaptain(room, life, 1, captainNone); !ok {
		t.Fatal("team 1 has one standing seat")
	}
}

// A dying seat and a departed seat are never named; when the team has nobody
// left the pick fails instead of naming a dead character.
func TestPickCaptainSkipsTheFallen(t *testing.T) {
	room := captainTestRoom([maxRoomSlots]uint8{1, 1})
	life := room.Life()
	// The first seat is caught in a bubble (still standing), the second is dying.
	life.players[0].state = stateBubbled
	life.players[1].state = stateDying
	pick, ok := pickCaptain(room, life, 1, captainNone)
	if !ok || pick != 0 {
		t.Fatalf("pick = %d, %v; want the bubbled seat 0", pick, ok)
	}
	life.players[0].state = stateDead
	if _, ok := pickCaptain(room, life, 1, captainNone); ok {
		t.Fatal("a team with nobody standing must not be given a Captain")
	}
	life.players[0].state = stateActive
	life.Drop(0)
	if _, ok := pickCaptain(room, life, 1, captainNone); ok {
		t.Fatal("a departed seat must not be named")
	}
}

// Standing is the predicate the appointments rest on: an active, bubbled or
// briefly stunned seat counts, everything from the kick onwards does not.
func TestStandingStates(t *testing.T) {
	life := NewBattleLife(nil, false)
	if err := life.Bind(0, objectFor(0), 1); err != nil {
		t.Fatal(err)
	}
	if !life.Standing(0) {
		t.Fatal("an active seat stands")
	}
	for _, state := range []lifeState{stateBubbled, stateStunned, stateStunRecovering} {
		life.players[0].state = state
		if !life.Standing(0) {
			t.Fatalf("%v still stands", state)
		}
	}
	for _, state := range []lifeState{stateKicked, stateDying, stateDead, stateRespawnWaiting,
		stateRespawnOffered, stateRespawnConfirmed} {
		life.players[0].state = state
		if life.Standing(0) {
			t.Fatalf("%v must not stand", state)
		}
	}
	life.players[0].state = stateActive
	life.Drop(0)
	if life.Standing(0) {
		t.Fatal("a departed seat must not stand")
	}
	if life.Standing(uint8(maxRoomSlots)) {
		t.Fatal("a seat past the roster must not stand")
	}
}

// The full rule: one Captain per team at the start, the Captaincy moves to a
// standing teammate when the Captain goes down, a repeated check keeps the
// successor, and a stale round generation changes nothing.
func TestRefreshCaptainsAppointsAndHandsOn(t *testing.T) {
	room := captainTestRoom([maxRoomSlots]uint8{1, 1, 2, 2})
	room.generation = 7
	server := captainTestServer()

	server.refreshCaptains(room, 7)
	for team := uint8(1); team <= 2; team++ {
		slot := room.CaptainSlot(team)
		if slot == captainNone {
			t.Fatalf("team %d has no Captain", team)
		}
		if room.slots[slot].team != team {
			t.Fatalf("team %d's Captain sits on team %d", team, room.slots[slot].team)
		}
	}

	first := room.CaptainSlot(1)
	room.life.players[first].state = stateDying
	server.refreshCaptains(room, 7)
	successor := room.CaptainSlot(1)
	if successor == captainNone || successor == first {
		t.Fatalf("team 1's Captaincy did not move off the fallen seat %d (now %d)", first, successor)
	}
	if room.slots[successor].team != 1 {
		t.Fatalf("the successor sits on team %d", room.slots[successor].team)
	}

	// A second death report for the same seat must not move it again.
	server.refreshCaptains(room, 7)
	if room.CaptainSlot(1) != successor {
		t.Fatal("a repeated check replaced a standing Captain")
	}

	// A check fenced to another round generation leaves the appointments alone.
	other := room.CaptainSlot(2)
	room.life.players[other].state = stateDying
	server.refreshCaptains(room, 8)
	if room.CaptainSlot(2) != other {
		t.Fatal("a stale generation changed the appointments")
	}
}

// A team whose last seat is down is vacated, and the seat that comes back is
// announced again: a respawned character is a new one on the client, so the
// Captaincy has to be re-sent rather than assumed.
func TestRefreshCaptainsVacatesAndAnnouncesAgain(t *testing.T) {
	room, recorders := captainTestRoomWithRecorders([maxRoomSlots]uint8{1, 1})
	room.generation = 2
	server := captainTestServer()
	server.refreshCaptains(room, 2)
	captain := room.CaptainSlot(1)
	if captain == captainNone {
		t.Fatal("team 1 has no Captain")
	}
	for _, recorder := range recorders {
		if recorder.captainAnnouncements() != 1 {
			t.Fatalf("the opening announcement reached %d times, want once", recorder.captainAnnouncements())
		}
	}

	room.life.players[0].state = stateDead
	room.life.players[1].state = stateDead
	server.refreshCaptains(room, 2)
	if room.CaptainSlot(1) != captainNone {
		t.Fatal("a team with nobody standing must be vacated")
	}
	for _, recorder := range recorders {
		if recorder.captainAnnouncements() != 1 {
			t.Fatal("a vacated team must not announce a Captain")
		}
	}

	room.life.players[captain].state = stateActive
	server.refreshCaptains(room, 2)
	if room.CaptainSlot(1) != captain {
		t.Fatalf("the returning seat should hold the Captaincy again, got %d", room.CaptainSlot(1))
	}
	for _, recorder := range recorders {
		if recorder.captainAnnouncements() != 2 {
			t.Fatalf("the returning Captain was announced %d times, want twice", recorder.captainAnnouncements())
		}
	}
}
