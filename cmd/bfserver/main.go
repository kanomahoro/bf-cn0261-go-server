// Command bfserver runs the BubbleFighter CN0261 server.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/config"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/features"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/game"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/logging"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/net"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/store"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/world"
)

// Version is the release version shown in the startup banner. Override it at
// build time without editing the source:
//
//	go build -ldflags "-X main.Version=0.2.0" ./cmd/bfserver
var Version = "0.1.2"

// Author is the signature printed with the version.
const Author = "rurudoo"

// banner is the one-line identification printed at every start.
func banner() string {
	return "BubbleFighter CN0261 Go Server v" + Version + "  By:" + Author
}

// usageText describes the real flag set; keep it in sync with the flags below.
func usageText() string {
	return banner() + `

Usage: bfserver [flags]

  --version                print the server version and exit
  --config <path>          configuration file (default config/server.json)
  --check-config           validate the configuration and exit, without touching the database
  --create-account <name>  create a game account
  --issue-ticket <name>    issue a one-time login ticket for an existing account
  --nickname <character>   character to create with --create-account
  --character-type <1|2>   starter character type: 1=皮蛋(Pidan) 2=黑妞(Heiniu), default 1
  --password <password>    account password for the two account commands above

Database password: database.password in the configuration, or BF_DATABASE_PASSWORD.
Account password: --password, or BF_ACCOUNT_PASSWORD environment variable.
`
}

// ticketRetention is how long an expired login ticket is kept before the
// startup purge removes it. Zero removes every expired ticket immediately;
// raise it if recently expired rows are needed for diagnostics.
const ticketRetention = time.Duration(0)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Startup error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("bfserver", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	versionOnly := flags.Bool("version", false, "print the server version and exit")
	configPath := flags.String("config", "config/server.json", "configuration file")
	checkOnly := flags.Bool("check-config", false, "validate configuration only")
	createAccount := flags.String("create-account", "", "create a game account")
	issueTicket := flags.String("issue-ticket", "", "issue a one-time login ticket")
	nickname := flags.String("nickname", "", "character name to create with --create-account")
	accountPasswordFlag := flags.String("password", "",
		"account password for --create-account/--issue-ticket (falls back to BF_ACCOUNT_PASSWORD)")
	characterType := flags.Uint("character-type", 1, "starter character type for --nickname (1=皮蛋/Pidan, 2=黑妞/Heiniu)")
	// The default flag usage printer is replaced by usageText below, so a help
	// request shows one block instead of the flag package's own listing.
	flags.Usage = func() {}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(os.Stdout, usageText())
			return nil
		}
		fmt.Fprint(os.Stderr, usageText())
		return err
	}
	if len(flags.Args()) != 0 {
		fmt.Fprint(os.Stderr, usageText())
		return fmt.Errorf("invalid command line: unexpected arguments %v", flags.Args())
	}
	if *versionOnly {
		fmt.Println(banner())
		return nil
	}
	if *createAccount != "" && *issueTicket != "" {
		return fmt.Errorf("only one account operation may be specified")
	}
	if *nickname != "" && *createAccount == "" {
		return fmt.Errorf("--nickname requires --create-account")
	}
	if *nickname == "" && *characterType != 1 {
		return fmt.Errorf("--character-type requires --nickname")
	}
	if *accountPasswordFlag != "" && *createAccount == "" && *issueTicket == "" {
		return fmt.Errorf("--password requires --create-account or --issue-ticket")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	if *checkOnly {
		if *createAccount != "" || *issueTicket != "" || *nickname != "" || *accountPasswordFlag != "" {
			return fmt.Errorf("account operations cannot be combined with --check-config")
		}
		if _, err := world.LoadMaps(cfg.GameMapsFile); err != nil {
			return err
		}
		if _, err := world.LoadWeapons(cfg.GameWeaponsFile); err != nil {
			return err
		}
		if _, err := world.LoadDefense(cfg.GameDefenseFile); err != nil {
			return err
		}
		if _, err := world.LoadCharacters(cfg.GameCharactersFile); err != nil {
			return err
		}
		if _, err := world.LoadRespawn(cfg.GameRespawnFile); err != nil {
			return err
		}
		if _, err := world.LoadProgression(cfg.ProgressionFile); err != nil {
			return err
		}
		fmt.Println(banner())
		fmt.Println("Configuration is valid. Database connectivity has not been checked.")
		return nil
	}

	password, err := cfg.ResolveDatabasePassword()
	if err != nil {
		return err
	}

	if *createAccount != "" || *issueTicket != "" {
		name := strings.ToLower(strings.TrimSpace(*createAccount + *issueTicket))
		// -password wins over the environment; the environment keeps the password
		// out of shell history for anyone who prefers that.
		accountPassword := *accountPasswordFlag
		if accountPassword == "" {
			accountPassword = os.Getenv("BF_ACCOUNT_PASSWORD")
		}
		if err := features.CheckAccountName(name); err != nil {
			return err
		}
		db, err := store.Open(cfg.Database, password)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := db.CheckConnection(); err != nil {
			return err
		}
		if err := db.CheckSchema(); err != nil {
			return err
		}
		// The -create-account path creates a character too, so it needs the
		// grenade catalogue rows for the starter loadout to include one.
		if err := db.EnsureGrenadeItem(); err != nil {
			return err
		}
		accounts := features.NewAccounts(db)
		if *createAccount != "" {
			if *nickname == "" {
				id, err := accounts.Create(name, accountPassword)
				if err != nil {
					return err
				}
				return printJSON(map[string]any{"accountId": id, "accountName": name})
			}
			character, err := accounts.Register(name, accountPassword, *nickname, uint32(*characterType))
			if err != nil {
				return err
			}
			return printJSON(map[string]any{
				"accountId": character.AccountID, "accountName": name,
				"characterId": character.ID, "nickname": character.Nickname,
				"characterType": character.CharacterType,
			})
		}
		ticket, err := accounts.IssueTicketFor(name, accountPassword)
		if err != nil {
			return err
		}
		return printJSON(map[string]any{"loginTicket": ticket, "expiresInSeconds": 120})
	}

	logger, err := logging.Initialize(cfg.Logging)
	if err != nil {
		return err
	}
	// Identify the build first, so a failed start still names the binary and the
	// version it was running.
	logger.Info(banner())
	logger.Info("[Application] environment",
		"go", runtime.Version(),
		"config", *configPath,
		"database", fmt.Sprintf("%s@%s:%d/%s", cfg.Database.Username, cfg.Database.Host,
			cfg.Database.Port, cfg.Database.DatabaseName),
		"gamePort", cfg.Network.ListenPort,
		"authPort", cfg.Authentication.ListenPort,
		"relayPort", cfg.Relay.ListenPort,
		"logging", cfg.Logging.Level)

	db, err := store.Open(cfg.Database, password)
	if err != nil {
		return err
	}
	defer db.Close()

	server, err := game.New(cfg, db, logger)
	if err != nil {
		return err
	}

	logger.Info("[Database] checking connection and schema version")
	if err := db.CheckConnection(); err != nil {
		return err
	}
	if err := db.CheckSchema(); err != nil {
		return err
	}
	// The grenade's catalogue rows are seeded here rather than by a migration:
	// CheckSchema pins the applied migration list exactly, and the insert is
	// idempotent so an already-seeded database is unaffected.
	logger.Info("[Database] ensuring the grenade item definition")
	if err := db.EnsureGrenadeItem(); err != nil {
		logger.Error("[Database] grenade item definition could not be ensured; new characters would start without grenades", "error", err)
		return err
	}

	logger.Info("[Database] purging expired login tickets", "retentionSeconds", int(ticketRetention.Seconds()))
	if removed, err := db.PurgeExpiredTickets(ticketRetention); err != nil {
		// Housekeeping must not keep the server down; the next start retries.
		logger.Warn("[Database] expired ticket purge failed", "error", err)
	} else {
		logger.Info("[Database] expired login tickets purged", "removed", removed)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errs := make(chan error, 4)
	gameListener := net.NewGame(cfg, server, logger)
	go func() { errs <- gameListener.Serve(ctx) }()

	if cfg.Relay.Enabled {
		relay := net.NewRelay(cfg.Relay, logger)
		server.SetRelay(relay)
		// Relay traffic carries in-match state (character reports, defense
		// confirmations, life events), so the game logic observes it too.
		relay.SetPacketHandler(server.HandleRelayPacket)
		// A relay connection that dies mid-match means the member cannot see or
		// be seen any more, so the round is cancelled exactly like a disconnect.
		relay.SetDisconnectHandler(server.RelayDisconnected)
		go func() { errs <- relay.Serve(ctx) }()
	}
	if cfg.Authentication.Enabled {
		auth := net.NewAuth(cfg.Authentication, features.NewAccounts(db), cfg.Network.ListenPort, logger)
		go func() { errs <- auth.Serve(ctx) }()
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	exitCode := 0
	select {
	case sig := <-signals:
		logger.Info("[Application] received signal", "signal", sig.String())
	case err := <-errs:
		if err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("[Application] startup failed", "error", err,
				"hint", "check database authentication, TLS, schema and listener")
			exitCode = 1
		}
	}
	cancel()
	logger.Info(fmt.Sprintf("[Application] stopped exitCode=%d", exitCode))
	if exitCode != 0 {
		return fmt.Errorf("event loop failed; check the log for details")
	}
	return nil
}

func printJSON(value map[string]any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	return nil
}
