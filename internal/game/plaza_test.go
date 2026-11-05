package game

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/features"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/store"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/world"
)

type recordingSender struct {
	sent []protocol.Packet
	fail bool
}

func (r *recordingSender) Send(packet protocol.Packet) error {
	if r.fail {
		return errors.New("socket is gone")
	}
	r.sent = append(r.sent, packet)
	return nil
}

func plazaTestServer(t *testing.T) *Server {
	t.Helper()
	progression, err := world.LoadProgression("../../config/progression.json")
	if err != nil {
		t.Fatalf("load progression: %v", err)
	}
	return &Server{
		log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		plazas:   NewPlazaDirectory(),
		channels: NewChannelDirectory(nil),
		rooms:    map[uint16]*Room{},
		online:   map[uint32]*Session{},
		svc:      features.NewService(nil, progression),
		prog:     progression,
	}
}

func plazaTestSession(id uint64, characterID, accountID uint32, send Sender) *Session {
	session := &Session{ID: id, send: send, slot: -1}
	session.Character = store.Character{ID: characterID, AccountID: accountID, Nickname: "tester", CharacterType: 1}
	session.channelType = lobbyChannelType
	session.channelIndex = 0
	session.Account = store.Account{ID: accountID, Name: "tester"}
	return session
}

// TestEnterPlazaRollsBackWhenTheEntryPacketFails pins the bug a player reported:
// leaving a match into a plaza whose first packet could not be delivered left the
// seat behind, so the character stayed in everyone's list and could not come back.
func TestEnterPlazaRollsBackWhenTheEntryPacketFails(t *testing.T) {
	server := plazaTestServer(t)
	session := plazaTestSession(1, 7, 3, &recordingSender{fail: true})

	if err := server.enterPlaza(session); err == nil {
		t.Fatalf("entry reported success although the socket was gone")
	}
	if holder := server.plazas.characterHolder(7, 0); holder != nil {
		t.Fatalf("a seat survived a failed entry")
	}
	if session.inPlaza {
		t.Fatalf("session still believes it is in the plaza")
	}
}

// TestEnterPlazaReplacesASeatWhoseSessionIsGone covers the state an older build
// could leave behind: the seat is held by a session that is not logged in any
// more, and it must not lock the player out.
func TestEnterPlazaReplacesASeatWhoseSessionIsGone(t *testing.T) {
	server := plazaTestServer(t)
	stale := plazaTestSession(9, 7, 3, nil)
	if _, _, err := server.plazas.enter(stale, lobbyChannelType, 0); err != nil {
		t.Fatalf("seat the stale session: %v", err)
	}

	live := plazaTestSession(10, 7, 3, &recordingSender{})
	if err := server.enterPlaza(live); err != nil {
		t.Fatalf("live session could not enter: %v", err)
	}
	if holder := server.plazas.characterHolder(7, live.ID); holder != nil {
		t.Fatalf("the stale seat was not replaced")
	}
	if !live.inPlaza {
		t.Fatalf("live session is not marked as seated")
	}
}

// TestEnterPlazaRefusesALiveDuplicate keeps the guard that matters: a character
// whose session is still the account's live one cannot be seated twice.
func TestEnterPlazaRefusesALiveDuplicate(t *testing.T) {
	server := plazaTestServer(t)
	first := plazaTestSession(11, 7, 3, &recordingSender{})
	server.online[3] = first
	if err := server.enterPlaza(first); err != nil {
		t.Fatalf("first entry: %v", err)
	}
	second := plazaTestSession(12, 7, 3, &recordingSender{})
	if err := server.enterPlaza(second); err == nil {
		t.Fatalf("a second live session was seated on the same character")
	}
}
