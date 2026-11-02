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
	ItemMode           ItemMode       `json:"itemMode"`
}

// ItemMode holds the item battle (client mode 3) options.
type ItemMode struct {
	// SpawnFromMapReport makes the server create the map boxes itself, on a timer
	// derived from the host's C702 supply report. It is off by default because the
	// boxes are the clients' job: they place them from the map's own resources and
	// refresh them on their own interval, and the 2015 clients also announce each
	// box over the relay. The reference wires the same server-side path up for these
	// builds, but it depends on the host's C702 report and that report does not
	// reach it on a real 2015 client, so in practice the server spawns nothing.
	// Turning this on duplicates the intent of that path (and needs C702 too); it
	// must stay off while the clients still place the boxes themselves or the map
	// would show two boxes per point.
	SpawnFromMapReport bool `json:"spawnFromMapReport"`
	// EnforceBoxColour refuses a pickup whose box colour is the other team's. The
	// colour is bound to the team (red/blue) and the clients refuse it themselves
	// (kr-812 sub_6378A0), so this only adds server-side authority. It is off by
	// default while the colour word's numbering is being confirmed on a real
	// client: a wrong mapping would make a player unable to take their own boxes,
	// which is worse than the client-only check the mode ships with.
	EnforceBoxColour bool `json:"enforceBoxColour"`
	// ServerGrantsItem makes the server tell a picker which item it now holds
	// (S2C gspAddItemToSlot). It is on by default because that is what the
	// reference does, but the 2015 client may grant the item itself as well: two
	// grants leave a phantom item that occupies a slot, cannot be used and stops
	// further pickups. Turn this off to let the client own the slot.
	ServerGrantsItem bool `json:"serverGrantsItem"`
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
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
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
	if c.Network.IdleTimeoutSeconds < 1 || c.Network.IdleTimeoutSeconds > 3600 {
		return fmt.Errorf("network.idleTimeoutSeconds must be between 1 and 3600")
	}
	if c.Relay.Enabled {
		if c.Relay.ListenPort == 0 {
			return fmt.Errorf("relay.listenPort must be between 1 and 65535")
		}
		if c.Relay.MaxConnections < 1 || c.Relay.MaxConnections > 65535 {
			return fmt.Errorf("relay.maxConnections must be between 1 and 65535")
		}
		if c.Relay.IdleTimeoutSeconds < 1 || c.Relay.IdleTimeoutSeconds > 3600 {
			return fmt.Errorf("relay.idleTimeoutSeconds must be between 1 and 3600")
		}
		if c.Relay.AdvertisedAddress != "" && net.ParseIP(c.Relay.AdvertisedAddress) == nil {
			return fmt.Errorf("relay.advertisedAddress must be an IPv4 address")
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
	return nil
}

// Address formats the game listener address.
func (c Config) Address() string {
	return net.JoinHostPort(c.Network.ListenAddress, fmt.Sprint(c.Network.ListenPort))
}
