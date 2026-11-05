package admin

import (
	"net"
	"sync"
	"time"
)

// AuthFailures bounds how fast a token can be guessed. Loopback needs none of
// this, but a console someone exposes on a network does, and the cost of a small
// counter is nothing.
const (
	authFailWindow = time.Minute
	authFailLimit  = 10
	authFailBan    = time.Minute
	// authFailEntries bounds the counter map itself.
	authFailEntries = 1024
)

// failLimiter counts rejected tokens per client address.
type failLimiter struct {
	mu      sync.Mutex
	entries map[string]*failEntry
}

type failEntry struct {
	count    int
	first    time.Time
	bannedTo time.Time
}

func newFailLimiter() *failLimiter {
	return &failLimiter{entries: map[string]*failEntry{}}
}

// blocked reports whether the address has to wait before trying again.
func (l *failLimiter) blocked(address string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[address]
	if entry == nil {
		return false
	}
	if now.Before(entry.bannedTo) {
		return true
	}
	// A ban that has just run out starts the count over.
	if !entry.bannedTo.IsZero() {
		delete(l.entries, address)
	}
	return false
}

// fail records one rejected attempt and starts a ban once the window is full.
func (l *failLimiter) fail(address string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[address]
	if entry == nil || now.Sub(entry.first) > authFailWindow {
		if len(l.entries) > authFailEntries {
			l.pruneLocked(now)
		}
		l.entries[address] = &failEntry{count: 1, first: now}
		return
	}
	entry.count++
	if entry.count >= authFailLimit {
		entry.bannedTo = now.Add(authFailBan)
	}
}

// pruneLocked drops entries whose window and ban are both over. Failures call it
// as well as successes: a flood of failed guesses must not grow the map.
func (l *failLimiter) pruneLocked(now time.Time) {
	for key, entry := range l.entries {
		if now.Sub(entry.first) > authFailWindow && now.After(entry.bannedTo) {
			delete(l.entries, key)
		}
	}
}

// reset clears the address after a successful attempt and prunes stale entries so
// the map cannot grow with every client that ever guessed.
func (l *failLimiter) reset(address string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, address)
	l.pruneLocked(now)
}

// clientAddress is the connection's own address; proxy headers are deliberately
// ignored, since anyone can set them.
func clientAddress(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}
