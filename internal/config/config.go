// Package config loads and validates the server configuration.
package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

// Network is the game (lobby) listener.
type Network struct {
	ListenAddress      string `json:"listenAddress"`
	ListenPort         uint16 `json:"listenPort"`
	MaxConnections     int    `json:"maxConnections"`
	IdleTimeoutSeconds int    `json:"idleTimeoutSeconds"`
}

// Relay is the peer relay listener.
type Relay struct {
	Enabled            bool   `json:"enabled"`
	ListenAddress      string `json:"listenAddress"`
	ListenPort         uint16 `json:"listenPort"`
	AdvertisedAddress  string `json:"advertisedAddress"`
	MaxConnections     int    `json:"maxConnections"`
	IdleTimeoutSeconds int    `json:"idleTimeoutSeconds"`
}

// Zombie holds the zombie battle's revive timing. The mode's rules are fixed
// (5 s opening, 180 s length, kind 104 ultra) and the respawn is the client's own:
// the server sends no respawn offer at all, it only announces the turn once the
// client's bar has had its time.
type Zombie struct {
	// HumanTurnMs is the wait between a human's death report and the notice that it
	// is a zombie now: the client draws its bar from the death animation, so this is
	// measured on a real client rather than derived from the respawn catalog.
	HumanTurnMs int `json:"humanTurnMs"`
	// ZombiniTurnMs is the same wait for a zombini coming back, measured separately
	// because its bar is longer.
	ZombiniTurnMs int `json:"zombiniTurnMs"`
}

// Database holds the MariaDB connection settings.
type Database struct {
	Host                  string `json:"host"`
	Port                  uint16 `json:"port"`
	DatabaseName          string `json:"databaseName"`
	Username              string `json:"username"`
	ConnectTimeoutSeconds int    `json:"connectTimeoutSeconds"`
	QueryTimeoutSeconds   int    `json:"queryTimeoutSeconds"`
	PoolSize              int    `json:"poolSize"`
	TLSMode               string `json:"tlsMode"`
	VerifyCertificate     bool   `json:"verifyCertificate"`
	CAFile                string `json:"caFile"`
	// Password is the database password. Leave it empty in committed sample
	// configurations and supply BF_DATABASE_PASSWORD instead; a value here wins
	// over the environment variable and must be protected with file mode 0600.
	Password string `json:"password"`
}

// Authentication is the launcher TLS endpoint.
type Authentication struct {
	Enabled         bool   `json:"enabled"`
	ListenAddress   string `json:"listenAddress"`
	ListenPort      uint16 `json:"listenPort"`
	CertificateFile string `json:"certificateFile"`
	PrivateKeyFile  string `json:"privateKeyFile"`
	GameHost        string `json:"gameHost"`
	// InviteCode gates in-launcher account creation; empty leaves registration
	// open to anyone who can reach the endpoint.
	InviteCode string `json:"inviteCode"`
	// RegistrationsPerHour and RegistrationsPerDay bound account creation per
	// client address. Zero keeps the defaults (5 per hour, 20 per day): every
	// registration costs one 64 MiB scrypt hash, so an unbounded endpoint is a
	// memory and CPU amplifier.
	RegistrationsPerHour int `json:"registrationsPerHour"`
	RegistrationsPerDay  int `json:"registrationsPerDay"`
}

// ResolveDatabasePassword returns the password to connect with. A value in the
// configuration wins; otherwise BF_DATABASE_PASSWORD is used. The environment
// value is trimmed so an export with a trailing newline still works, while the
// configured value is taken verbatim because it is explicit.
func (c Config) ResolveDatabasePassword() (string, error) {
	if c.Database.Password != "" {
		return c.Database.Password, nil
	}
	if password := strings.TrimSpace(os.Getenv(envDatabasePassword)); password != "" {
		return password, nil
	}
	return "", fmt.Errorf("database password is missing: set database.password or %s", envDatabasePassword)
}

// envDatabasePassword is the environment variable holding the database password.
const envDatabasePassword = "BF_DATABASE_PASSWORD"

// Logging configures the server log.
type Logging struct {
	Level            string `json:"level"`
	Directory        string `json:"directory"`
	MaxFileSizeBytes int64  `json:"maxFileSizeBytes"`
	MaxFiles         int    `json:"maxFiles"`
}

// Admin is the management HTTP endpoint. It is meant for the machine itself or a
// private network: bind it to loopback and reach it through a tunnel unless the
// network in between is trusted.
type Admin struct {
	Enabled       bool   `json:"enabled"`
	ListenAddress string `json:"listenAddress"`
	ListenPort    uint16 `json:"listenPort"`
	// Token is the single bearer token every request must carry. Keep it long
	// enough that guessing it is pointless.
	Token string `json:"token"`
}

// Config is the root server configuration.
type Config struct {
	GameMapsFile       string         `json:"gameMapsFile"`
	GameWeaponsFile    string         `json:"gameWeaponsFile"`
	GameDefenseFile    string         `json:"gameDefenseFile"`
	GameCharactersFile string         `json:"gameCharactersFile"`
	GameRespawnFile    string         `json:"gameRespawnFile"`
	ProgressionFile    string         `json:"progressionFile"`
	Network            Network        `json:"network"`
	Relay              Relay          `json:"relay"`
	Database           Database       `json:"database"`
	Authentication     Authentication `json:"authentication"`
	Logging            Logging        `json:"logging"`
	Zombie             Zombie         `json:"zombie"`
	Admin              Admin          `json:"admin"`
}

// AllowedDatabaseNames are the only database names the server accepts.
var AllowedDatabaseNames = map[string]bool{
	"bubblefighter_cn0261":       true,
	"bubblefighter_cn0261_tests": true,
}

// Load reads and validates a configuration file.
func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("cannot open configuration file: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("invalid configuration: %w", err)
	}
	if decoder.More() {
		return Config{}, fmt.Errorf("invalid configuration: trailing content")
	}
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Measured on a real client: the wait between a death report and the turn notice,
// one for a human becoming the king and a longer one for a zombini coming back.
const (
	defaultHumanTurnMs   = 12000
	defaultZombiniTurnMs = 18000
)

// applyDefaults fills the fields whose zero value means "not stated". A
// configuration written before the zombie mode existed carries no zombie block,
// and a zero wait would announce the turn the instant a player died.
func (c *Config) applyDefaults() {
	if c.Zombie.HumanTurnMs == 0 {
		c.Zombie.HumanTurnMs = defaultHumanTurnMs
	}
	if c.Zombie.ZombiniTurnMs == 0 {
		c.Zombie.ZombiniTurnMs = defaultZombiniTurnMs
	}
}

func (c Config) Validate() error {
	for name, value := range map[string]string{
		"gameMapsFile":       c.GameMapsFile,
		"gameWeaponsFile":    c.GameWeaponsFile,
		"gameDefenseFile":    c.GameDefenseFile,
		"gameCharactersFile": c.GameCharactersFile,
		"gameRespawnFile":    c.GameRespawnFile,
		"progressionFile":    c.ProgressionFile,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("configuration field %s must not be empty", name)
		}
	}
	if c.Network.ListenPort == 0 {
		return fmt.Errorf("network.listenPort must be between 1 and 65535")
	}
	if c.Network.MaxConnections < 1 || c.Network.MaxConnections > 65535 {
		return fmt.Errorf("network.maxConnections must be between 1 and 65535")
	}
	if c.Network.IdleTimeoutSeconds < 0 || c.Network.IdleTimeoutSeconds > 3600 {
		return fmt.Errorf("network.idleTimeoutSeconds must be between 0 and 3600 (0 disables the idle timeout)")
	}
	if c.Relay.Enabled {
		if c.Relay.ListenPort == 0 {
			return fmt.Errorf("relay.listenPort must be between 1 and 65535")
		}
		if c.Relay.MaxConnections < 1 || c.Relay.MaxConnections > 65535 {
			return fmt.Errorf("relay.maxConnections must be between 1 and 65535")
		}
		if c.Relay.IdleTimeoutSeconds < 0 || c.Relay.IdleTimeoutSeconds > 3600 {
			return fmt.Errorf("relay.idleTimeoutSeconds must be between 0 and 3600 (0 disables the idle timeout)")
		}
		if c.Relay.AdvertisedAddress != "" && net.ParseIP(c.Relay.AdvertisedAddress) == nil {
			return fmt.Errorf("relay.advertisedAddress must be an IPv4 address")
		}
	}
	if c.Admin.Enabled {
		if c.Admin.ListenAddress == "" {
			return fmt.Errorf("admin.listenAddress must not be empty")
		}
		if c.Admin.ListenPort == 0 {
			return fmt.Errorf("admin.listenPort must be between 1 and 65535")
		}
		if len(c.Admin.Token) < 16 {
			return fmt.Errorf("admin.token must be at least 16 characters")
		}
	}
	if c.Database.Host == "" {
		return fmt.Errorf("database.host must not be empty")
	}
	if c.Database.Port == 0 {
		return fmt.Errorf("database.port must be between 1 and 65535")
	}
	if !AllowedDatabaseNames[c.Database.DatabaseName] {
		return fmt.Errorf("database.databaseName must be bubblefighter_cn0261 or bubblefighter_cn0261_tests")
	}
	if c.Database.Username == "" {
		return fmt.Errorf("database.username must not be empty")
	}
	if c.Database.ConnectTimeoutSeconds < 1 || c.Database.ConnectTimeoutSeconds > 300 {
		return fmt.Errorf("database.connectTimeoutSeconds must be between 1 and 300")
	}
	if c.Database.QueryTimeoutSeconds < 1 || c.Database.QueryTimeoutSeconds > 300 {
		return fmt.Errorf("database.queryTimeoutSeconds must be between 1 and 300")
	}
	if c.Database.PoolSize < 1 || c.Database.PoolSize > 128 {
		return fmt.Errorf("database.poolSize must be between 1 and 128")
	}
	switch c.Database.TLSMode {
	case "require":
	case "disable":
		if c.Database.VerifyCertificate {
			return fmt.Errorf("database.verifyCertificate must be false when TLS is disabled")
		}
	default:
		return fmt.Errorf("database.tlsMode must be require or disable")
	}
	if c.Authentication.Enabled {
		if c.Authentication.ListenPort == 0 {
			return fmt.Errorf("authentication.listenPort must be between 1 and 65535")
		}
		if c.Authentication.CertificateFile == "" || c.Authentication.PrivateKeyFile == "" {
			return fmt.Errorf("authentication certificate and key files are required")
		}
		if net.ParseIP(c.Authentication.GameHost) == nil && c.Authentication.GameHost != "" {
			return fmt.Errorf("authentication.gameHost must be an IPv4 address")
		}
	}
	switch c.Logging.Level {
	case "trace", "debug", "info", "warn", "error", "critical", "off":
	default:
		return fmt.Errorf("logging.level is invalid")
	}
	if c.Logging.Directory == "" {
		return fmt.Errorf("logging.directory must not be empty")
	}
	if c.Logging.MaxFileSizeBytes < 4096 {
		return fmt.Errorf("logging.maxFileSizeBytes must be at least 4096")
	}
	if c.Logging.MaxFiles < 1 || c.Logging.MaxFiles > 64 {
		return fmt.Errorf("logging.maxFiles must be between 1 and 64")
	}
	// The zombie turns are announcements timed against the client's own bar; a
	// value outside this window is a typo rather than a preference.
	if c.Zombie.HumanTurnMs < 500 || c.Zombie.HumanTurnMs > 60000 {
		return fmt.Errorf("zombie.humanTurnMs must be between 500 and 60000")
	}
	if c.Zombie.ZombiniTurnMs < 500 || c.Zombie.ZombiniTurnMs > 60000 {
		return fmt.Errorf("zombie.zombiniTurnMs must be between 500 and 60000")
	}
	return nil
}

// Address formats the game listener address.
func (c Config) Address() string {
	return net.JoinHostPort(c.Network.ListenAddress, fmt.Sprint(c.Network.ListenPort))
}
