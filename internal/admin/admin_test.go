package admin

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/config"
)

func testServer(token string) *Server {
	return &Server{
		cfg: config.Admin{Token: token},
		log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// TestAuthRequiresTheToken pins the one rule the console must never lose: every
// API call carries the configured bearer token.
func TestAuthRequiresTheToken(t *testing.T) {
	handler := testServer("0123456789abcdef").auth(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	urlHandler := testServer("0123456789abcdef").authURL(func(w http.ResponseWriter, _ *http.Request) {
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
		{"query token", "", "token=0123456789abcdef", http.StatusOK},
	}
	for _, testCase := range cases {
		request := httptest.NewRequest(http.MethodGet, "/api/overview"+querySuffix(testCase.query), nil)
		if testCase.header != "" {
			request.Header.Set("Authorization", testCase.header)
		}
		recorder := httptest.NewRecorder()
		handler(recorder, request)
		want := testCase.want
		if testCase.query != "" {
			// A query token is only accepted where the route opts in (the CSV).
			want = http.StatusUnauthorized
		}
		if recorder.Code != want {
			t.Fatalf("%s: status = %d, want %d", testCase.name, recorder.Code, want)
		}
	}

	// The CSV route is the one that opts in.
	csvRequest := httptest.NewRequest(http.MethodGet, "/api/results.csv?token=0123456789abcdef", nil)
	csvRecorder := httptest.NewRecorder()
	urlHandler(csvRecorder, csvRequest)
	if csvRecorder.Code != http.StatusOK {
		t.Fatalf("csv query token: status = %d, want 200", csvRecorder.Code)
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
