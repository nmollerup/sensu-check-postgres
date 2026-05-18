package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgpassfile"
	"github.com/jackc/pgx/v5"
	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

type Config struct {
	sensu.PluginConfig
	User     string
	Password string
	IniFile  string
	Hostname string
	Port     int
	Database string
	Sslmode  string
	Scheme   string
	Timeout  int
	Scope    string
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "metric-postgres-statsio",
			Short:    "postgres I/O stats metric",
			Keyspace: "",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[string]{
			Path:      "User",
			Argument:  "user",
			Shorthand: "u",
			Usage:     "postgres user to connect",
			Value:     &plugin.User,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "Password",
			Argument:  "password",
			Shorthand: "p",
			Usage:     "Password for user",
			Value:     &plugin.Password,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "pgpass file",
			Argument:  "pgpass",
			Shorthand: "f",
			Usage:     "Location of .pgpass file for access to postgres",
			Value:     &plugin.IniFile,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "port",
			Argument:  "port",
			Shorthand: "P",
			Usage:     "Port to connect to",
			Value:     &plugin.Port,
			Default:   5432,
		},
		&sensu.PluginConfigOption[string]{
			Path:     "hostname",
			Argument: "hostname",
			Usage:    "Hostname to login to",
			Value:    &plugin.Hostname,
			Default:  "localhost",
		},
		&sensu.PluginConfigOption[string]{
			Path:      "database",
			Argument:  "database",
			Shorthand: "d",
			Usage:     "Database name",
			Value:     &plugin.Database,
			Default:   "postgres",
		},
		&sensu.PluginConfigOption[string]{
			Path:      "sslmode",
			Argument:  "sslmode",
			Shorthand: "s",
			Usage:     "SSL mode for connecting to postgres",
			Value:     &plugin.Sslmode,
			Default:   "prefer",
		},
		&sensu.PluginConfigOption[string]{
			Path:     "scheme",
			Argument: "scheme",
			Usage:    "Metric naming scheme",
			Value:    &plugin.Scheme,
			Default:  "postgresql",
		},
		&sensu.PluginConfigOption[int]{
			Path:      "timeout",
			Argument:  "timeout",
			Shorthand: "T",
			Usage:     "Connection timeout (seconds)",
			Value:     &plugin.Timeout,
			Default:   10,
		},
		&sensu.PluginConfigOption[string]{
			Path:     "scope",
			Argument: "scope",
			Usage:    "Stats scope (user or all)",
			Value:    &plugin.Scope,
			Default:  "user",
		},
	}
)

func main() {
	metric := sensu.NewGoHandler(&plugin.PluginConfig, options, checkArgs, executeMetric)
	metric.Execute()
}

func checkArgs(_ *corev2.Event) error {
	if plugin.Port <= 1 || plugin.Port >= 65535 {
		return fmt.Errorf("invalid port, should be a value between 1 and 65535")
	}
	if plugin.IniFile != "" {
		if _, err := os.Stat(plugin.IniFile); os.IsNotExist(err) {
			return fmt.Errorf("unable to open the supplied config file %s", plugin.IniFile)
		}
	}
	if plugin.Scope != "user" && plugin.Scope != "all" {
		return fmt.Errorf("scope must be 'user' or 'all'")
	}
	return nil
}

func executeMetric(_ *corev2.Event) error {
	ctx := context.Background()
	timestamp := time.Now().Unix()

	var dbUser, dbPass, dbHost, dbDatabase string
	var dbPort int
	if plugin.IniFile != "" {
		iniFile, err := pgpassfile.ReadPassfile(plugin.IniFile)
		if err != nil {
			return fmt.Errorf("error parsing ini file: %v", err)
		}
		for _, entry := range iniFile.Entries {
			dbUser = entry.Username
			dbPass = entry.Password
			dbHost = entry.Hostname
			dbPort, _ = strconv.Atoi(entry.Port)
			dbDatabase = entry.Database
		}
	} else {
		dbUser = plugin.User
		dbPass = plugin.Password
		dbHost = plugin.Hostname
		dbPort = plugin.Port
		dbDatabase = plugin.Database
	}

	dataSourceName := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s&connect_timeout=%d",
		dbUser, dbPass, dbHost, dbPort, dbDatabase, plugin.Sslmode, plugin.Timeout)
	db, err := pgx.Connect(ctx, dataSourceName)
	if err != nil {
		return fmt.Errorf("error connecting to postgres: %v", err)
	}
	defer func() {
		_ = db.Close(ctx)
	}()

	query := fmt.Sprintf(`SELECT
		sum(heap_blks_read) AS heap_blks_read, sum(heap_blks_hit) AS heap_blks_hit,
		sum(idx_blks_read) AS idx_blks_read, sum(idx_blks_hit) AS idx_blks_hit,
		sum(toast_blks_read) AS toast_blks_read, sum(toast_blks_hit) AS toast_blks_hit,
		sum(tidx_blks_read) AS tidx_blks_read, sum(tidx_blks_hit) AS tidx_blks_hit
		FROM pg_statio_%s_tables`, plugin.Scope)

	var heapBlksRead, heapBlksHit, idxBlksRead, idxBlksHit int64
	var toastBlksRead, toastBlksHit, tidxBlksRead, tidxBlksHit int64

	err = db.QueryRow(ctx, query).Scan(
		&heapBlksRead, &heapBlksHit,
		&idxBlksRead, &idxBlksHit,
		&toastBlksRead, &toastBlksHit,
		&tidxBlksRead, &tidxBlksHit,
	)
	if err != nil {
		return fmt.Errorf("error querying statsio: %v", err)
	}

	fmt.Printf("%s.statsio.%s.heap_blks_read %d %d\n", plugin.Scheme, dbDatabase, heapBlksRead, timestamp)
	fmt.Printf("%s.statsio.%s.heap_blks_hit %d %d\n", plugin.Scheme, dbDatabase, heapBlksHit, timestamp)
	fmt.Printf("%s.statsio.%s.idx_blks_read %d %d\n", plugin.Scheme, dbDatabase, idxBlksRead, timestamp)
	fmt.Printf("%s.statsio.%s.idx_blks_hit %d %d\n", plugin.Scheme, dbDatabase, idxBlksHit, timestamp)
	fmt.Printf("%s.statsio.%s.toast_blks_read %d %d\n", plugin.Scheme, dbDatabase, toastBlksRead, timestamp)
	fmt.Printf("%s.statsio.%s.toast_blks_hit %d %d\n", plugin.Scheme, dbDatabase, toastBlksHit, timestamp)
	fmt.Printf("%s.statsio.%s.tidx_blks_read %d %d\n", plugin.Scheme, dbDatabase, tidxBlksRead, timestamp)
	fmt.Printf("%s.statsio.%s.tidx_blks_hit %d %d\n", plugin.Scheme, dbDatabase, tidxBlksHit, timestamp)

	return nil
}
