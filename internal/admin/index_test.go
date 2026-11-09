package admin

import (
	"regexp"
	"testing"
)

// Every list renders its controls into <key>-pager, so each key the page passes
// to pager() must own an element with that id. The page shipped for a while with
// `log-pager` and `res-pager` against the keys `logs` and `results`: pager() found
// no host, returned early, and those two lists silently had no controls at all.
func TestPagerHostsMatchTheirKeys(t *testing.T) {
	keys := regexp.MustCompile(`pager\("([a-z]+)"`).FindAllStringSubmatch(string(indexHTML), -1)
	if len(keys) == 0 {
		t.Fatal("the page calls pager() nowhere; the pattern in this test is stale")
	}
	hosts := map[string]bool{}
	for _, match := range regexp.MustCompile(`id="([a-z]+)-pager"`).FindAllStringSubmatch(string(indexHTML), -1) {
		hosts[match[1]] = true
	}
	for _, match := range keys {
		if !hosts[match[1]] {
			t.Fatalf("pager(%q) has no %s-pager element, so that list shows no controls", match[1], match[1])
		}
	}
	// The same keys must exist in the page state, which pager() reads first.
	for _, match := range keys {
		if !regexp.MustCompile(`\b` + match[1] + `: \{`).Match(indexHTML) {
			t.Fatalf("pager(%q) has no PAGES entry", match[1])
		}
	}
}
