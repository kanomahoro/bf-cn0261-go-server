package net

import (
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/config"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
)

// newTestRelayConn builds a relay connection over an in-memory pipe pair.
func newTestRelayConn(t *testing.T) (*relayConn, net.Conn) {
	t.Helper()
	server, client := net.Pipe()
	t.Cleanup(func() {
		_ = server.Close()
		_ = client.Close()
	})
	return &relayConn{conn: server}, client
}

// TestRelayRegistrationTakesOverARebuiltTransport pins the bomb mission's second
// sub-round: the client re-dials the relay and registers the same key again, and
// the server has to replace the stale transport instead of rejecting the new one
// (a rejected registration leaves the sub-round without peer traffic).
func TestRelayRegistrationTakesOverARebuiltTransport(t *testing.T) {
	relay := NewRelay(config.Relay{}, slog.Default())
	var key protocol.RelayKey
	key[0] = 7
	relay.Register(key, 3, 1, "127.0.0.1")

	first, _ := newTestRelayConn(t)
	registration := protocol.RelayRegistration{Key: key, Slot: 1}
	if !relay.join(first, registration) {
		t.Fatal("the first registration was rejected")
	}
	if !relay.HasSlot(3, 1) {
		t.Fatal("the first registration did not attach a transport")
	}

	second, _ := newTestRelayConn(t)
	if !relay.join(second, registration) {
		t.Fatal("a client rebuilding its scene could not re-register")
	}
	if !relay.HasSlot(3, 1) {
		t.Fatal("the rebuilt transport is not attached")
	}
	// The replaced pipe is closed, so a read never blocks for a byte.
	buffer := make([]byte, 1)
	if _, err := first.conn.Read(buffer); err == nil {
		t.Fatal("the replaced transport stayed open")
	}
}

// TestRelayRefreshKeepsTheReservationUsable checks a series' key: refreshing it
// must extend the expiry without clearing the binding, so the next sub-round's
// registration still resolves (it was refused as "unknown relay key" once the
// two-minute reservation expired mid-series, 2026-11-10).
func TestRelayRefreshKeepsTheReservationUsable(t *testing.T) {
	relay := NewRelay(config.Relay{}, slog.Default())
	var key protocol.RelayKey
	key[0] = 9
	relay.Register(key, 5, 0, "127.0.0.1")

	first, _ := newTestRelayConn(t)
	if !relay.join(first, protocol.RelayRegistration{Key: key, Slot: 0}) {
		t.Fatal("the first registration was rejected")
	}
	// Pretend the reservation ran out.
	relay.mu.Lock()
	reservation := relay.reservations[key]
	reservation.expiresAt = time.Now().Add(-time.Minute)
	relay.reservations[key] = reservation
	relay.mu.Unlock()

	relay.Refresh(key)

	second, _ := newTestRelayConn(t)
	if !relay.join(second, protocol.RelayRegistration{Key: key, Slot: 0}) {
		t.Fatal("the refreshed key could not be registered again")
	}
	if !relay.HasSlot(5, 0) {
		t.Fatal("the refreshed registration did not attach")
	}
}
