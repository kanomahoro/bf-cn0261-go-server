package admin

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
