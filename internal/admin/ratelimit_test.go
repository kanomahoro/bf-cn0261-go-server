package admin

import (
	"testing"
	"time"
)

// TestFailLimiterBansAfterTooManyAttempts pins the guessing brake: ten rejects in
// a minute stop the address for one minute, and a success clears the count.
func TestFailLimiterBansAfterTooManyAttempts(t *testing.T) {
	limiter := newFailLimiter()
	start := time.Now()
	for i := 0; i < authFailLimit; i++ {
		if limiter.blocked("10.0.0.1", start) {
			t.Fatalf("blocked too early at attempt %d", i)
		}
		limiter.fail("10.0.0.1", start)
	}
	if !limiter.blocked("10.0.0.1", start) {
		t.Fatalf("not blocked after %d failures", authFailLimit)
	}
	// Another address is unaffected.
	if limiter.blocked("10.0.0.2", start) {
		t.Fatalf("an unrelated address was blocked")
	}
	// The ban lifts after its window and the count starts over.
	if limiter.blocked("10.0.0.1", start.Add(authFailBan+time.Second)) {
		t.Fatalf("still blocked after the ban window")
	}
	// A success clears pending failures.
	limiter.fail("10.0.0.3", start)
	limiter.reset("10.0.0.3", start)
	if limiter.blocked("10.0.0.3", start) {
		t.Fatalf("reset did not clear the failures")
	}
}
