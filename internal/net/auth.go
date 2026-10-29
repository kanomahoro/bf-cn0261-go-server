package net

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/config"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/features"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/store"
)

// TicketIssuer issues one-time login tickets for verified credentials.
type TicketIssuer interface {
	IssueTicketFor(name, password string) (string, error)
}

// AccountService issues login tickets and registers accounts for the launcher.
type AccountService interface {
	TicketIssuer
	// Create registers an account without a character (kept for tooling and for
	// launchers that predate the character-name field).
	Create(name, password string) (uint32, error)
	// Register creates the account and its first character together.
	Register(name, password, nickname string, characterType uint32) (store.Character, error)
}

const (
	// maxAuthConnections bounds concurrent launcher handshakes.
	maxAuthConnections = 256
	// authFailuresPerWindow limits *failed* logins per client address; scrypt
	// costs 64 MiB each, so unbounded guessing would starve real players. Only
	// failures count, otherwise several players behind one address lock
	// themselves out.
	authFailuresPerWindow = 10
	authWindow            = time.Minute
	// Registration is charged on success as well as failure: a successful
	// registration is itself the expensive scrypt hash an abuser wants, so the
	// login budget (failures only) cannot protect it.
	defaultRegistrationsPerHour = 5
	defaultRegistrationsPerDay  = 20
)

type attemptBucket struct {
	count int
	reset time.Time
}

// Auth implements the launcher TLS authentication endpoint (default 26102).
type Auth struct {
	cfg      config.Authentication
	accounts AccountService
	gamePort uint16
	log      *slog.Logger

	mu       sync.Mutex
	active   int
	attempts map[string]attemptBucket
	// registers bounds account creation per address. It is separate from
	// attempts because that window only charges failed logins.
	registers map[string]registerBucket
}

// registerBucket counts registrations in the hourly and daily windows.
type registerBucket struct {
	hourCount int
	hourReset time.Time
	dayCount  int
	dayReset  time.Time
}

// registerLimits reports the configured caps, falling back to the defaults.
func (a *Auth) registerLimits() (hour, day int) {
	hour = a.cfg.RegistrationsPerHour
	if hour <= 0 {
		hour = defaultRegistrationsPerHour
	}
	day = a.cfg.RegistrationsPerDay
	if day <= 0 {
		day = defaultRegistrationsPerDay
	}
	return hour, day
}

// allowRegistration charges one registration to the address, counting both
// successful and failed attempts, and reports whether it may proceed.
func (a *Auth) allowRegistration(address string) bool {
	hourLimit, dayLimit := a.registerLimits()
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	if a.registers == nil {
		a.registers = map[string]registerBucket{}
	}
	if len(a.registers) > 4096 {
		for key, entry := range a.registers {
			if !entry.hourReset.After(now) && !entry.dayReset.After(now) {
				delete(a.registers, key)
			}
		}
	}
	entry := a.registers[address]
	if !entry.hourReset.After(now) {
		entry.hourCount, entry.hourReset = 0, now.Add(time.Hour)
	}
	if !entry.dayReset.After(now) {
		entry.dayCount, entry.dayReset = 0, now.Add(24*time.Hour)
	}
	if entry.hourCount >= hourLimit || entry.dayCount >= dayLimit {
		a.registers[address] = entry
		return false
	}
	entry.hourCount++
	entry.dayCount++
	a.registers[address] = entry
	return true
}

// blocked reports whether the address already exhausted its failure budget.
func (a *Auth) blocked(address string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	entry, ok := a.attempts[address]
	if !ok {
		return false
	}
	if !entry.reset.After(time.Now()) {
		delete(a.attempts, address)
		return false
	}
	return entry.count >= authFailuresPerWindow
}

// recordFailure charges one failed login to the address.
func (a *Auth) recordFailure(address string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	if a.attempts == nil {
		a.attempts = map[string]attemptBucket{}
	}
	if len(a.attempts) > 4096 {
		for key, entry := range a.attempts {
			if !entry.reset.After(now) {
				delete(a.attempts, key)
			}
		}
	}
	entry := a.attempts[address]
	if !entry.reset.After(now) {
		entry = attemptBucket{reset: now.Add(authWindow)}
	}
	entry.count++
	a.attempts[address] = entry
}

// allowAttempt applies the per-address sliding window.
func (a *Auth) allowAttempt(address string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	if a.attempts == nil {
		a.attempts = map[string]attemptBucket{}
	}
	if len(a.attempts) > 4096 {
		for key, entry := range a.attempts {
			if !entry.reset.After(now) {
				delete(a.attempts, key)
			}
		}
	}
	entry := a.attempts[address]
	if !entry.reset.After(now) {
		entry = attemptBucket{reset: now.Add(authWindow)}
	}
	if entry.count >= authFailuresPerWindow {
		a.attempts[address] = entry
		return false
	}
	entry.count++
	a.attempts[address] = entry
	return true
}

func (a *Auth) acquire() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.active >= maxAuthConnections {
		return false
	}
	a.active++
	return true
}

func (a *Auth) release() {
	a.mu.Lock()
	a.active--
	a.mu.Unlock()
}

// NewAuth builds the launcher authentication listener.
func NewAuth(cfg config.Authentication, accounts AccountService, gamePort uint16, log *slog.Logger) *Auth {
	return &Auth{cfg: cfg, accounts: accounts, gamePort: gamePort, log: log}
}

type authRequest struct {
	Version     uint32 `json:"version"`
	Action      string `json:"action"`
	AccountName string `json:"accountName"`
	Password    string `json:"password"`
	InviteCode  string `json:"inviteCode"`
	// Nickname and CharacterType make registration produce a playable account:
	// the character is created in the same transaction. An empty nickname keeps
	// the old behaviour (account only, character created in game).
	Nickname      string `json:"nickname"`
	CharacterType uint32 `json:"characterType"`
}

type authResponse struct {
	Success         bool   `json:"success"`
	LoginTicket     string `json:"loginTicket,omitempty"`
	GamePort        uint16 `json:"gamePort,omitempty"`
	ProtocolVersion uint32 `json:"protocolVersion,omitempty"`
	Error           string `json:"error,omitempty"`
}

// Serve accepts launcher connections until the context is cancelled.
func (a *Auth) Serve(ctx context.Context) error {
	certificate, err := tls.LoadX509KeyPair(a.cfg.CertificateFile, a.cfg.PrivateKeyFile)
	if err != nil {
		return err
	}
	listener, err := tls.Listen("tcp", net.JoinHostPort(a.cfg.ListenAddress, itoa(int(a.cfg.ListenPort))),
		&tls.Config{
			Certificates: []tls.Certificate{certificate},
			MinVersion:   tls.VersionTLS12,
		})
	if err != nil {
		return err
	}
	defer listener.Close()
	go func() {
		<-ctx.Done()
		listener.Close()
	}()
	a.log.Info("[Authentication] listening", "port", a.cfg.ListenPort)
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if !a.acquire() {
			a.log.Warn("[Authentication] connection limit reached")
			conn.Close()
			continue
		}
		go a.handle(conn)
	}
}

func (a *Auth) handle(conn net.Conn) {
	defer a.release()
	defer conn.Close()
	defer func() {
		if recovered := recover(); recovered != nil {
			a.log.Error("[Authentication] connection panic", "panic", recovered)
		}
	}()
	var remote string
	if address, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		remote = address.IP.String()
	}

	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	var header [4]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return
	}
	length := binary.LittleEndian.Uint32(header[:])
	if length == 0 || length > 4096 {
		return
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(conn, body); err != nil {
		return
	}
	var request authRequest
	if err := json.Unmarshal(body, &request); err != nil {
		a.reply(conn, authResponse{Success: false, Error: "invalidRequest"})
		return
	}
	if request.Version != 1 || (request.Action != "login" && request.Action != "register") {
		a.reply(conn, authResponse{Success: false, Error: "invalidRequest"})
		return
	}
	if err := features.CheckAccountName(request.AccountName); err != nil {
		a.reply(conn, authResponse{Success: false, Error: "invalidRequest"})
		return
	}
	if err := features.CheckPasswordPolicy(request.Password); err != nil {
		a.reply(conn, authResponse{Success: false, Error: "invalidRequest"})
		return
	}
	if request.Action == "register" {
		// The budget is charged before the invite code is compared so guessing the
		// code costs the same as creating an account.
		if remote != "" && !a.allowRegistration(remote) {
			a.reply(conn, authResponse{Success: false, Error: "serviceBusy"})
			return
		}
		if a.cfg.InviteCode != "" &&
			subtle.ConstantTimeCompare([]byte(request.InviteCode), []byte(a.cfg.InviteCode)) != 1 {
			a.reply(conn, authResponse{Success: false, Error: "inviteRequired"})
			return
		}
		if request.Nickname == "" {
			// Launcher without the character field: account only, the client
			// creates the character on first login (the original flow).
			if _, err := a.accounts.Create(request.AccountName, request.Password); err != nil {
				a.registrationError(conn, request.AccountName, err)
				return
			}
			a.log.Info("[Authentication] account registered", "account", request.AccountName,
				"character", false, "address", remote)
		} else {
			characterType := request.CharacterType
			if characterType == 0 {
				characterType = 1
			}
			if _, err := a.accounts.Register(request.AccountName, request.Password,
				request.Nickname, characterType); err != nil {
				a.registrationError(conn, request.AccountName, err)
				return
			}
			a.log.Info("[Authentication] account registered", "account", request.AccountName,
				"character", true, "nickname", strings.TrimSpace(request.Nickname),
				"address", remote)
		}
		a.reply(conn, authResponse{Success: true})
		return
	}
	// Count only complete login attempts from this address, not half-open
	// connections or malformed frames.
	if remote != "" && a.blocked(remote) {
		a.reply(conn, authResponse{Success: false, Error: "serviceBusy"})
		return
	}
	ticket, err := a.accounts.IssueTicketFor(request.AccountName, request.Password)
	if err != nil {
		if errors.Is(err, features.ErrInvalidCredentials) {
			if remote != "" {
				a.recordFailure(remote)
			}
			a.reply(conn, authResponse{Success: false, Error: "credentialsRejected"})
			return
		}
		a.log.Error("authentication failed", "account", request.AccountName, "error", err)
		a.reply(conn, authResponse{Success: false, Error: "serviceBusy"})
		return
	}
	a.reply(conn, authResponse{
		Success:         true,
		LoginTicket:     ticket,
		GamePort:        a.gamePort,
		ProtocolVersion: 717,
	})
}

// registrationError maps a registration failure to the launcher's error code.
func (a *Auth) registrationError(conn net.Conn, account string, err error) {
	switch {
	case errors.Is(err, features.ErrAccountTaken), errors.Is(err, store.ErrDuplicate):
		a.reply(conn, authResponse{Success: false, Error: "accountTaken"})
	case errors.Is(err, features.ErrNicknameTaken):
		a.reply(conn, authResponse{Success: false, Error: "nicknameTaken"})
	case errors.Is(err, features.ErrNicknamePolicy), errors.Is(err, features.ErrCharacterType),
		errors.Is(err, features.ErrPasswordPolicy):
		a.reply(conn, authResponse{Success: false, Error: "invalidRequest"})
	default:
		a.log.Error("registration failed", "account", account, "error", err)
		a.reply(conn, authResponse{Success: false, Error: "serviceBusy"})
	}
}

func (a *Auth) reply(conn net.Conn, response authResponse) {
	body, err := json.Marshal(response)
	if err != nil {
		return
	}
	frame := make([]byte, 4+len(body))
	binary.LittleEndian.PutUint32(frame, uint32(len(body)))
	copy(frame[4:], body)
	_, _ = conn.Write(frame)
}
