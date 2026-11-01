// Package store persists accounts, characters, inventory, shop and results in
// MariaDB. Only MariaDB is supported and verified; MySQL is untested.
package store

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/config"
)

// ErrNotFound reports a missing row.
var ErrNotFound = errors.New("store: not found")

// CharacterSlot is the equipment slot that holds the worn character item.
const CharacterSlot = 63

// Grenade item and its special-weapon slot. The CN0261 client defines exactly
// one grenade (weapon definition 5003, item id 200000+5003) and reads the worn
// item in slot 21 of a character to decide how many grenades a round carries.
const (
	GrenadeItemID      = 205003
	GrenadeSlot        = 21
	StarterGrenades    = 100
	grenadeOccupiedBit = 1 << GrenadeSlot
)

// ErrDuplicate reports a unique-constraint violation.
var ErrDuplicate = errors.New("store: duplicate entry")

var tlsConfigSeq uint64

// DB wraps the SQL pool.
type DB struct {
	sql     *sql.DB
	timeout time.Duration
}

// Open connects to the database using the configured TLS mode.
func Open(cfg config.Database, password string) (*DB, error) {
	mc := mysql.NewConfig()
	mc.User = cfg.Username
	mc.Passwd = password
	mc.Net = "tcp"
	mc.Addr = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	mc.DBName = cfg.DatabaseName
	mc.ParseTime = true
	mc.Loc = time.UTC
	mc.Timeout = time.Duration(cfg.ConnectTimeoutSeconds) * time.Second
	mc.ReadTimeout = time.Duration(cfg.QueryTimeoutSeconds) * time.Second
	mc.WriteTimeout = time.Duration(cfg.QueryTimeoutSeconds) * time.Second
	// Send the collation in the handshake instead of the "charset" parameter:
	// the latter is issued as SET charset=..., which MariaDB rejects with
	// "Unknown system variable 'charset'".
	mc.Collation = "utf8mb4_unicode_ci"

	switch cfg.TLSMode {
	case "require":
		if cfg.VerifyCertificate {
			pool := x509.NewCertPool()
			if cfg.CAFile != "" {
				pem, err := os.ReadFile(cfg.CAFile)
				if err != nil {
					return nil, fmt.Errorf("cannot read database CA file: %w", err)
				}
				if !pool.AppendCertsFromPEM(pem) {
					return nil, fmt.Errorf("database CA file contains no certificates")
				}
			}
			host := cfg.Host
			name := fmt.Sprintf("bf-verify-%d", atomic.AddUint64(&tlsConfigSeq, 1))
			mc.TLSConfig = name
			if err := mysql.RegisterTLSConfig(name, &tls.Config{
				RootCAs:    pool,
				ServerName: host,
				MinVersion: tls.VersionTLS12,
			}); err != nil {
				return nil, fmt.Errorf("cannot register TLS config: %w", err)
			}
		} else {
			mc.TLSConfig = "skip-verify"
		}
	case "disable":
		mc.TLSConfig = "false"
	}

	connector, err := mysql.NewConnector(mc)
	if err != nil {
		return nil, fmt.Errorf("cannot create database connector: %w", err)
	}
	pool := sql.OpenDB(connector)
	pool.SetMaxOpenConns(cfg.PoolSize)
	pool.SetMaxIdleConns(cfg.PoolSize)
	pool.SetConnMaxLifetime(30 * time.Minute)

	return &DB{sql: pool, timeout: time.Duration(cfg.QueryTimeoutSeconds) * time.Second}, nil
}

// Close releases the pool.
func (db *DB) Close() error { return db.sql.Close() }

func (db *DB) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), db.timeout)
}

// CheckConnection verifies the pool answers a trivial query.
func (db *DB) CheckConnection() error {
	ctx, cancel := db.ctx()
	defer cancel()
	var one int
	if err := db.sql.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
		return err
	}
	if one != 1 {
		return fmt.Errorf("database health query returned an unexpected result")
	}
	return nil
}

// migrationNames is the exact schema this build expects.
var migrationNames = []string{
	"InitializeSchema", "CreateAccounts", "CreateCharacters", "CreateInventory",
	"CreateCharacterStats", "CreateShop", "CreateGameResults", "ExpandShopCosmetics",
	"CreateShopCharacters", "ExpandShopEquipment", "CreateRespawnEquipment",
	"CreateResultAwards", "CreateWeaponPresets", "ExpandShopOther",
}

// CheckSchema enforces the 14-row migration contract.
func (db *DB) CheckSchema() error {
	ctx, cancel := db.ctx()
	defer cancel()
	rows, err := db.sql.QueryContext(ctx, "SELECT version, name FROM schema_migrations ORDER BY version")
	if err != nil {
		return err
	}
	defer rows.Close()
	var versions []uint32
	var names []string
	for rows.Next() {
		var version uint32
		var name string
		if err := rows.Scan(&version, &name); err != nil {
			return err
		}
		versions = append(versions, version)
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(versions) != len(migrationNames) {
		return fmt.Errorf("database schema version mismatch: expected versions 1 through %d (ExpandShopOther)", len(migrationNames))
	}
	for i, want := range migrationNames {
		if versions[i] != uint32(i+1) || names[i] != want {
			return fmt.Errorf("database schema version mismatch: expected versions 1 through %d (ExpandShopOther)", len(migrationNames))
		}
	}
	return nil
}

// Account is one row of the accounts table.
type Account struct {
	ID             uint32
	Name           string
	PasswordScheme string
	PasswordSalt   string
	PasswordHash   string
	Disabled       bool
}

// CreateAccount inserts an account and returns its id.
func (db *DB) CreateAccount(name, scheme, salt, hash string) (uint32, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	res, err := db.sql.ExecContext(ctx,
		`INSERT INTO accounts (account_name, password_scheme, password_salt, password_hash, disabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, FALSE, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`,
		name, scheme, salt, hash)
	if err != nil {
		return 0, mapError(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return uint32(id), nil
}

// AccountByName loads one account by name.
func (db *DB) AccountByName(name string) (Account, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	var account Account
	err := db.sql.QueryRowContext(ctx,
		`SELECT id, account_name, password_scheme, password_salt, password_hash, disabled
		 FROM accounts WHERE account_name = ?`, name).
		Scan(&account.ID, &account.Name, &account.PasswordScheme, &account.PasswordSalt,
			&account.PasswordHash, &account.Disabled)
	if errors.Is(err, sql.ErrNoRows) {
		return account, ErrNotFound
	}
	return account, err
}

// AccountByID loads one account by id.
func (db *DB) AccountByID(id uint32) (Account, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	var account Account
	err := db.sql.QueryRowContext(ctx,
		`SELECT id, account_name, password_scheme, password_salt, password_hash, disabled
		 FROM accounts WHERE id = ?`, id).
		Scan(&account.ID, &account.Name, &account.PasswordScheme, &account.PasswordSalt,
			&account.PasswordHash, &account.Disabled)
	if errors.Is(err, sql.ErrNoRows) {
		return account, ErrNotFound
	}
	return account, err
}

// IssueTicket stores a hashed one-time login ticket.
func (db *DB) IssueTicket(accountID uint32, tokenHash string, expires time.Time) error {
	ctx, cancel := db.ctx()
	defer cancel()
	_, err := db.sql.ExecContext(ctx,
		`INSERT INTO login_tickets (account_id, token_hash, expires_at, created_at)
		 VALUES (?, ?, ?, UTC_TIMESTAMP(6))`, accountID, tokenHash, expires.UTC())
	return mapError(err)
}

// ConsumeTicket marks a ticket as used and returns the owning account id.
func (db *DB) ConsumeTicket(tokenHash string) (uint32, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var accountID uint32
	err = tx.QueryRowContext(ctx,
		`SELECT account_id FROM login_tickets
		 WHERE token_hash = ? AND consumed_at IS NULL AND expires_at > UTC_TIMESTAMP(6)
		 FOR UPDATE`, tokenHash).Scan(&accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE login_tickets SET consumed_at = UTC_TIMESTAMP(6)
		 WHERE token_hash = ? AND consumed_at IS NULL`, tokenHash); err != nil {
		return 0, err
	}
	return accountID, tx.Commit()
}

// PurgeExpiredTickets deletes tickets that expired longer ago than grace and
// returns the number of removed rows. Consumed tickets are already unusable,
// so the grace window only keeps recently expired rows around for diagnostics.
func (db *DB) PurgeExpiredTickets(grace time.Duration) (int64, error) {
	if grace < 0 {
		return 0, fmt.Errorf("store: ticket purge grace must not be negative")
	}
	ctx, cancel := db.ctx()
	defer cancel()
	result, err := db.sql.ExecContext(ctx,
		`DELETE FROM login_tickets
		 WHERE expires_at < UTC_TIMESTAMP(6) - INTERVAL ? SECOND`,
		int64(grace.Seconds()))
	if err != nil {
		return 0, mapError(err)
	}
	removed, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return removed, nil
}

// Character is a persisted character row plus its progress.
type Character struct {
	ID            uint32
	AccountID     uint32
	Nickname      string
	CharacterType uint32
	StarterChoice uint32
	Experience    uint32
	Lucci         uint32
	Cash          uint32
}

// CharacterByAccount loads the account's character.
func (db *DB) CharacterByAccount(accountID uint32) (Character, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	var c Character
	err := db.sql.QueryRowContext(ctx,
		`SELECT c.id, c.account_id, c.nickname, c.character_type, c.starter_selection,
		        COALESCE(p.experience, 0), COALESCE(s.lucci, 0), COALESCE(s.cash, 0)
		 FROM characters c
		 LEFT JOIN character_progression p ON p.character_id = c.id
		 LEFT JOIN character_stats s ON s.character_id = c.id
		 WHERE c.account_id = ?`, accountID).
		Scan(&c.ID, &c.AccountID, &c.Nickname, &c.CharacterType, &c.StarterChoice,
			&c.Experience, &c.Lucci, &c.Cash)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// NicknameTaken reports whether a nickname is already used.
func (db *DB) NicknameTaken(nickname string) (bool, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	var id uint32
	err := db.sql.QueryRowContext(ctx, `SELECT id FROM characters WHERE nickname = ?`, nickname).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// CreateCharacter inserts a character with progression, stats and starter items
// in one transaction.
// insertCharacter writes the character row and everything creation grants:
// progression, the starter wallet, the practice weapons and the appearance item
// in slot 63. Callers own the transaction.
func insertCharacter(ctx context.Context, tx *sql.Tx, accountID uint32, nickname string, characterType, starter uint32) (uint32, error) {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO characters (account_id, nickname, character_type, starter_selection, created_at, updated_at)
		 VALUES (?, ?, ?, ?, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`,
		accountID, nickname, characterType, starter)
	if err != nil {
		return 0, mapError(err)
	}
	lastID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	characterID := uint32(lastID)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO character_progression (character_id, experience, updated_at)
		 VALUES (?, 0, UTC_TIMESTAMP(6))`, characterID); err != nil {
		return 0, mapError(err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO character_stats (character_id, lucci, cash, updated_at)
		 VALUES (?, ?, ?, UTC_TIMESTAMP(6))`, characterID, StarterLucci, StarterCash); err != nil {
		return 0, mapError(err)
	}
	// Starter loadout, mirroring the reference creation exactly: the five
	// practice weapons, equipped only when the catalog marks them as starters.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO character_items (character_id, item_id, quantity, equipped_slot, created_at, updated_at)
		 SELECT ?, d.id, 1,
		        CASE WHEN d.starter_equipped THEN d.equipment_slot ELSE NULL END,
		        UTC_TIMESTAMP(6), UTC_TIMESTAMP(6)
		 FROM item_definitions d
		 WHERE d.id IN (201001, 202001, 203001, 204001, 207001)
		 ON DUPLICATE KEY UPDATE quantity = quantity`, characterID); err != nil {
		return 0, mapError(err)
	}
	// Appearance item: slot 63 holds the character model. Without it the client
	// has nothing to render in the plaza or the result screen.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO character_items (character_id, item_id, quantity, equipped_slot, created_at, updated_at)
		 SELECT ?, a.item_id, 1, 63, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6)
		 FROM character_appearances a WHERE a.character_type = ?
		 ON DUPLICATE KEY UPDATE quantity = quantity`, characterID, characterType); err != nil {
		return 0, mapError(err)
	}
	// Grenade comes last so it cannot disturb the starter grants above: every
	// new character carries the free grenades worn in the special weapon slot.
	// The SELECT form keeps character creation working (zero rows, no error) on
	// a database whose catalogue lacks the definition, instead of failing on the
	// foreign key.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO character_items (character_id, item_id, quantity, equipped_slot, created_at, updated_at)
		 SELECT ?, d.id, ?, d.equipment_slot, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6)
		 FROM item_definitions d WHERE d.id = ? AND d.equipment_slot = ?
		 ON DUPLICATE KEY UPDATE quantity = quantity`,
		characterID, StarterGrenades, GrenadeItemID, GrenadeSlot); err != nil {
		return 0, mapError(err)
	}
	return characterID, nil
}

// insertAccount writes one account row. Callers own the transaction.
func insertAccount(ctx context.Context, tx *sql.Tx, name, scheme, salt, hash string) (uint32, error) {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO accounts (account_name, password_scheme, password_salt, password_hash, disabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, FALSE, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`,
		name, scheme, salt, hash)
	if err != nil {
		return 0, mapError(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return uint32(id), nil
}

func (db *DB) CreateCharacter(accountID uint32, nickname string, characterType, starter uint32) (Character, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return Character{}, err
	}
	defer tx.Rollback()

	characterID, err := insertCharacter(ctx, tx, accountID, nickname, characterType, starter)
	if err != nil {
		return Character{}, err
	}
	if err := tx.Commit(); err != nil {
		return Character{}, err
	}
	// The rows are committed at this point, so a failed read-back must not turn
	// a successful creation into a reported failure. The fallback carries
	// exactly what insertCharacter wrote for a brand-new character.
	character, err := db.CharacterByAccount(accountID)
	if err != nil {
		return newCharacter(characterID, accountID, nickname, characterType, starter), nil
	}
	return character, nil
}

// newCharacter is the row insertCharacter creates, without a database read.
func newCharacter(characterID, accountID uint32, nickname string, characterType, starter uint32) Character {
	return Character{
		ID:            characterID,
		AccountID:     accountID,
		Nickname:      nickname,
		CharacterType: characterType,
		StarterChoice: starter,
		Lucci:         StarterLucci,
		Cash:          StarterCash,
	}
}

// CreateAccountWithCharacter registers an account and its first character in one
// transaction, so a registration either produces a playable account or nothing
// at all. starter_selection follows the original's IsStarterSelection: it must
// equal the chosen character type.
func (db *DB) CreateAccountWithCharacter(name, scheme, salt, hash, nickname string, characterType uint32) (Character, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return Character{}, err
	}
	defer tx.Rollback()

	accountID, err := insertAccount(ctx, tx, name, scheme, salt, hash)
	if err != nil {
		return Character{}, err
	}
	characterID, err := insertCharacter(ctx, tx, accountID, nickname, characterType, characterType)
	if err != nil {
		return Character{}, err
	}
	if err := tx.Commit(); err != nil {
		return Character{}, err
	}
	character, err := db.CharacterByAccount(accountID)
	if err != nil {
		return newCharacter(characterID, accountID, nickname, characterType, characterType), nil
	}
	return character, nil
}

// EnsureGrenadeItem makes the grenade's catalogue rows exist. It runs at server
// startup instead of a schema migration because CheckSchema pins the migration
// list exactly; the statements are idempotent so a database that already holds
// the rows (hand-seeded or created by an earlier start) is left untouched.
//
// The character mask is taken from the appearance table so every character type
// this build can actually create may wear the grenade; the fallback covers
// types 1 through 12 when the table is empty.
func (db *DB) EnsureGrenadeItem() error {
	ctx, cancel := db.ctx()
	defer cancel()
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
		 SELECT ?, '手雷', ?, FALSE
		 FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = ?)`,
		GrenadeItemID, GrenadeSlot, GrenadeItemID); err != nil {
		return mapError(err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
		 SELECT ?, ?,
		        COALESCE((SELECT BIT_OR(CAST(1 AS UNSIGNED) << (character_type - 1))
		                  FROM character_appearances), 4095)
		 FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = ?)`,
		GrenadeItemID, grenadeOccupiedBit, GrenadeItemID); err != nil {
		return mapError(err)
	}
	return tx.Commit()
}

// Starter wallet for a new character. The reference hands out 10000 lucci and
// no cash; this server gives enough that buying gear never gets in the way.
//
// Five million covers the whole on-sale catalogue roughly four times over (one
// of every stock costs ~1.25M lucci / ~0.38M cash) while staying far below both
// limits: character_stats checks value <= 2147483647 and the wire validator
// rejects anything above math.MaxInt32, and settlement adds each match's
// LucciGain to the balance, so the starter amount must leave room to grow.
const (
	StarterLucci = 5_000_000
	StarterCash  = 5_000_000
)

// MaxLucci is the highest balance settlement will store or report. Anything a
// match would add past it is clamped and the result screen reports the gain that
// was actually applied, so the interface and the database always agree.
const MaxLucci = 10_000_000

// NormalizeTotal pins a stored total to its cap. A value can only exceed the cap
// through legacy data or a manual edit; the result row requires
// `before + gain = after`, so the baseline has to be normalized as well.
func NormalizeTotal(value, cap uint32) uint32 {
	if cap > 0 && value > cap {
		return cap
	}
	return value
}

// ClampGain applies one settlement gain to a stored total and reports the total
// that is actually stored plus the gain that was actually applied. A stored
// value above the cap is normalized first, because the result row enforces
// `before + gain = after` and a legacy value would otherwise fail settlement.
// A cap of zero means "no cap".
func ClampGain(before, gain, cap uint32) (after, applied uint32) {
	if cap > 0 && before > cap {
		before = cap
	}
	// Sum in 64 bits: with no cap configured a stored total can already sit near
	// the uint32 ceiling and the addition would wrap.
	total := uint64(before) + uint64(gain)
	if cap > 0 && total > uint64(cap) {
		total = uint64(cap)
	}
	after = uint32(total)
	if after > before {
		applied = after - before
		if applied > gain {
			applied = gain
		}
	}
	return after, applied
}

// Item is one inventory row.
type Item struct {
	RowID        uint64
	ItemID       uint32
	Quantity     uint32
	EquippedSlot int
	ExpiresAt    *time.Time
}

// Items lists a character's inventory.
func (db *DB) Items(characterID uint32) ([]Item, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	rows, err := db.sql.QueryContext(ctx,
		`SELECT ci.id, ci.item_id, ci.quantity, COALESCE(ci.equipped_slot, 0), e.expires_at
		 FROM character_items ci
		 LEFT JOIN character_item_expirations e ON e.character_item_id = ci.id
		 WHERE ci.character_id = ? AND (e.expires_at IS NULL OR e.expires_at > UTC_TIMESTAMP(6))
		 ORDER BY ci.id`, characterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Item
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.RowID, &item.ItemID, &item.Quantity, &item.EquippedSlot, &item.ExpiresAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// NormalizeAppearance reconciles a character's worn items before any
// inventory read, mirroring NormalizeAppearance in the original
// inventory_repository:
//
//  1. an expired item never stays equipped,
//  2. the character slot holds a character (the creation type is worn when the
//     player has no character item), and
//  3. gear the active character cannot wear comes off.
//
// It returns the active character type so callers compute health and respawn
// from the body the player is actually using.
func (db *DB) NormalizeAppearance(characterID uint32) (uint32, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	active, err := normalizeAppearanceTx(ctx, tx, characterID)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return active, nil
}

// normalizeAppearanceTx is NormalizeAppearance inside a caller-owned
// transaction, so preset selection can reconcile and equip atomically.
func normalizeAppearanceTx(ctx context.Context, tx *sql.Tx, characterID uint32) (uint32, error) {

	var fallback uint32
	if err := tx.QueryRowContext(ctx,
		`SELECT character_type FROM characters WHERE id = ? FOR UPDATE`, characterID).Scan(&fallback); err != nil {
		return 0, mapError(err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE character_items
		 JOIN character_item_expirations ON character_item_expirations.character_item_id = character_items.id
		 SET character_items.equipped_slot = NULL, character_items.updated_at = UTC_TIMESTAMP(6)
		 WHERE character_items.character_id = ? AND character_items.equipped_slot IS NOT NULL
		   AND character_item_expirations.expires_at <= UTC_TIMESTAMP(6)`, characterID); err != nil {
		return 0, mapError(err)
	}

	active := fallback
	var err error
	err = tx.QueryRowContext(ctx,
		`SELECT character_appearances.character_type
		 FROM character_items
		 JOIN character_appearances ON character_appearances.item_id = character_items.item_id
		 WHERE character_items.character_id = ? AND character_items.equipped_slot = ?`,
		characterID, CharacterSlot).Scan(&active)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		active = fallback
		// A row can hold slot 63 without being a character item (an imported
		// database or a definition edit). Taking it off first keeps the next
		// statement from colliding with the unique
		// (character_id, equipped_slot) key, which would fail this transaction
		// and with it every inventory read for that character.
		if _, err := tx.ExecContext(ctx,
			`UPDATE character_items
			 LEFT JOIN character_appearances ON character_appearances.item_id = character_items.item_id
			 SET character_items.equipped_slot = NULL, character_items.updated_at = UTC_TIMESTAMP(6)
			 WHERE character_items.character_id = ? AND character_items.equipped_slot = ?
			   AND character_appearances.item_id IS NULL`, characterID, CharacterSlot); err != nil {
			return 0, mapError(err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE character_items
			 JOIN character_appearances ON character_appearances.item_id = character_items.item_id
			 SET character_items.equipped_slot = ?, character_items.updated_at = UTC_TIMESTAMP(6)
			 WHERE character_items.character_id = ? AND character_appearances.character_type = ?
			 LIMIT 1`, CharacterSlot, characterID, fallback); err != nil {
			return 0, mapError(err)
		}
	case err != nil:
		return 0, mapError(err)
	}
	// A zero type would turn the mask shift below into an underflow and strip
	// the whole loadout.
	if active == 0 {
		active = 1
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE character_items
		 JOIN item_equipment_rules ON item_equipment_rules.item_id = character_items.item_id
		 SET character_items.equipped_slot = NULL, character_items.updated_at = UTC_TIMESTAMP(6)
		 WHERE character_items.character_id = ? AND character_items.equipped_slot IS NOT NULL
		   AND character_items.equipped_slot <> ?
		   AND (item_equipment_rules.character_mask & (CAST(1 AS UNSIGNED) << (? - 1))) = 0`,
		characterID, CharacterSlot, active); err != nil {
		return 0, mapError(err)
	}
	return active, nil
}

// Equip moves an item into its equipment slot.
func (db *DB) Equip(characterID, itemID uint32) error {
	ctx, cancel := db.ctx()
	defer cancel()
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var slot int
	if err := tx.QueryRowContext(ctx,
		`SELECT equipment_slot FROM item_definitions WHERE id = ?`, itemID).Scan(&slot); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if slot == 0 {
		return ErrNotFound
	}
	var owned uint64
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM character_items WHERE character_id = ? AND item_id = ? FOR UPDATE`,
		characterID, itemID).Scan(&owned); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE character_items SET equipped_slot = NULL, updated_at = UTC_TIMESTAMP(6)
		 WHERE character_id = ? AND equipped_slot = ?`, characterID, slot); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE character_items SET equipped_slot = ?, updated_at = UTC_TIMESTAMP(6)
		 WHERE character_id = ? AND item_id = ?`, slot, characterID, itemID); err != nil {
		return err
	}
	return tx.Commit()
}

// Unequip clears an item's equipment slot.
func (db *DB) Unequip(characterID, itemID uint32) error {
	ctx, cancel := db.ctx()
	defer cancel()
	res, err := db.sql.ExecContext(ctx,
		`UPDATE character_items SET equipped_slot = NULL, updated_at = UTC_TIMESTAMP(6)
		 WHERE character_id = ? AND item_id = ?`, characterID, itemID)
	if err != nil {
		return err
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return ErrNotFound
	}
	return nil
}

// WeaponSlot is the equipment slot that holds the active weapon.
const WeaponSlot = 18

// PresetExpansionItem is the permanent item that opens the third weapon preset
// slot (kWeaponPresetExpansionItem in the original).
const PresetExpansionItem = 500001

// Presets is the persisted weapon preset state.
type Presets struct {
	Selected uint8
	Slots    uint8
	Items    []*uint32
}

// NormalizePresets seeds and repairs the weapon presets, mirroring
// NormalizeWeaponPresets in the original inventory_repository:
//
//  1. the preset row exists,
//  2. every available slot starts from the equipped weapon,
//  3. the equipped inventory stays authoritative for the selected slot,
//  4. an entry whose item vanished, ran out, is not a weapon or has expired is
//     cleared.
//
// The equipped weapon is never replaced: no default weapon is granted here.
func (db *DB) NormalizePresets(characterID uint32) (Presets, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return Presets{}, err
	}
	defer tx.Rollback()
	presets, err := normalizePresetsTx(ctx, tx, characterID)
	if err != nil {
		return Presets{}, err
	}
	if err := tx.Commit(); err != nil {
		return Presets{}, err
	}
	return presets, nil
}

// normalizePresetsTx is NormalizePresets inside a caller-owned transaction.
func normalizePresetsTx(ctx context.Context, tx *sql.Tx, characterID uint32) (Presets, error) {

	if _, err := tx.ExecContext(ctx,
		`INSERT IGNORE INTO character_weapon_presets (character_id) VALUES (?)`, characterID); err != nil {
		return Presets{}, mapError(err)
	}
	var presets Presets
	if err := tx.QueryRowContext(ctx,
		`SELECT active_index, slot_count FROM character_weapon_presets WHERE character_id = ?`,
		characterID).Scan(&presets.Selected, &presets.Slots); err != nil {
		return Presets{}, mapError(err)
	}
	// The schema allows two or three slots; anything else is outside what the
	// client renders.
	if presets.Slots < 2 || presets.Slots > 3 {
		presets.Slots = 2
	}
	if presets.Selected >= presets.Slots {
		presets.Selected = 0
	}
	for index := uint8(0); index < presets.Slots; index++ {
		if _, err := tx.ExecContext(ctx,
			`INSERT IGNORE INTO character_weapon_preset_items (character_id, preset_index, item_id)
			 VALUES (?, ?, (SELECT item_id FROM character_items WHERE character_id = ? AND equipped_slot = ?))`,
			characterID, index, characterID, WeaponSlot); err != nil {
			return Presets{}, mapError(err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE character_weapon_preset_items
		 SET item_id = (SELECT item_id FROM character_items WHERE character_id = ? AND equipped_slot = ?)
		 WHERE character_id = ? AND preset_index = ?`,
		characterID, WeaponSlot, characterID, presets.Selected); err != nil {
		return Presets{}, mapError(err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE character_weapon_preset_items
		 LEFT JOIN character_items ON character_items.character_id = character_weapon_preset_items.character_id
		   AND character_items.item_id = character_weapon_preset_items.item_id
		 LEFT JOIN character_item_expirations ON character_item_expirations.character_item_id = character_items.id
		 LEFT JOIN item_definitions ON item_definitions.id = character_weapon_preset_items.item_id
		 SET character_weapon_preset_items.item_id = NULL
		 WHERE character_weapon_preset_items.character_id = ?
		   AND (character_items.id IS NULL OR character_items.quantity = 0
		        OR item_definitions.equipment_slot <> ?
		        OR character_item_expirations.expires_at <= UTC_TIMESTAMP(6))`,
		characterID, WeaponSlot); err != nil {
		return Presets{}, mapError(err)
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT preset_index, item_id FROM character_weapon_preset_items
		 WHERE character_id = ? ORDER BY preset_index`, characterID)
	if err != nil {
		return Presets{}, mapError(err)
	}
	items := make([]*uint32, presets.Slots)
	for rows.Next() {
		var index uint8
		var itemID sql.NullInt64
		if err := rows.Scan(&index, &itemID); err != nil {
			rows.Close()
			return Presets{}, err
		}
		if index < presets.Slots && itemID.Valid {
			value := uint32(itemID.Int64)
			items[index] = &value
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Presets{}, err
	}
	presets.Items = items
	return presets, nil
}

// LoadPresets reconciles and returns the presets in one transaction, mirroring
// the original's ReadWeaponPresets: appearance first, then the preset rows.
func (db *DB) LoadPresets(characterID uint32) (Presets, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return Presets{}, err
	}
	defer tx.Rollback()
	if _, err := normalizeAppearanceTx(ctx, tx, characterID); err != nil {
		return Presets{}, err
	}
	presets, err := normalizePresetsTx(ctx, tx, characterID)
	if err != nil {
		return Presets{}, err
	}
	if err := tx.Commit(); err != nil {
		return Presets{}, err
	}
	return presets, nil
}

// SelectPreset switches to one preset and equips the weapon it holds, following
// the original's SelectWeaponPreset: the index must be available, the weapon
// must fit the worn character, the previous weapon leaves slot 18 first, and an
// empty preset simply unequips. It reports whether the selection was accepted.
func (db *DB) SelectPreset(characterID uint32, index uint8) (Presets, bool, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return Presets{}, false, err
	}
	defer tx.Rollback()

	active, err := normalizeAppearanceTx(ctx, tx, characterID)
	if err != nil {
		return Presets{}, false, err
	}
	presets, err := normalizePresetsTx(ctx, tx, characterID)
	if err != nil {
		return Presets{}, false, err
	}
	if index >= presets.Slots {
		// The reference commits the transaction even when the selection is
		// refused, so the reconciliation it just performed stays.
		if err := tx.Commit(); err != nil {
			return Presets{}, false, err
		}
		return presets, false, nil
	}
	var weapon uint32
	if item := presets.Items[index]; item != nil {
		weapon = *item
	}
	if weapon != 0 {
		// The weapon has to fit the body the player is wearing.
		var allowed uint32
		err := tx.QueryRowContext(ctx,
			`SELECT item_id FROM item_equipment_rules
			 WHERE item_id = ? AND (character_mask & (CAST(1 AS UNSIGNED) << (? - 1))) <> 0`,
			weapon, active).Scan(&allowed)
		if errors.Is(err, sql.ErrNoRows) {
			if err := tx.Commit(); err != nil {
				return Presets{}, false, err
			}
			return presets, false, nil
		}
		if err != nil {
			return Presets{}, false, mapError(err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE character_items SET equipped_slot = NULL, updated_at = UTC_TIMESTAMP(6)
		 WHERE character_id = ? AND equipped_slot = ?`, characterID, WeaponSlot); err != nil {
		return Presets{}, false, mapError(err)
	}
	if weapon != 0 {
		if _, err := tx.ExecContext(ctx,
			`UPDATE character_items SET equipped_slot = ?, updated_at = UTC_TIMESTAMP(6)
			 WHERE character_id = ? AND item_id = ?`,
			WeaponSlot, characterID, weapon); err != nil {
			return Presets{}, false, mapError(err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE character_weapon_presets SET active_index = ? WHERE character_id = ?`,
		index, characterID); err != nil {
		return Presets{}, false, mapError(err)
	}
	// Reconcile once more so the returned view reflects the new equipment.
	presets, err = normalizePresetsTx(ctx, tx, characterID)
	if err != nil {
		return Presets{}, false, err
	}
	presets.Selected = index
	if err := tx.Commit(); err != nil {
		return Presets{}, false, err
	}
	return presets, true, nil
}

// ShopCard is one purchasable item with its stock options.
type ShopCard struct {
	ItemID       uint32
	Currency     string
	Name         string
	DisplayOrder uint32
	SubCategory  uint32
	MainCategory uint32
	SaleStart    time.Time
	SaleEnd      time.Time
	// Preview marks a card the client only shows: shop_preview_items entries are
	// display-only, so the purchase path refuses their stocks.
	Preview bool
	Stocks  []ShopStock
}

// ShopStock is one purchase option.
type ShopStock struct {
	ID        uint32
	ItemID    uint32
	Duration  uint32
	ShowPrice uint32
	SellPrice uint32
	// Currency is the shop_items.currency label ("lucci" or "cash_cn").
	Currency string
}

// ShopCategory is a main category with its children.
type ShopCategory struct {
	ID       uint32
	Name     string
	Order    uint32
	Children []ShopChild
}

// ShopChild is a sub-category.
type ShopChild struct {
	ID       uint32
	Name     string
	Order    uint32
	PageType uint32
}

// ShopCatalog loads the whole store front.
func (db *DB) ShopCatalog() ([]ShopCard, []ShopCategory, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	cards := []ShopCard{}
	rows, err := db.sql.QueryContext(ctx,
		`SELECT i.item_id, d.name, i.display_order, i.sub_category_id, s.main_category_id,
		        i.sale_starts_at, i.sale_ends_at, i.currency, p.item_id
		 FROM shop_items i
		 JOIN item_definitions d ON d.id = i.item_id
		 JOIN shop_sub_categories s ON s.id = i.sub_category_id
		 LEFT JOIN shop_preview_items p ON p.item_id = i.item_id
		 WHERE i.is_on_sale = TRUE
		   AND UTC_TIMESTAMP(6) >= i.sale_starts_at
		   AND UTC_TIMESTAMP(6) < i.sale_ends_at
		 ORDER BY i.display_order, i.item_id`)
	if err != nil {
		return nil, nil, err
	}
	index := map[uint32]int{}
	for rows.Next() {
		var card ShopCard
		var previewItem sql.NullInt64
		if err := rows.Scan(&card.ItemID, &card.Name, &card.DisplayOrder, &card.SubCategory,
			&card.MainCategory, &card.SaleStart, &card.SaleEnd, &card.Currency,
			&previewItem); err != nil {
			rows.Close()
			return nil, nil, err
		}
		card.Preview = previewItem.Valid
		index[card.ItemID] = len(cards)
		cards = append(cards, card)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	stockRows, err := db.sql.QueryContext(ctx,
		`SELECT id, item_id, duration_days, show_price, sell_price
		 FROM shop_stocks WHERE is_on_sale = TRUE
		 ORDER BY duration_days, id`)
	if err != nil {
		return nil, nil, err
	}
	for stockRows.Next() {
		var stock ShopStock
		if err := stockRows.Scan(&stock.ID, &stock.ItemID, &stock.Duration, &stock.ShowPrice, &stock.SellPrice); err != nil {
			stockRows.Close()
			return nil, nil, err
		}
		if i, ok := index[stock.ItemID]; ok {
			stock.Currency = cards[i].Currency
			cards[i].Stocks = append(cards[i].Stocks, stock)
		}
	}
	if err := stockRows.Err(); err != nil {
		stockRows.Close()
		return nil, nil, err
	}
	stockRows.Close()
	// The original joins shop_stocks, so a card without a purchasable stock is
	// not listed at all.
	withStock := cards[:0]
	for _, card := range cards {
		if len(card.Stocks) > 0 {
			withStock = append(withStock, card)
		}
	}
	cards = withStock
	if err := stockRows.Err(); err != nil {
		return nil, nil, err
	}

	categories := []ShopCategory{}
	catRows, err := db.sql.QueryContext(ctx,
		`SELECT id, name, display_order FROM shop_main_categories ORDER BY display_order`)
	if err != nil {
		return nil, nil, err
	}
	catIndex := map[uint32]int{}
	for catRows.Next() {
		var category ShopCategory
		if err := catRows.Scan(&category.ID, &category.Name, &category.Order); err != nil {
			catRows.Close()
			return nil, nil, err
		}
		catIndex[category.ID] = len(categories)
		categories = append(categories, category)
	}
	catRows.Close()

	childRows, err := db.sql.QueryContext(ctx,
		`SELECT id, main_category_id, name, display_order, page_type
		 FROM shop_sub_categories ORDER BY display_order`)
	if err != nil {
		return nil, nil, err
	}
	defer childRows.Close()
	for childRows.Next() {
		var child ShopChild
		var parent uint32
		if err := childRows.Scan(&child.ID, &parent, &child.Name, &child.Order, &child.PageType); err != nil {
			return nil, nil, err
		}
		if i, ok := catIndex[parent]; ok {
			categories[i].Children = append(categories[i].Children, child)
		}
	}
	return cards, categories, childRows.Err()
}

// Balance reports the character's currencies.
func (db *DB) Balance(characterID uint32) (lucci, cash uint32, err error) {
	ctx, cancel := db.ctx()
	defer cancel()
	err = db.sql.QueryRowContext(ctx,
		`SELECT lucci, cash FROM character_stats WHERE character_id = ?`, characterID).Scan(&lucci, &cash)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, ErrNotFound
	}
	return lucci, cash, err
}

// GrantItem adds an item to the inventory, extending an existing row.
func (db *DB) GrantItem(characterID, itemID, days uint32) error {
	ctx, cancel := db.ctx()
	defer cancel()
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO character_items (character_id, item_id, quantity, equipped_slot, created_at, updated_at)
		 VALUES (?, ?, 1, NULL, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))
		 ON DUPLICATE KEY UPDATE quantity = quantity + 1, updated_at = UTC_TIMESTAMP(6)`,
		characterID, itemID)
	if err != nil {
		return mapError(err)
	}
	if days > 0 {
		rowID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		var target uint64
		if rowID > 0 {
			target = uint64(rowID)
		} else {
			if err := tx.QueryRowContext(ctx,
				`SELECT id FROM character_items WHERE character_id = ? AND item_id = ?`,
				characterID, itemID).Scan(&target); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO character_item_expirations (character_item_id, expires_at)
			 VALUES (?, DATE_ADD(UTC_TIMESTAMP(6), INTERVAL ? DAY))
			 ON DUPLICATE KEY UPDATE expires_at = DATE_ADD(UTC_TIMESTAMP(6), INTERVAL ? DAY)`,
			target, days, days); err != nil {
			return mapError(err)
		}
	}
	return tx.Commit()
}

// SpendLucci deducts currency, failing when the balance is insufficient.
func (db *DB) SpendLucci(characterID, amount uint32) error {
	ctx, cancel := db.ctx()
	defer cancel()
	res, err := db.sql.ExecContext(ctx,
		`UPDATE character_stats SET lucci = lucci - ?, updated_at = UTC_TIMESTAMP(6)
		 WHERE character_id = ? AND lucci >= ?`, amount, characterID, amount)
	if err != nil {
		return err
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return ErrNotFound
	}
	return nil
}

// PurchaseItem spends currency, grants the item and records the audit row in a
// single transaction so a failure cannot take payment without delivery.
func (db *DB) PurchaseItem(characterID, stockID, itemID, price, days uint32, cash bool) error {
	ctx, cancel := db.ctx()
	defer cancel()
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	deduct := `UPDATE character_stats SET lucci = lucci - ?, updated_at = UTC_TIMESTAMP(6)
		 WHERE character_id = ? AND lucci >= ?`
	if cash {
		deduct = `UPDATE character_stats SET cash = cash - ?, updated_at = UTC_TIMESTAMP(6)
		 WHERE character_id = ? AND cash >= ?`
	}
	res, err := tx.ExecContext(ctx, deduct, price, characterID, price)
	if err != nil {
		return mapError(err)
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return ErrNotFound
	}

	insert, err := tx.ExecContext(ctx,
		`INSERT INTO character_items (character_id, item_id, quantity, equipped_slot, created_at, updated_at)
		 VALUES (?, ?, 1, NULL, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))
		 ON DUPLICATE KEY UPDATE quantity = quantity + 1, updated_at = UTC_TIMESTAMP(6)`,
		characterID, itemID)
	if err != nil {
		return mapError(err)
	}
	rowID, err := insert.LastInsertId()
	if err != nil {
		return err
	}
	if days > 0 {
		target := uint64(rowID)
		if target == 0 {
			if err := tx.QueryRowContext(ctx,
				`SELECT id FROM character_items WHERE character_id = ? AND item_id = ?`,
				characterID, itemID).Scan(&target); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO character_item_expirations (character_item_id, expires_at)
			 VALUES (?, DATE_ADD(UTC_TIMESTAMP(6), INTERVAL ? DAY))
			 ON DUPLICATE KEY UPDATE expires_at = DATE_ADD(UTC_TIMESTAMP(6), INTERVAL ? DAY)`,
			target, days, days); err != nil {
			return mapError(err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO shop_purchases (character_id, stock_id, lucci_spent, expires_at, created_at)
		 VALUES (?, ?, ?, DATE_ADD(UTC_TIMESTAMP(6), INTERVAL ? DAY), UTC_TIMESTAMP(6))`,
		characterID, stockID, price, days); err != nil {
		return mapError(err)
	}
	// The weapon-preset expansion opens the third slot, but only the permanent
	// stock carries that effect: a timed rental must not unlock it forever.
	if itemID == PresetExpansionItem {
		var permanent int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM shop_permanent_stocks WHERE stock_id = ?`, stockID).Scan(&permanent); err != nil {
			return mapError(err)
		}
		if permanent > 0 {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO character_weapon_presets (character_id, slot_count) VALUES (?, 3)
				 ON DUPLICATE KEY UPDATE slot_count = 3`, characterID); err != nil {
				return mapError(err)
			}
		}
	}
	return tx.Commit()
}

// RecordPurchase stores a shop purchase audit row.
func (db *DB) RecordPurchase(characterID, stockID, lucci uint32, expireDays uint32) error {
	ctx, cancel := db.ctx()
	defer cancel()
	_, err := db.sql.ExecContext(ctx,
		`INSERT INTO shop_purchases (character_id, stock_id, lucci_spent, expires_at, created_at)
		 VALUES (?, ?, ?, DATE_ADD(UTC_TIMESTAMP(6), INTERVAL ? DAY), UTC_TIMESTAMP(6))`,
		characterID, stockID, lucci, expireDays)
	return mapError(err)
}

// ResultPlayer is one settlement row.
type ResultPlayer struct {
	Slot           uint8
	CharacterID    uint32
	AccountID      uint32
	Nickname       string
	Team           uint8
	Bubbles        uint16
	Pangs          uint16
	Rescues        uint16
	ExperienceGain uint32
	LucciGain      uint32
	Score          uint32
	Grade          uint32
	Medal          uint32
}

// Result is one finished match.
type Result struct {
	RoomID        uint16
	TrackingID    uint32
	Mode          uint32
	SubMode       uint32
	MapName       string
	EndReason     uint8
	FirstScore    uint16
	SecondScore   uint16
	Winner        uint8
	PolicyVersion string
	Players       []ResultPlayer
}

// LevelFn resolves the level, its starting experience and the next threshold
// for a total experience value.
type LevelFn func(experience uint32) (level, start, next uint32)

// SaveResult writes a finished match, awards and progression updates.
func (db *DB) SaveResult(result Result, levels LevelFn, maxExperience uint32) error {
	ctx, cancel := db.ctx()
	defer cancel()
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	resultID := newResultID()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO game_results (id, room_id, tracking_id, mode, sub_mode, map_name,
		    end_reason, first_team_score, second_team_score, winner, policy_version, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, UTC_TIMESTAMP(6))`,
		resultID, result.RoomID, result.TrackingID, result.Mode, result.SubMode, result.MapName,
		result.EndReason, result.FirstScore, result.SecondScore, result.Winner,
		result.PolicyVersion); err != nil {
		return mapError(err)
	}

	for _, player := range result.Players {
		var experienceBefore, lucciBefore uint32
		if err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(p.experience, 0), COALESCE(s.lucci, 0)
			 FROM characters c
			 LEFT JOIN character_progression p ON p.character_id = c.id
			 LEFT JOIN character_stats s ON s.character_id = c.id
			 WHERE c.id = ?`, player.CharacterID).Scan(&experienceBefore, &lucciBefore); err != nil {
			return err
		}
		// Both totals are clamped the same way the reference clamps experience
		// (min(total + gain, cap)). The baseline is normalized first: the result row
		// enforces `before + gain = after`, so a legacy value above the cap has to
		// be pinned before the gain is applied, or the insert is rejected.
		experienceBefore = NormalizeTotal(experienceBefore, maxExperience)
		lucciBefore = NormalizeTotal(lucciBefore, MaxLucci)
		experienceAfter, appliedGain := ClampGain(experienceBefore, player.ExperienceGain, maxExperience)
		lucciAfter, appliedLucci := ClampGain(lucciBefore, player.LucciGain, MaxLucci)
		levelAfter, levelStart, nextLevel := levels(experienceAfter)
		levelBefore, _, _ := levels(experienceBefore)
		// The result table only accepts levels 1..200.
		if levelAfter < 1 {
			levelAfter = 1
		}
		if levelAfter > 200 {
			levelAfter = 200
		}
		if levelBefore < 1 {
			levelBefore = 1
		}
		if levelBefore > 200 {
			levelBefore = 200
		}
		if levelStart > experienceAfter {
			levelStart = experienceAfter
		}
		if nextLevel < experienceAfter {
			nextLevel = experienceAfter
		}

		res, err := tx.ExecContext(ctx,
			`INSERT INTO game_result_players (game_result_id, slot, character_id, account_id, nickname,
			   team, bubble_count, pang_count, rescue_count, experience_before, experience_after,
			   lucci_before, lucci_after, cash, experience_gain, lucci_gain, level, level_start,
			   next_level, level_before)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?, ?)`,
			resultID, player.Slot, player.CharacterID, player.AccountID, player.Nickname,
			player.Team, player.Bubbles, player.Pangs, player.Rescues, experienceBefore,
			experienceAfter, lucciBefore, lucciAfter, appliedGain, appliedLucci,
			levelAfter, levelStart, nextLevel, levelBefore)
		if err != nil {
			return mapError(err)
		}
		playerID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO game_result_awards (game_result_player_id, score, grade, medal)
			 VALUES (?, ?, ?, ?)`, playerID, player.Score, player.Grade, player.Medal); err != nil {
			return mapError(err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE character_progression SET experience = ?, updated_at = UTC_TIMESTAMP(6)
			 WHERE character_id = ?`, experienceAfter, player.CharacterID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE character_stats SET lucci = ?, updated_at = UTC_TIMESTAMP(6)
			 WHERE character_id = ?`, lucciAfter, player.CharacterID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func newResultID() string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	return hex.EncodeToString(sum[:16])
}

// HashToken returns the SHA-256 hex digest stored for a login ticket.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return ErrDuplicate
	}
	if strings.Contains(err.Error(), "Duplicate entry") {
		return ErrDuplicate
	}
	return err
}

// PurgeAccount hard-deletes an account and every row that belongs to it:
// results, tickets, items, characters and the account itself. It exists for
// operator tooling and test cleanup; there is no undo.
func (db *DB) PurgeAccount(name string) (bool, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var accountID uint32
	err = tx.QueryRowContext(ctx, `SELECT id FROM accounts WHERE account_name = ?`, name).Scan(&accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, mapError(err)
	}
	// Foreign keys are RESTRICT from results, tickets and items onto characters
	// and accounts, so the dependants go first.
	for _, statement := range []string{
		`DELETE grp FROM game_result_players grp JOIN characters c ON c.id = grp.character_id WHERE c.account_id = ?`,
		`DELETE FROM login_tickets WHERE account_id = ?`,
		`DELETE ci FROM character_items ci JOIN characters c ON c.id = ci.character_id WHERE c.account_id = ?`,
		`DELETE FROM characters WHERE account_id = ?`,
		`DELETE FROM accounts WHERE id = ?`,
	} {
		if _, err := tx.ExecContext(ctx, statement, accountID); err != nil {
			return false, mapError(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// AccountNames lists account names for operator tooling.
func (db *DB) AccountNames() ([]string, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	rows, err := db.sql.QueryContext(ctx, `SELECT account_name FROM accounts ORDER BY id`)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}
