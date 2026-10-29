package game

import (
	"sync"
	"time"
)

// pingProbeExpiry is how long a probe stays valid.
const pingProbeExpiry = 5 * time.Second

// pingRequestCooldown throttles repeated probes from one session.
const pingRequestCooldown = 500 * time.Millisecond

type pingProbe struct {
	expiresAt     time.Time
	nextAt        time.Time
	requesterSlot uint32
	expected      []uint64
}

// PingTracker correlates one requester with the responders of one probe. It is
// telemetry only: it never changes game rules or picks hosts.
type PingTracker struct {
	mu     sync.Mutex
	probes map[uint64]*pingProbe
}

// NewPingTracker builds a tracker.
func NewPingTracker() *PingTracker {
	return &PingTracker{probes: map[uint64]*pingProbe{}}
}

// Begin records a probe from requester at slot; members must map slot to session.
func (t *PingTracker) Begin(requester uint64, slot uint32, members []uint64, now time.Time) bool {
	if requester == 0 || len(members) > 32 || int(slot) >= len(members) ||
		members[slot] != requester {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for key, probe := range t.probes {
		if !probe.expiresAt.After(now) {
			delete(t.probes, key)
		}
	}
	if previous, ok := t.probes[requester]; ok && previous.nextAt.After(now) {
		return false
	}
	expected := append([]uint64(nil), members...)
	t.probes[requester] = &pingProbe{
		expiresAt:     now.Add(pingProbeExpiry),
		nextAt:        now.Add(pingRequestCooldown),
		requesterSlot: slot,
		expected:      expected,
	}
	return true
}

// Complete validates a responder's report and returns the requester session.
func (t *PingTracker) Complete(responder uint64, responderSlot, requesterSlot uint32, members []uint64, now time.Time) (uint64, bool) {
	if responder == 0 || len(members) > 32 || int(responderSlot) >= len(members) ||
		int(requesterSlot) >= len(members) || members[responderSlot] != responder ||
		members[requesterSlot] == 0 {
		return 0, false
	}
	requester := members[requesterSlot]
	t.mu.Lock()
	defer t.mu.Unlock()
	probe, ok := t.probes[requester]
	if !ok {
		return 0, false
	}
	if !probe.expiresAt.After(now) {
		delete(t.probes, requester)
		return 0, false
	}
	if probe.requesterSlot != requesterSlot || int(responderSlot) >= len(probe.expected) ||
		probe.expected[responderSlot] != responder {
		return 0, false
	}
	probe.expected[responderSlot] = 0
	return requester, true
}

// Remove drops a session from every probe.
func (t *PingTracker) Remove(sessionID uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.probes, sessionID)
	for _, probe := range t.probes {
		for index, expected := range probe.expected {
			if expected == sessionID {
				probe.expected[index] = 0
			}
		}
	}
}

// Clear drops all probes.
func (t *PingTracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.probes = map[uint64]*pingProbe{}
}
