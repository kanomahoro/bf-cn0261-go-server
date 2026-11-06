// Package admin serves the management console: a small bearer-token HTTP API and
// one embedded page. It reads live state through game.Server and the database
// through store; it never touches either one directly.
package admin

import (
	"context"
	"crypto/subtle"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/config"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/game"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/logging"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/store"
)

// sampleToken is the value this repository used to ship in config/server.json.
// It sits in the public commit history, so anyone who copied it into their own
// configuration has a token the world already knows; the console warns when it
// sees it. config/server.json now ships with the console disabled and no token.
const sampleToken = "547566ff61272b15eadf0c5021b83e78"

// Server is the management console.
type Server struct {
	cfg     config.Admin
	game    *game.Server
	db      *store.DB
	log     *slog.Logger
	version string
	cfgPath string
	// cfgMu serializes configuration writes: a second save must not interleave
	// with the read-merge-validate-replace sequence of the first.
	cfgMu sync.Mutex
	// authFails slows down token guessing; see ratelimit.go.
	authFails *failLimiter
}

// New builds the console.
func New(cfg config.Admin, gameServer *game.Server, db *store.DB, version, configPath string, log *slog.Logger) *Server {
	return &Server{
		cfg: cfg, game: gameServer, db: db, log: log, version: version,
		cfgPath: configPath, authFails: newFailLimiter(),
	}
}

// routes builds the console's handler table.
func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/overview", s.auth(s.overview))
	mux.HandleFunc("GET /api/players", s.auth(s.players))
	mux.HandleFunc("POST /api/players/kick", s.auth(s.kick))
	mux.HandleFunc("POST /api/players/ban", s.auth(s.ban))
	mux.HandleFunc("GET /api/rooms", s.auth(s.rooms))
	mux.HandleFunc("POST /api/rooms/disband", s.auth(s.disband))
	mux.HandleFunc("GET /api/logs", s.auth(s.logs))
	mux.HandleFunc("GET /api/search", s.auth(s.search))
	mux.HandleFunc("GET /api/bans", s.auth(s.bans))
	mux.HandleFunc("GET /api/results", s.auth(s.results))
	mux.HandleFunc("GET /api/results/detail", s.auth(s.resultDetail))
	mux.HandleFunc("GET /api/results.csv", s.auth(s.resultsCSV))
	mux.HandleFunc("POST /api/balance", s.auth(s.balance))
	mux.HandleFunc("POST /api/notice", s.auth(s.notice))
	mux.HandleFunc("GET /api/server", s.auth(s.serverInfo))
	mux.HandleFunc("GET /api/config", s.auth(s.getConfig))
	mux.HandleFunc("POST /api/config", s.auth(s.saveConfig))
	mux.HandleFunc("GET /", s.page)
	return mux
}

// Serve answers until the context is cancelled.
func (s *Server) Serve(ctx context.Context) error {
	// Configuration validation already refuses this, but an empty token would make
	// every request pass the constant-time comparison, so the console refuses to
	// start without one.
	if s.cfg.Token == "" {
		return errors.New("admin console needs a token")
	}
	if s.authFails == nil {
		// New sets this; a console built another way must still be limited rather
		// than silently accepting unlimited guesses.
		s.authFails = newFailLimiter()
	}
	address := net.JoinHostPort(s.cfg.ListenAddress, strconv.Itoa(int(s.cfg.ListenPort)))
	// Bind before announcing anything: an operator must never be pointed at a port
	// that is already taken by something else.
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	httpServer := &http.Server{
		Handler: s.headers(s.routes()),
		// The console is tiny, so every phase gets a bound: an exposed port must not
		// let a stalled client hold a connection open indefinitely.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdown)
	}()
	if s.cfg.Token == sampleToken {
		s.log.Warn("[Admin] admin.token is the sample value shipped in the repository; " +
			"replace it before the console is reachable by anyone else")
	}
	s.log.Info("[Admin] listening", "address", listener.Addr().String())
	s.printAddress(listener.Addr().String())
	if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// headers sets the three response headers that cost nothing and stop the console
// from being framed, sniffed or leaking its URL as a referrer.
func (s *Server) headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		// The page is one file with inline script and style, so those two have to be
		// allowed; connect-src keeps anything injected from talking to another host.
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		next.ServeHTTP(w, r)
	})
}

// printAddress puts the console URL on stdout, where an operator starting the
// server by hand will see it without reading the log file.
func (s *Server) printAddress(address string) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	fmt.Println("--------------------------------------------------------------")
	fmt.Println(" Admin console : http://" + net.JoinHostPort(host, port) + "/")
	fmt.Println(" Token         : admin.token in " + s.cfgPath)
	fmt.Println("--------------------------------------------------------------")
}

// auth requires the configured bearer token in the Authorization header. The
// token is never accepted from the URL: it would end up in browser history, in
// logs and in any copy of the link.
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return s.guard(next)
}

func (s *Server) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// A panic here must not reach net/http's default handler: this runs beside
		// the game server, and the operator has to see which call went wrong.
		defer func() {
			if recovered := recover(); recovered != nil {
				s.log.Error("[Admin] handler panic", "path", r.URL.Path, "panic", recovered)
				writeError(w, http.StatusInternalServerError, "内部错误")
			}
		}()
		address := clientAddress(r.RemoteAddr)
		now := time.Now()
		if s.authFails != nil && s.authFails.blocked(address, now) {
			writeError(w, http.StatusTooManyRequests, "尝试次数过多，请一分钟后再试")
			return
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.Token)) != 1 {
			if s.authFails != nil {
				s.authFails.fail(address, now)
			}
			writeError(w, http.StatusUnauthorized, "token 无效")
			return
		}
		if s.authFails != nil {
			s.authFails.reset(address, now)
		}
		next(w, r)
	}
}

func (s *Server) page(w http.ResponseWriter, r *http.Request) {
	// The mux sends every unmatched GET here, so anything but the console's own
	// path (a browser's /favicon.ico probe, for one) answers 404 instead of the
	// page with a 200.
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(indexHTML)
}

func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	overview := s.game.Snapshot()
	pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	latency, err := s.db.Ping(pingCtx)
	dbOk := err == nil
	writeJSON(w, map[string]any{
		"overview": overview,
		"db": map[string]any{
			"ok":        dbOk,
			"latencyMs": latency.Milliseconds(),
			"error":     errorText(err),
		},
		"server": map[string]any{
			"version":    s.version,
			"goVersion":  runtime.Version(),
			"configPath": s.cfgPath,
			"commit":     commitHash(),
		},
	})
}

func (s *Server) players(w http.ResponseWriter, r *http.Request) {
	limit, offset := pageArgs(r, 20)
	items, total, used := window(s.game.AdminPlayers(), limit, offset)
	writeJSON(w, paged{Total: total, Offset: used, Limit: limit, Items: items})
}

func (s *Server) rooms(w http.ResponseWriter, r *http.Request) {
	limit, offset := pageArgs(r, 20)
	items, total, used := window(s.game.AdminRooms(), limit, offset)
	writeJSON(w, paged{Total: total, Offset: used, Limit: limit, Items: items})
}

// window takes one page out of an already sorted in-memory list, clamping the
// offset onto the last page the way the database-backed lists do.
func window[T any](items []T, limit, offset int) ([]T, int, int) {
	total := len(items)
	if total == 0 {
		return []T{}, 0, 0
	}
	if limit <= 0 {
		limit = total
	}
	if offset > total-1 {
		offset = total - 1
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return items[offset:end], total, offset
}

func (s *Server) kick(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccountID uint32 `json:"accountId"`
	}
	if !decode(w, r, &body) {
		return
	}
	if !s.game.AdminKickAccount(body.AccountID) {
		writeError(w, http.StatusNotFound, "该账号不在线")
		return
	}
	s.log.Info("[Admin] kick", "account", body.AccountID)
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) ban(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccountID uint32 `json:"accountId"`
		Disabled  bool   `json:"disabled"`
	}
	if !decode(w, r, &body) {
		return
	}
	kicked, err := s.game.AdminSetBan(body.AccountID, body.Disabled)
	if err != nil {
		writeError(w, http.StatusBadRequest, storeMessage(err, "账号不存在"))
		return
	}
	writeJSON(w, map[string]any{"ok": true, "kicked": kicked})
}

func (s *Server) disband(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RoomID uint16 `json:"roomId"`
	}
	if !decode(w, r, &body) {
		return
	}
	members, ok := s.game.AdminDisbandRoom(body.RoomID)
	if !ok {
		writeError(w, http.StatusNotFound, "房间不存在")
		return
	}
	writeJSON(w, map[string]any{"ok": true, "members": members})
}

// paged is the shape every list endpoint answers when it supports a pager: the
// total number of matches the filter found, the window the server actually used
// (it clamps an offset past the end), and the rows.
type paged struct {
	Total  int `json:"total"`
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	Items  any `json:"items"`
}

// pageArgs reads limit/offset from a request and clamps them to a sane window.
func pageArgs(r *http.Request, defaultLimit int) (limit, offset int) {
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// maxPageSize bounds one page of any list the console shows. The log pane asks for
// 300 lines by default, so the bound has to sit above that.
const maxPageSize = 500

func (s *Server) logs(w http.ResponseWriter, r *http.Request) {
	limit, offset := pageArgs(r, 300)
	items, total := logging.Recent(limit, offset, r.URL.Query().Get("q"))
	writeJSON(w, paged{Total: total, Offset: offset, Limit: limit, Items: items})
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	term := strings.TrimSpace(r.URL.Query().Get("q"))
	if term == "" {
		writeJSON(w, paged{Items: []map[string]any{}})
		return
	}
	limit, offset := pageArgs(r, 20)
	found, total, err := s.game.AdminSearchCharacters(term, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]map[string]any, 0, len(found))
	for _, entry := range found {
		out = append(out, map[string]any{
			"accountId":     entry.AccountID,
			"accountName":   entry.AccountName,
			"disabled":      entry.Disabled,
			"characterId":   entry.CharacterID,
			"nickname":      entry.Nickname,
			"characterType": entry.CharacterType,
			"level":         s.game.LevelOf(entry.Experience),
			"experience":    entry.Experience,
			"lucci":         entry.Lucci,
			"cash":          entry.Cash,
			"createdAt":     entry.CreatedAt,
		})
	}
	writeJSON(w, paged{Total: total, Offset: offset, Limit: limit, Items: out})
}

func (s *Server) bans(w http.ResponseWriter, r *http.Request) {
	limit, offset := pageArgs(r, 20)
	bans, total, err := s.db.ListBans(limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, paged{Total: total, Offset: offset, Limit: limit, Items: bans})
}

func (s *Server) results(w http.ResponseWriter, r *http.Request) {
	mode, _ := strconv.Atoi(r.URL.Query().Get("mode"))
	limit, offset := pageArgs(r, 50)
	// The results query keeps its own, smaller ceiling: a century of matches does
	// not need to arrive in one page.
	if limit > 200 {
		limit = 200
	}
	rows, total, err := s.db.ListResults(uint32(mode), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, paged{Total: total, Offset: offset, Limit: limit, Items: rows})
}

func (s *Server) resultDetail(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少 id")
		return
	}
	players, err := s.db.ResultPlayers(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"id": id, "players": players})
}

func (s *Server) resultsCSV(w http.ResponseWriter, r *http.Request) {
	mode, _ := strconv.Atoi(r.URL.Query().Get("mode"))
	rows, total, err := s.db.ListResults(uint32(mode), 200, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if total > len(rows) {
		// One page is what the console can show anyway; the CSV states the cap.
		s.log.Info("[Admin] csv capped", "rows", len(rows), "total", total)
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="matches.csv"`)
	writer := csv.NewWriter(w)
	defer writer.Flush()
	_ = writer.Write([]string{"id", "completed_at", "mode", "map", "first_score", "second_score", "winner", "end_reason"})
	for _, row := range rows {
		_ = writer.Write([]string{
			row.ID, row.CompletedAt.Format("2006-01-02 15:04:05"), strconv.Itoa(int(row.Mode)),
			row.MapName, strconv.Itoa(int(row.FirstScore)), strconv.Itoa(int(row.SecondScore)),
			strconv.Itoa(int(row.Winner)), strconv.Itoa(int(row.EndReason)),
		})
	}
}

func (s *Server) balance(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CharacterID uint32 `json:"characterId"`
		Lucci       uint32 `json:"lucci"`
		Cash        uint32 `json:"cash"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.CharacterID == 0 {
		writeError(w, http.StatusBadRequest, "缺少角色 ID")
		return
	}
	if body.Lucci > store.MaxLucci {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("金币上限 %d", store.MaxLucci))
		return
	}
	if body.Cash > store.MaxCash {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("点券上限 %d", store.MaxCash))
		return
	}
	if err := s.game.AdminSetBalance(body.CharacterID, body.Lucci, body.Cash); err != nil {
		writeError(w, http.StatusBadRequest, storeMessage(err, "角色不存在（没有余额记录）"))
		return
	}
	writeJSON(w, map[string]any{"ok": true, "note": "写库完成；玩家重新登录、购买或打完一局后生效"})
}

func (s *Server) notice(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if !decode(w, r, &body) {
		return
	}
	text := strings.TrimSpace(body.Text)
	if text == "" {
		writeError(w, http.StatusBadRequest, "公告内容为空")
		return
	}
	delivered, err := s.game.BroadcastNotice(text)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "delivered": delivered})
}

func (s *Server) serverInfo(w http.ResponseWriter, r *http.Request) {
	versions, err := s.db.SchemaVersions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{
		"version":    s.version,
		"commit":     commitHash(),
		"goVersion":  runtime.Version(),
		"configPath": s.cfgPath,
		"pool":       s.db.PoolStats(),
		"migrations": versions,
		"uptime":     s.game.Snapshot().UptimeSeconds,
	})
}

// commitHash pulls the VCS revision Go embeds at build time.
func commitHash() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}
	return ""
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "请求体不是合法 JSON")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// storeMessage turns the store's sentinel error into something an operator can
// act on; anything else keeps the original text, which usually names the column.
func storeMessage(err error, missing string) string {
	if errors.Is(err, store.ErrNotFound) {
		return missing
	}
	return err.Error()
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
