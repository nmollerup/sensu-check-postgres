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
			Name:     "metric-postgres-statstable",
			Short:    "postgres table stats metric",
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
		sum(seq_scan) AS seq_scan, sum(seq_tup_read) AS seq_tup_read,
		sum(idx_scan) AS idx_scan, sum(idx_tup_fetch) AS idx_tup_fetch,
		sum(n_tup_ins) AS n_tup_ins, sum(n_tup_upd) AS n_tup_upd, sum(n_tup_del) AS n_tup_del,
		sum(n_tup_hot_upd) AS n_tup_hot_upd, sum(n_live_tup) AS n_live_tup, sum(n_dead_tup) AS n_dead_tup
		FROM pg_stat_%s_tables`, plugin.Scope)

	var seqScan, seqTupRead, idxScan, idxTupFetch int64
	var nTupIns, nTupUpd, nTupDel, nTupHotUpd int64
	var nLiveTup, nDeadTup int64

	err = db.QueryRow(ctx, query).Scan(
		&seqScan, &seqTupRead,
		&idxScan, &idxTupFetch,
		&nTupIns, &nTupUpd, &nTupDel,
		&nTupHotUpd, &nLiveTup, &nDeadTup,
	)
	if err != nil {
		return fmt.Errorf("error querying statstable: %v", err)
	}

	fmt.Printf("%s.statstable.%s.seq_scan %d %d\n", plugin.Scheme, dbDatabase, seqScan, timestamp)
	fmt.Printf("%s.statstable.%s.seq_tup_read %d %d\n", plugin.Scheme, dbDatabase, seqTupRead, timestamp)
	fmt.Printf("%s.statstable.%s.idx_scan %d %d\n", plugin.Scheme, dbDatabase, idxScan, timestamp)
	fmt.Printf("%s.statstable.%s.idx_tup_fetch %d %d\n", plugin.Scheme, dbDatabase, idxTupFetch, timestamp)
	fmt.Printf("%s.statstable.%s.n_tup_ins %d %d\n", plugin.Scheme, dbDatabase, nTupIns, timestamp)
	fmt.Printf("%s.statstable.%s.n_tup_upd %d %d\n", plugin.Scheme, dbDatabase, nTupUpd, timestamp)
	fmt.Printf("%s.statstable.%s.n_tup_del %d %d\n", plugin.Scheme, dbDatabase, nTupDel, timestamp)
	fmt.Printf("%s.statstable.%s.n_tup_hot_upd %d %d\n", plugin.Scheme, dbDatabase, nTupHotUpd, timestamp)
	fmt.Printf("%s.statstable.%s.n_live_tup %d %d\n", plugin.Scheme, dbDatabase, nLiveTup, timestamp)
	fmt.Printf("%s.statstable.%s.n_dead_tup %d %d\n", plugin.Scheme, dbDatabase, nDeadTup, timestamp)

	return nil
}
