// Package features implements the accounts, characters, inventory and shop
// services.
package features

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/scrypt"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/store"
)

// PasswordScheme is the scheme name stored with each account.
const PasswordScheme = "scrypt"

// scrypt parameters matching the original server (N=32768, r=8, p=3, 64 MiB).
const (
	scryptN       = 32768
	scryptR       = 8
	scryptP       = 3
	scryptKeyLen  = 32
	scryptSaltLen = 16
)

// TicketLifetime is how long an issued login ticket stays valid.
const TicketLifetime = 120 * time.Second

// ErrInvalidCredentials reports a rejected account name or password.
var ErrInvalidCredentials = errors.New("features: invalid credentials")

// ErrPasswordPolicy reports a password that violates the length policy.
var ErrPasswordPolicy = errors.New("Password must contain 8 to 128 UTF-8 bytes and no null characters")

// Registration outcomes the launcher translates into messages.
var (
	// ErrAccountTaken reports an account name that already exists.
	ErrAccountTaken = errors.New("features: account name already taken")
	// ErrNicknameTaken reports a character name that already exists.
	ErrNicknameTaken = errors.New("features: nickname already taken")
	// ErrNicknamePolicy reports a nickname the client would refuse.
	ErrNicknamePolicy = errors.New("features: nickname policy")
	// ErrCharacterType reports a starter choice outside the two playable ones.
	ErrCharacterType = errors.New("features: unsupported character type")
)

// Accounts implements account creation, login and one-time tickets.
//
// scrypt needs 64 MiB per hash, so hashing is bounded exactly like the original
// server's two-thread password pool; unbounded concurrency would let a few
// simultaneous logins exhaust a small host.
type Accounts struct {
	db    *store.DB
	slots chan struct{}
}

// maxConcurrentHashes mirrors the original thread_pool{2}.
const maxConcurrentHashes = 2

// NewAccounts returns an account service.
func NewAccounts(db *store.DB) *Accounts {
	return &Accounts{db: db, slots: make(chan struct{}, maxConcurrentHashes)}
}

// hashPasswordBounded runs scrypt while holding one of the hash slots.
func (a *Accounts) hashPasswordBounded(password, salt []byte) (string, error) {
	a.slots <- struct{}{}
	defer func() { <-a.slots }()
	return hashPassword(password, salt)
}

// CheckPasswordPolicy validates the password byte length.
func CheckPasswordPolicy(password string) error {
	if len(password) < 8 || len(password) > 128 || strings.ContainsRune(password, 0) {
		return ErrPasswordPolicy
	}
	return nil
}

func hashPassword(password, salt []byte) (string, error) {
	key, err := scrypt.Key(password, salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}

// Create registers a new account and returns its id.
func (a *Accounts) Create(name, password string) (uint32, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if err := CheckAccountName(name); err != nil {
		return 0, err
	}
	if err := CheckPasswordPolicy(password); err != nil {
		return 0, err
	}
	salt := make([]byte, scryptSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return 0, err
	}
	hash, err := a.hashPasswordBounded([]byte(password), salt)
	if err != nil {
		return 0, err
	}
	return a.db.CreateAccount(name, PasswordScheme, hex.EncodeToString(salt), hash)
}

// Register creates an account together with its first character in one
// transaction, so a failed registration never leaves an account without a
// character. The starter choice follows the original's IsStarterSelection: only
// the two playable types exist and the selection equals the type.
func (a *Accounts) Register(name, password, nickname string, characterType uint32) (store.Character, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if err := CheckAccountName(name); err != nil {
		return store.Character{}, err
	}
	if err := CheckPasswordPolicy(password); err != nil {
		return store.Character{}, err
	}
	if characterType != 1 && characterType != 2 {
		return store.Character{}, ErrCharacterType
	}
	// Trim before validating so a leading/trailing space never reaches the
	// database; the launcher and both CLIs share this path.
	nickname = strings.TrimSpace(nickname)
	if protocol.CheckNickname(protocol.U16String(nickname)) != protocol.NicknameOK {
		return store.Character{}, ErrNicknamePolicy
	}
	taken, err := a.db.NicknameTaken(nickname)
	if err != nil {
		return store.Character{}, err
	}
	if taken {
		return store.Character{}, ErrNicknameTaken
	}
	salt := make([]byte, scryptSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return store.Character{}, err
	}
	hash, err := a.hashPasswordBounded([]byte(password), salt)
	if err != nil {
		return store.Character{}, err
	}
	character, err := a.db.CreateAccountWithCharacter(name, PasswordScheme,
		hex.EncodeToString(salt), hash, nickname, characterType)
	if err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			// The unique key that tripped can be either the account name or the
			// nickname; callers show a different message for each.
			if _, lookupErr := a.db.AccountByName(name); lookupErr == nil {
				return store.Character{}, ErrAccountTaken
			}
			return store.Character{}, ErrNicknameTaken
		}
		return store.Character{}, err
	}
	return character, nil
}

// CheckAccountName enforces the 3..32 lowercase alphanumeric rule.
func CheckAccountName(name string) error {
	if len(name) < 3 || len(name) > 32 {
		return fmt.Errorf("account name must contain 3 to 32 characters")
	}
	for _, character := range name {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '_' {
			return fmt.Errorf("account name must use lowercase letters, digits or underscores")
		}
	}
	return nil
}

// Verify checks a name/password pair and returns the account.
func (a *Accounts) Verify(name, password string) (store.Account, error) {
	account, err := a.db.AccountByName(name)
	if errors.Is(err, store.ErrNotFound) {
		return account, ErrInvalidCredentials
	}
	if err != nil {
		return account, err
	}
	if account.Disabled {
		return account, ErrInvalidCredentials
	}
	if account.PasswordScheme != PasswordScheme {
		return account, ErrInvalidCredentials
	}
	salt, err := hex.DecodeString(account.PasswordSalt)
	if err != nil {
		return account, err
	}
	hash, err := a.hashPasswordBounded([]byte(password), salt)
	if err != nil {
		return account, err
	}
	if subtle.ConstantTimeCompare([]byte(hash), []byte(account.PasswordHash)) != 1 {
		return account, ErrInvalidCredentials
	}
	return account, nil
}

// IssueTicket creates a one-time login ticket valid for TicketLifetime.
func (a *Accounts) IssueTicket(accountID uint32) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	if err := a.db.IssueTicket(accountID, store.HashToken(token), time.Now().UTC().Add(TicketLifetime)); err != nil {
		return "", err
	}
	return token, nil
}

// ConsumeTicket redeems a ticket and returns its account id.
func (a *Accounts) ConsumeTicket(token string) (uint32, error) {
	if len(token) != 64 {
		return 0, ErrInvalidCredentials
	}
	for _, character := range token {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return 0, ErrInvalidCredentials
		}
	}
	accountID, err := a.db.ConsumeTicket(store.HashToken(token))
	if errors.Is(err, store.ErrNotFound) {
		return 0, ErrInvalidCredentials
	}
	return accountID, err
}

// IssueTicketFor verifies credentials and issues a ticket in one step.
func (a *Accounts) IssueTicketFor(name, password string) (string, error) {
	account, err := a.Verify(name, password)
	if err != nil {
		return "", err
	}
	return a.IssueTicket(account.ID)
}
