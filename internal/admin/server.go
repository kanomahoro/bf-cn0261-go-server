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
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/config"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/game"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/logging"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/store"
)

// Server is the management console.
type Server struct {
	cfg     config.Admin
	game    *game.Server
	db      *store.DB
	log     *slog.Logger
	version string
	cfgPath string
}

// New builds the console.
func New(cfg config.Admin, gameServer *game.Server, db *store.DB, version, configPath string, log *slog.Logger) *Server {
	return &Server{cfg: cfg, game: gameServer, db: db, log: log, version: version, cfgPath: configPath}
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
	mux.HandleFunc("GET /api/results", s.auth(s.results))
	mux.HandleFunc("GET /api/results/detail", s.auth(s.resultDetail))
	mux.HandleFunc("GET /api/results.csv", s.auth(s.resultsCSV))
	mux.HandleFunc("POST /api/balance", s.auth(s.balance))
	mux.HandleFunc("POST /api/notice", s.auth(s.notice))
	mux.HandleFunc("GET /api/server", s.auth(s.serverInfo))
	mux.HandleFunc("GET /", s.page)
	return mux
}

// Serve answers until the context is cancelled.
func (s *Server) Serve(ctx context.Context) error {
	mux := s.routes()

	httpServer := &http.Server{
		Addr:              net.JoinHostPort(s.cfg.ListenAddress, strconv.Itoa(int(s.cfg.ListenPort))),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdown)
	}()
	s.log.Info("[Admin] listening", "address", httpServer.Addr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// auth requires the configured bearer token on every API call.
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			token = r.URL.Query().Get("token")
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.Token)) != 1 {
			writeError(w, http.StatusUnauthorized, "token 无效")
			return
		}
		next(w, r)
	}
}

func (s *Server) page(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, s.game.AdminPlayers())
}

func (s *Server) rooms(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.game.AdminRooms())
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
		writeError(w, http.StatusBadRequest, err.Error())
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

func (s *Server) logs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 300
	}
	entries := logging.Recent(limit, r.URL.Query().Get("q"))
	writeJSON(w, entries)
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	term := strings.TrimSpace(r.URL.Query().Get("q"))
	if term == "" {
		writeJSON(w, []store.AdminCharacter{})
		return
	}
	found, err := s.game.AdminSearchCharacters(term, 50)
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
	writeJSON(w, out)
}

func (s *Server) results(w http.ResponseWriter, r *http.Request) {
	mode, _ := strconv.Atoi(r.URL.Query().Get("mode"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	rows, total, err := s.db.ListResults(uint32(mode), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"total": total, "items": rows})
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
	if body.Cash > maxCash {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("点券上限 %d", maxCash))
		return
	}
	if err := s.game.AdminSetBalance(body.CharacterID, body.Lucci, body.Cash); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "note": "写库完成；玩家重新登录、购买或打完一局后生效"})
}

// maxCash is the highest cash value the wire and the column accept.
const maxCash = 2147483647

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

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
