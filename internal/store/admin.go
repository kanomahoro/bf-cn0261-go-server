package store

import (
	"context"
	"database/sql"
	"time"
)

// AdminCharacter is one player row the console shows: the account it belongs to,
// the character, its wallet and its progress.
type AdminCharacter struct {
	AccountID     uint32
	AccountName   string
	Disabled      bool
	CharacterID   uint32
	Nickname      string
	CharacterType uint32
	Experience    uint32
	Lucci         uint32
	Cash          uint32
	CreatedAt     time.Time
}

// SearchCharacters finds players by account name, nickname, account id or
// character id. The console calls it for both the search box and the detail view.
func (db *DB) SearchCharacters(term string, limit int) ([]AdminCharacter, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	like := "%" + term + "%"
	rows, err := db.sql.QueryContext(ctx,
		`SELECT a.id, a.account_name, a.disabled,
		        c.id, c.nickname, c.character_type,
		        COALESCE(p.experience, 0), COALESCE(s.lucci, 0), COALESCE(s.cash, 0), c.created_at
		 FROM characters c
		 JOIN accounts a ON a.id = c.account_id
		 LEFT JOIN character_progression p ON p.character_id = c.id
		 LEFT JOIN character_stats s ON s.character_id = c.id
		 WHERE a.account_name LIKE ? OR c.nickname LIKE ? OR CAST(a.id AS CHAR) = ? OR CAST(c.id AS CHAR) = ?
		 ORDER BY c.id
		 LIMIT ?`, like, like, term, term, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminCharacter
	for rows.Next() {
		var entry AdminCharacter
		if err := rows.Scan(&entry.AccountID, &entry.AccountName, &entry.Disabled,
			&entry.CharacterID, &entry.Nickname, &entry.CharacterType,
			&entry.Experience, &entry.Lucci, &entry.Cash, &entry.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

// SetBalance writes a character's wallet. It is the console's only wallet write:
// changing the database is enough, because the server reloads a balance at login,
// at a shop purchase and after a match settles.
func (db *DB) SetBalance(characterID, lucci, cash uint32) error {
	ctx, cancel := db.ctx()
	defer cancel()
	res, err := db.sql.ExecContext(ctx,
		`UPDATE character_stats SET lucci = ?, cash = ?, updated_at = UTC_TIMESTAMP(6)
		 WHERE character_id = ?`, lucci, cash, characterID)
	if err != nil {
		return mapError(err)
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return ErrNotFound
	}
	return nil
}

// SetAccountDisabled bans or unbans an account.
func (db *DB) SetAccountDisabled(accountID uint32, disabled bool) error {
	ctx, cancel := db.ctx()
	defer cancel()
	res, err := db.sql.ExecContext(ctx,
		`UPDATE accounts SET disabled = ?, updated_at = UTC_TIMESTAMP(6) WHERE id = ?`,
		disabled, accountID)
	if err != nil {
		return mapError(err)
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return ErrNotFound
	}
	return nil
}

// RevokeLoginTickets burns every unspent ticket of an account, so a ban cannot be
// walked around with a ticket that was handed out before it.
func (db *DB) RevokeLoginTickets(accountID uint32) (int64, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	res, err := db.sql.ExecContext(ctx,
		`UPDATE login_tickets SET consumed_at = UTC_TIMESTAMP(6)
		 WHERE account_id = ? AND consumed_at IS NULL`, accountID)
	if err != nil {
		return 0, mapError(err)
	}
	return res.RowsAffected()
}

// AdminResult is one finished match as the console lists it.
type AdminResult struct {
	ID          string
	RoomID      uint16
	Mode        uint32
	MapName     string
	FirstScore  uint16
	SecondScore uint16
	Winner      uint8
	EndReason   uint8
	CompletedAt time.Time
}

// AdminResultPlayer is one seat of a finished match.
type AdminResultPlayer struct {
	Slot           uint8
	Nickname       string
	Team           uint8
	BubbleCount    uint16
	PangCount      uint16
	RescueCount    uint16
	ExperienceGain uint32
	LucciGain      uint32
	Level          uint32
}

// ListResults pages through finished matches, newest first. A mode of 0 lists
// every mode.
func (db *DB) ListResults(mode uint32, limit, offset int) ([]AdminResult, int, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	where, args := "", []any{}
	if mode != 0 {
		where = " WHERE mode = ?"
		args = append(args, mode)
	}
	var total int
	if err := db.sql.QueryRowContext(ctx, "SELECT COUNT(*) FROM game_results"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := db.sql.QueryContext(ctx,
		`SELECT id, room_id, mode, map_name, first_team_score, second_team_score, winner, end_reason, completed_at
		 FROM game_results`+where+` ORDER BY completed_at DESC LIMIT ? OFFSET ?`,
		append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []AdminResult
	for rows.Next() {
		var entry AdminResult
		if err := rows.Scan(&entry.ID, &entry.RoomID, &entry.Mode, &entry.MapName,
			&entry.FirstScore, &entry.SecondScore, &entry.Winner, &entry.EndReason, &entry.CompletedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, entry)
	}
	return out, total, rows.Err()
}

// ResultPlayers returns the seats of one finished match, in slot order.
func (db *DB) ResultPlayers(id string) ([]AdminResultPlayer, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	rows, err := db.sql.QueryContext(ctx,
		`SELECT slot, nickname, team, bubble_count, pang_count, rescue_count,
		        experience_gain, lucci_gain, level
		 FROM game_result_players WHERE game_result_id = ? ORDER BY slot`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminResultPlayer
	for rows.Next() {
		var entry AdminResultPlayer
		if err := rows.Scan(&entry.Slot, &entry.Nickname, &entry.Team, &entry.BubbleCount,
			&entry.PangCount, &entry.RescueCount, &entry.ExperienceGain, &entry.LucciGain,
			&entry.Level); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

// SchemaVersion is one applied migration row.
type SchemaVersion struct {
	Version uint32
	Name    string
	Applied time.Time
}

// SchemaVersions lists the applied migrations, which is what the console shows
// next to the build information.
func (db *DB) SchemaVersions() ([]SchemaVersion, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	rows, err := db.sql.QueryContext(ctx,
		`SELECT version, name, applied_at FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SchemaVersion
	for rows.Next() {
		var entry SchemaVersion
		if err := rows.Scan(&entry.Version, &entry.Name, &entry.Applied); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

// PoolStats exposes the connection pool counters.
func (db *DB) PoolStats() sql.DBStats { return db.sql.Stats() }

// Ping measures one round trip to the database.
func (db *DB) Ping(ctx context.Context) (time.Duration, error) {
	started := time.Now()
	if err := db.sql.PingContext(ctx); err != nil {
		return 0, err
	}
	return time.Since(started), nil
}
