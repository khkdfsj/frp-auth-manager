package database

import (
	"database/sql"
	"fmt"
	"time"

	"frp_auth/models"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func Init(dataSource string) error {
	var err error
	db, err = sql.Open("sqlite", dataSource)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := migrate(); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

func migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS admin_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			notes TEXT DEFAULT '',
			created_by INTEGER REFERENCES admin_users(id),
			is_active BOOLEAN DEFAULT 1,
			expires_at TIMESTAMP,
			traffic_limit BIGINT DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS port_permissions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token_id INTEGER REFERENCES tokens(id) ON DELETE CASCADE,
			port INTEGER NOT NULL,
			is_active BOOLEAN DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS port_config (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			port INTEGER UNIQUE NOT NULL,
			require_auth BOOLEAN DEFAULT 1,
			auth_mode TEXT DEFAULT 'token',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS port_ip_allowlist (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			port INTEGER NOT NULL,
			ip TEXT NOT NULL,
			notes TEXT DEFAULT '',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(port, ip)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_port_ip_allowlist_lookup ON port_ip_allowlist(port, ip)`,
		`CREATE TABLE IF NOT EXISTS ip_whitelist (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token_id INTEGER REFERENCES tokens(id),
			ip TEXT NOT NULL,
			port INTEGER NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ip_whitelist_lookup ON ip_whitelist(ip, port)`,
		`CREATE INDEX IF NOT EXISTS idx_ip_whitelist_expire ON ip_whitelist(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_ip_whitelist_token ON ip_whitelist(token_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tokens_token ON tokens(token)`,
		`CREATE INDEX IF NOT EXISTS idx_port_permissions_lookup ON port_permissions(token_id, port)`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("exec %q: %w", q[:40], err)
		}
	}
	// Add columns if missing (for upgrades)
	db.Exec("ALTER TABLE tokens ADD COLUMN notes TEXT DEFAULT ''")
	db.Exec("ALTER TABLE tokens ADD COLUMN traffic_limit BIGINT DEFAULT 0")
	// Migrate port_config: add auth_mode column, populate from require_auth
	db.Exec("ALTER TABLE port_config ADD COLUMN auth_mode TEXT DEFAULT 'token'")
	db.Exec("UPDATE port_config SET auth_mode = CASE WHEN require_auth THEN 'token' ELSE 'open' END WHERE auth_mode IS NULL OR auth_mode = ''")
	return nil
}

func GetAdminByUsername(username string) (*models.AdminUser, error) {
	u := &models.AdminUser{}
	err := db.QueryRow(
		"SELECT id, username, password_hash, created_at FROM admin_users WHERE username = ?",
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func CreateAdminUser(username, passwordHash string) error {
	_, err := db.Exec(
		"INSERT OR IGNORE INTO admin_users (username, password_hash) VALUES (?, ?)",
		username, passwordHash,
	)
	return err
}

func CreateToken(token, name, notes string, createdBy int, expiresAt *time.Time, trafficLimit int64) (*models.Token, error) {
	res, err := db.Exec(
		"INSERT INTO tokens (token, name, notes, created_by, expires_at, traffic_limit) VALUES (?, ?, ?, ?, ?, ?)",
		token, name, notes, createdBy, expiresAt, trafficLimit,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return GetTokenByID(int(id))
}

func GetTokenByID(id int) (*models.Token, error) {
	t := &models.Token{}
	var expiresAt sql.NullTime
	var notes sql.NullString
	err := db.QueryRow(
		"SELECT id, token, name, COALESCE(notes,''), created_by, is_active, expires_at, COALESCE(traffic_limit,0), created_at FROM tokens WHERE id = ?",
		id,
	).Scan(&t.ID, &t.Token, &t.Name, &notes, &t.CreatedBy, &t.IsActive, &expiresAt, &t.TrafficLimit, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	t.Notes = notes.String
	if expiresAt.Valid {
		t.ExpiresAt = &expiresAt.Time
	}
	return t, nil
}

func GetTokenByValue(token string) (*models.Token, error) {
	t := &models.Token{}
	var expiresAt sql.NullTime
	var notes sql.NullString
	err := db.QueryRow(
		"SELECT id, token, name, COALESCE(notes,''), created_by, is_active, expires_at, COALESCE(traffic_limit,0), created_at FROM tokens WHERE token = ?",
		token,
	).Scan(&t.ID, &t.Token, &t.Name, &notes, &t.CreatedBy, &t.IsActive, &expiresAt, &t.TrafficLimit, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	t.Notes = notes.String
	if expiresAt.Valid {
		t.ExpiresAt = &expiresAt.Time
	}
	return t, nil
}

func ListTokens() ([]models.Token, error) {
	rows, err := db.Query(`
		SELECT t.id, t.token, t.name, COALESCE(t.notes,''), t.created_by, t.is_active, t.expires_at, COALESCE(t.traffic_limit,0), t.created_at,
			COALESCE((SELECT COUNT(*) FROM ip_whitelist w WHERE w.token_id = t.id), 0) as activation_count
		FROM tokens t ORDER BY t.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []models.Token
	for rows.Next() {
		var t models.Token
		var expiresAt sql.NullTime
		var notes sql.NullString
		var activationCount int64
		if err := rows.Scan(&t.ID, &t.Token, &t.Name, &notes, &t.CreatedBy, &t.IsActive, &expiresAt, &t.TrafficLimit, &t.CreatedAt, &activationCount); err != nil {
			return nil, err
		}
		t.Notes = notes.String
		if expiresAt.Valid {
			t.ExpiresAt = &expiresAt.Time
		}
		tokens = append(tokens, t)
	}
	return tokens, nil
}

func UpdateToken(id int, name, notes *string, isActive *bool, expiresAt *time.Time, trafficLimit *int64) error {
	if name != nil {
		if _, err := db.Exec("UPDATE tokens SET name = ? WHERE id = ?", *name, id); err != nil {
			return err
		}
	}
	if notes != nil {
		if _, err := db.Exec("UPDATE tokens SET notes = ? WHERE id = ?", *notes, id); err != nil {
			return err
		}
	}
	if isActive != nil {
		if _, err := db.Exec("UPDATE tokens SET is_active = ? WHERE id = ?", *isActive, id); err != nil {
			return err
		}
	}
	if expiresAt != nil {
		if _, err := db.Exec("UPDATE tokens SET expires_at = ? WHERE id = ?", *expiresAt, id); err != nil {
			return err
		}
	}
	if trafficLimit != nil {
		if _, err := db.Exec("UPDATE tokens SET traffic_limit = ? WHERE id = ?", *trafficLimit, id); err != nil {
			return err
		}
	}
	return nil
}

func DeleteToken(id int) error {
	_, err := db.Exec("DELETE FROM tokens WHERE id = ?", id)
	return err
}

func GetTokenActivationCount(tokenID int) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM ip_whitelist WHERE token_id = ?", tokenID).Scan(&count)
	return count, err
}

// Port permissions
func AddPortPermission(tokenID, port int) error {
	_, err := db.Exec(
		"INSERT OR IGNORE INTO port_permissions (token_id, port) VALUES (?, ?)",
		tokenID, port,
	)
	return err
}

func RemovePortPermission(id int) error {
	_, err := db.Exec("DELETE FROM port_permissions WHERE id = ?", id)
	return err
}

func RemovePortPermissionByTokenPort(tokenID, port int) error {
	_, err := db.Exec("DELETE FROM port_permissions WHERE token_id = ? AND port = ?", tokenID, port)
	return err
}

func GetTokenPortPermissions(tokenID int) ([]models.PortPermission, error) {
	rows, err := db.Query(
		"SELECT id, token_id, port, is_active, created_at FROM port_permissions WHERE token_id = ? AND is_active = 1",
		tokenID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []models.PortPermission
	for rows.Next() {
		var p models.PortPermission
		if err := rows.Scan(&p.ID, &p.TokenID, &p.Port, &p.IsActive, &p.CreatedAt); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, nil
}

func GetAllPortPermissions() ([]models.PortPermission, error) {
	rows, err := db.Query("SELECT id, token_id, port, is_active, created_at FROM port_permissions ORDER BY token_id, port")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []models.PortPermission
	for rows.Next() {
		var p models.PortPermission
		if err := rows.Scan(&p.ID, &p.TokenID, &p.Port, &p.IsActive, &p.CreatedAt); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, nil
}

// Port config
func SetPortConfig(port int, authMode string) error {
	_, err := db.Exec(
		"INSERT INTO port_config (port, auth_mode, require_auth) VALUES (?, ?, ?) ON CONFLICT(port) DO UPDATE SET auth_mode = ?, require_auth = ?",
		port, authMode, authMode != "open", authMode, authMode != "open",
	)
	return err
}

func GetPortConfig(port int) (*models.PortConfig, error) {
	p := &models.PortConfig{}
	err := db.QueryRow(
		"SELECT id, port, COALESCE(auth_mode,'token'), created_at FROM port_config WHERE port = ?",
		port,
	).Scan(&p.ID, &p.Port, &p.AuthMode, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func ListPortConfigs() ([]models.PortConfig, error) {
	rows, err := db.Query("SELECT id, port, COALESCE(auth_mode,'token'), created_at FROM port_config ORDER BY port")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []models.PortConfig
	for rows.Next() {
		var p models.PortConfig
		if err := rows.Scan(&p.ID, &p.Port, &p.AuthMode, &p.CreatedAt); err != nil {
			return nil, err
		}
		configs = append(configs, p)
	}
	return configs, nil
}

func DeletePortConfig(port int) error {
	_, err := db.Exec("DELETE FROM port_config WHERE port = ?", port)
	return err
}

// Port IP allowlist
func AddPortIPAllow(port int, ip, notes string) error {
	_, err := db.Exec(
		"INSERT OR IGNORE INTO port_ip_allowlist (port, ip, notes) VALUES (?, ?, ?)",
		port, ip, notes,
	)
	return err
}

func RemovePortIPAllow(id int) error {
	_, err := db.Exec("DELETE FROM port_ip_allowlist WHERE id = ?", id)
	return err
}

func ListPortIPAllowlist(port int) ([]models.PortIPAllowEntry, error) {
	var rows *sql.Rows
	var err error
	if port > 0 {
		rows, err = db.Query("SELECT id, port, ip, COALESCE(notes,''), created_at FROM port_ip_allowlist WHERE port = ? ORDER BY created_at DESC", port)
	} else {
		rows, err = db.Query("SELECT id, port, ip, COALESCE(notes,''), created_at FROM port_ip_allowlist ORDER BY port, created_at DESC")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.PortIPAllowEntry
	for rows.Next() {
		var e models.PortIPAllowEntry
		if err := rows.Scan(&e.ID, &e.Port, &e.IP, &e.Notes, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func CheckPortIPAllowed(ip string, port int) (bool, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM port_ip_allowlist WHERE port = ? AND ip = ?",
		port, ip,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func AddIPWhitelist(tokenID int, ip string, port int, expiresAt time.Time) error {
	_, err := db.Exec(
		"INSERT INTO ip_whitelist (token_id, ip, port, expires_at) VALUES (?, ?, ?, ?)",
		tokenID, ip, port, expiresAt,
	)
	return err
}

func CheckIPAuthorized(ip string, port int) (bool, error) {
	db.Exec("DELETE FROM ip_whitelist WHERE expires_at < ?", time.Now())

	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM ip_whitelist WHERE ip = ? AND port = ? AND expires_at > ?",
		ip, port, time.Now(),
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func CleanExpiredWhitelist() {
	db.Exec("DELETE FROM ip_whitelist WHERE expires_at < ?", time.Now())
}

func TokenHasPortPermission(tokenID, port int) (bool, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM tokens t
		 JOIN port_permissions pp ON t.id = pp.token_id
		 WHERE t.id = ? AND pp.port = ? AND t.is_active = 1 AND pp.is_active = 1
		 AND (t.expires_at IS NULL OR t.expires_at > ?)`,
		tokenID, port, time.Now(),
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
