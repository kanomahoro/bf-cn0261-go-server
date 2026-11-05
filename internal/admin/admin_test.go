package admin

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/config"
)

func testServer(token string) *Server {
	return &Server{
		cfg:       config.Admin{Token: token},
		log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		authFails: newFailLimiter(),
	}
}

// TestAuthRequiresTheToken pins the one rule the console must never lose: every
// API call carries the configured bearer token.
func TestAuthRequiresTheToken(t *testing.T) {
	handler := testServer("0123456789abcdef").auth(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cases := []struct {
		name   string
		header string
		query  string
		want   int
	}{
		{"no token", "", "", http.StatusUnauthorized},
		{"wrong token", "Bearer nope", "", http.StatusUnauthorized},
		{"header token", "Bearer 0123456789abcdef", "", http.StatusOK},
		{"query token", "", "token=0123456789abcdef", http.StatusUnauthorized},
	}
	for _, testCase := range cases {
		request := httptest.NewRequest(http.MethodGet, "/api/overview"+querySuffix(testCase.query), nil)
		if testCase.header != "" {
			request.Header.Set("Authorization", testCase.header)
		}
		recorder := httptest.NewRecorder()
		handler(recorder, request)
		if recorder.Code != testCase.want {
			t.Fatalf("%s: status = %d, want %d", testCase.name, recorder.Code, testCase.want)
		}
	}
}

func querySuffix(query string) string {
	if query == "" {
		return ""
	}
	return "?" + query
}

// TestPageIsEmbedded makes sure the console ships inside the binary.
func TestPageIsEmbedded(t *testing.T) {
	recorder := httptest.NewRecorder()
	testServer("0123456789abcdef").page(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "管理后台") {
		t.Fatalf("page does not look like the console")
	}
}

// TestRoutesGuardTheApi checks the table itself: the page is open, every API
// route without a token is refused before a handler can touch live state.
func TestRoutesGuardTheApi(t *testing.T) {
	mux := testServer("0123456789abcdef").routes()
	for _, path := range []string{"/api/overview", "/api/players", "/api/rooms", "/api/logs", "/api/results.csv"} {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s without a token: status = %d, want 401", path, recorder.Code)
		}
	}
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("page: status = %d", recorder.Code)
	}
}

// TestSaveConfigValidatesBeforeWriting pins the rule the console promises: a
// rejected value must leave the file exactly as it was.
func TestSaveConfigValidatesBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.json")
	original, err := os.ReadFile("../../config/server.json")
	if err != nil {
		t.Fatalf("read sample config: %v", err)
	}
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("write sample config: %v", err)
	}
	server := testServer("0123456789abcdef")
	server.cfgPath = path

	// A value the validator rejects must not reach the file.
	bad := httptest.NewRequest(http.MethodPost, "/api/config",
		strings.NewReader(`{"network.listenPort":70000}`))
	bad.Header.Set("Authorization", "Bearer 0123456789abcdef")
	recorder := httptest.NewRecorder()
	server.routes().ServeHTTP(recorder, bad)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid port: status = %d, want 400 (%s)", recorder.Code, recorder.Body.String())
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(after, original) {
		t.Fatalf("a rejected save rewrote the file")
	}

	// A valid change is written and can be loaded back.
	good := httptest.NewRequest(http.MethodPost, "/api/config",
		strings.NewReader(`{"zombie.humanTurnMs":13000,"relay.advertisedAddress":"203.0.113.7"}`))
	good.Header.Set("Authorization", "Bearer 0123456789abcdef")
	recorder = httptest.NewRecorder()
	server.routes().ServeHTTP(recorder, good)
	if recorder.Code != http.StatusOK {
		t.Fatalf("valid save: status = %d (%s)", recorder.Code, recorder.Body.String())
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("saved file does not load: %v", err)
	}
	if cfg.Zombie.HumanTurnMs != 13000 || cfg.Relay.AdvertisedAddress != "203.0.113.7" {
		t.Fatalf("saved values = %d / %q", cfg.Zombie.HumanTurnMs, cfg.Relay.AdvertisedAddress)
	}
	// Everything the console does not edit must survive the rewrite.
	if cfg.Database.Username == "" || cfg.GameMapsFile == "" {
		t.Fatalf("untouched fields were lost: %+v", cfg)
	}
	// A field outside the whitelist is refused.
	forbidden := httptest.NewRequest(http.MethodPost, "/api/config",
		strings.NewReader(`{"database.host":"evil"}`))
	forbidden.Header.Set("Authorization", "Bearer 0123456789abcdef")
	recorder = httptest.NewRecorder()
	server.routes().ServeHTTP(recorder, forbidden)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("forbidden key: status = %d, want 400", recorder.Code)
	}
}

// TestSecurityHeaders pins the three headers the console promises plus the CSP
// that keeps an injected script from talking to another host.
func TestSecurityHeaders(t *testing.T) {
	server := testServer("0123456789abcdef")
	handler := server.headers(server.routes())
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	} {
		if got := recorder.Header().Get(header); got != want {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}
	if csp := recorder.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "connect-src 'self'") {
		t.Fatalf("CSP = %q, want it to pin connect-src", csp)
	}
}

// freePort reserves a port the OS just handed out, so a test can bind a real
// listener without guessing.
func freePort(t *testing.T) uint16 {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return uint16(port)
}

// TestServeOverHTTP exercises the real listener: page served, API guarded, the
// guessing brake attached, and a clean stop when the context ends.
func TestServeOverHTTP(t *testing.T) {
	port := freePort(t)
	server := testServer("0123456789abcdef")
	server.cfg.ListenAddress = "127.0.0.1"
	server.cfg.ListenPort = port

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()

	base := "http://127.0.0.1:" + strconv.Itoa(int(port))
	var response *http.Response
	for attempt := 0; attempt < 50; attempt++ {
		response, _ = http.Get(base + "/")
		if response != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if response == nil {
		t.Fatalf("console never answered")
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("page status = %d", response.StatusCode)
	}
	if response.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("headers missing on the real listener")
	}

	// Ten wrong tokens from the same address must trip the brake: the tenth is
	// still answered with 401, the eleventh is refused before it is checked.
	client := &http.Client{}
	blocked := false
	for attempt := 0; attempt < authFailLimit+1; attempt++ {
		request, _ := http.NewRequest(http.MethodGet, base+"/api/players", nil)
		request.Header.Set("Authorization", "Bearer wrong")
		reply, err := client.Do(request)
		if err != nil {
			t.Fatalf("attempt %d: %v", attempt, err)
		}
		reply.Body.Close()
		if reply.StatusCode == http.StatusTooManyRequests {
			blocked = true
			break
		}
	}
	if !blocked {
		t.Fatalf("guessing was never braked")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve returned %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("serve did not stop")
	}
}

// TestBalanceCapsRejectBothCurrencies pins the operator-facing ceiling: the
// console writes at most one hundred million of either currency, even though the
// columns could hold more.
func TestBalanceCapsRejectBothCurrencies(t *testing.T) {
	server := testServer("0123456789abcdef")
	for name, body := range map[string]string{
		"lucci over cap": `{"characterId":7,"lucci":100000001,"cash":0}`,
		"cash over cap":  `{"characterId":7,"lucci":0,"cash":100000001}`,
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/balance", strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer 0123456789abcdef")
		recorder := httptest.NewRecorder()
		server.routes().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400 (%s)", name, recorder.Code, recorder.Body.String())
		}
	}
}
