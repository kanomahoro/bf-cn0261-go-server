package config

import "testing"

// TestValidateAdminToken covers what the console's bearer token has to survive:
// long enough, not a placeholder, and varied enough that guessing is pointless.
func TestValidateAdminToken(t *testing.T) {
	good := []string{"547566ff61272b15eadf0c5021b83e78", "hunter2-is-not-enough", "a1b2c3d4e5f6g7h8"}
	for _, token := range good {
		if err := ValidateAdminToken(token); err != nil {
			t.Fatalf("good token %q rejected: %v", token, err)
		}
	}
	bad := []string{
		"",                     // empty
		"short",                // too short
		"aaaaaaaaaaaaaaaaaa",   // one character repeated
		"1234567890123456",     // sequential digits
		"changemechangeme",     // placeholder
		"changeme12345678",     // placeholder with digits glued on
		"passwordpassword",     // placeholder
		"my-qwerty-token-2026", // placeholder inside a longer value
		"abcabcabcabcabcabc",   // six distinct characters only
	}
	for _, token := range bad {
		if err := ValidateAdminToken(token); err == nil {
			t.Fatalf("weak token %q accepted", token)
		}
	}
}
