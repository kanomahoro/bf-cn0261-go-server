package game

import (
	"io"
	"log/slog"
	"testing"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/store"
)

type fakeConn struct {
	sent   int
	closed bool
	reason string
}

func (f *fakeConn) Send(protocol.Packet) error { f.sent++; return nil }
func (f *fakeConn) Close(reason string) error  { f.closed, f.reason = true, reason; return nil }

func newTestServer() *Server {
	return &Server{
		log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		online:   map[uint32]*Session{},
		channels: NewChannelDirectory(nil),
		plazas:   NewPlazaDirectory(),
		rooms:    map[uint16]*Room{},
	}
}

func sessionFor(id uint64, accountID uint32, conn *fakeConn) *Session {
	s := &Session{ID: id, send: conn}
	s.Account = store.Account{ID: accountID}
	return s
}

// TestClaimAccountTakesOver pins the duplicate-login rule: the second login owns
// the account, the first is detached and its connection closed with a reason.
func TestClaimAccountTakesOver(t *testing.T) {
	s := newTestServer()
	firstConn, secondConn := &fakeConn{}, &fakeConn{}
	first := sessionFor(1, 7, firstConn)
	second := sessionFor(2, 7, secondConn)

	s.claimAccount(first, 7)
	if s.online[7] != first {
		t.Fatalf("first claim did not register the session")
	}
	s.claimAccount(second, 7)
	if s.online[7] != second {
		t.Fatalf("second claim did not take the account")
	}
	if !firstConn.closed || firstConn.reason != "duplicate login" {
		t.Fatalf("previous connection = %+v, want it closed for the duplicate login", firstConn)
	}
	if secondConn.closed {
		t.Fatalf("the new connection must stay open")
	}
	// Detaching the old session must not free the account the new one holds.
	s.Disconnect(first)
	if s.online[7] != second {
		t.Fatalf("detaching the old session removed the new owner")
	}
	// The new owner's own disconnect frees the account.
	s.Disconnect(second)
	if _, ok := s.online[7]; ok {
		t.Fatalf("account still held after its owner disconnected")
	}
}

// TestClaimAccountIgnoresRepeat keeps a client that logs in twice on one
// connection from closing itself.
func TestClaimAccountIgnoresRepeat(t *testing.T) {
	s := newTestServer()
	conn := &fakeConn{}
	session := sessionFor(1, 9, conn)
	s.claimAccount(session, 9)
	s.claimAccount(session, 9)
	if conn.closed {
		t.Fatalf("a repeated login on the same session closed its own connection")
	}
}
