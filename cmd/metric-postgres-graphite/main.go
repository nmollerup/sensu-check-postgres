package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/jackc/pgpassfile"
	"github.com/jackc/pgx/v5"
	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
	"nmollerup/sensu-check-postgres/internal/pgutil"
)

type Config struct {
	sensu.PluginConfig
	User       string
	Password   string
	IniFile    string
	MasterHost string
	SlaveHost  string
	Port       int
	Database   string
	Sslmode    string
	Scheme     string
	Timeout    int
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "metric-postgres-graphite",
			Short:    "postgres replication lag metric",
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
		&sensu.PluginConfigOption[string]{
			Path:      "master_host",
			Argument:  "master-host",
			Shorthand: "m",
			Usage:     "PostgreSQL master host",
			Value:     &plugin.MasterHost,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "slave_host",
			Argument:  "slave-host",
			Shorthand: "s",
			Usage:     "PostgreSQL slave host",
			Value:     &plugin.SlaveHost,
			Default:   "localhost",
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
			Path:      "database",
			Argument:  "database",
			Shorthand: "d",
			Usage:     "Database schema to connect to",
			Value:     &plugin.Database,
			Default:   "postgres",
		},
		&sensu.PluginConfigOption[string]{
			Path:      "sslmode",
			Argument:  "sslmode",
			Shorthand: "S",
			Usage:     "SSL mode for connecting to postgres",
			Value:     &plugin.Sslmode,
			Default:   "prefer",
		},
		&sensu.PluginConfigOption[string]{
			Path:     "scheme",
			Argument: "scheme",
			Usage:    "Metric naming scheme",
			Value:    &plugin.Scheme,
			Default:  "postgresql.replication_lag",
		},
		&sensu.PluginConfigOption[int]{
			Path:      "timeout",
			Argument:  "timeout",
			Shorthand: "T",
			Usage:     "Connection timeout (seconds)",
			Value:     &plugin.Timeout,
			Default:   10,
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
	if plugin.MasterHost == "" {
		return fmt.Errorf("master-host is required")
	}
	return nil
}

func executeMetric(_ *corev2.Event) error {
	ctx := context.Background()
	timestamp := time.Now().Unix()

	var dbUser, dbPass, dbDatabase string
	var dbPort int
	if plugin.IniFile != "" {
		iniFile, err := pgpassfile.ReadPassfile(plugin.IniFile)
		if err != nil {
			return fmt.Errorf("error parsing ini file: %v", err)
		}
		for _, entry := range iniFile.Entries {
			dbUser = entry.Username
			dbPass = entry.Password
			dbPort, _ = strconv.Atoi(entry.Port)
			dbDatabase = entry.Database
		}
	} else {
		dbUser = plugin.User
		dbPass = plugin.Password
		dbPort = plugin.Port
		dbDatabase = plugin.Database
	}

	// Connect to master
	masterDSN := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s&connect_timeout=%d",
		dbUser, dbPass, plugin.MasterHost, dbPort, dbDatabase, plugin.Sslmode, plugin.Timeout)
	connMaster, err := pgx.Connect(ctx, masterDSN)
	if err != nil {
		return fmt.Errorf("error connecting to master postgres: %v", err)
	}
	defer func() {
		_ = connMaster.Close(ctx)
	}()

	// Check PostgreSQL version
	isPG9, err := pgutil.CheckVersionNewerThanPostgres9(ctx, connMaster)
	if err != nil {
		return fmt.Errorf("error checking postgres version: %v", err)
	}

	// Get master WAL position
	var master string
	var walQuery string
	if isPG9 {
		walQuery = "SELECT pg_current_xlog_location()"
	} else {
		walQuery = "SELECT pg_current_wal_lsn()"
	}
	err = connMaster.QueryRow(ctx, walQuery).Scan(&master)
	if err != nil {
		return fmt.Errorf("error querying master WAL position: %v", err)
	}

	// Get WAL segment size
	var walSegmentSize string
	err = connMaster.QueryRow(ctx, "SHOW wal_segment_size").Scan(&walSegmentSize)
	if err != nil {
		return fmt.Errorf("error querying wal_segment_size: %v", err)
	}

	re := regexp.MustCompile(`\d+`)
	sizeStr := re.FindString(walSegmentSize)
	if sizeStr == "" {
		return fmt.Errorf("unable to parse wal_segment_size: %s", walSegmentSize)
	}
	segSize, _ := strconv.ParseInt(sizeStr, 10, 64)
	segBytes := segSize << 20

	_ = connMaster.Close(ctx)

	// Connect to slave
	slaveDSN := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s&connect_timeout=%d",
		dbUser, dbPass, plugin.SlaveHost, dbPort, dbDatabase, plugin.Sslmode, plugin.Timeout)
	connSlave, err := pgx.Connect(ctx, slaveDSN)
	if err != nil {
		return fmt.Errorf("error connecting to slave postgres: %v", err)
	}
	defer func() {
		_ = connSlave.Close(ctx)
	}()

	// Check slave PostgreSQL version
	isPG9Slave, err := pgutil.CheckVersionNewerThanPostgres9(ctx, connSlave)
	if err != nil {
		return fmt.Errorf("error checking slave postgres version: %v", err)
	}

	// Get slave WAL position
	var slave string
	if isPG9Slave {
		walQuery = "SELECT pg_last_xlog_receive_location()"
	} else {
		walQuery = "SELECT pg_last_wal_replay_lsn()"
	}
	err = connSlave.QueryRow(ctx, walQuery).Scan(&slave)
	if err != nil {
		return fmt.Errorf("error querying slave WAL position: %v", err)
	}

	_ = connSlave.Close(ctx)

	// Compute lag
	lag, err := pgutil.ComputeLag(master, slave, segBytes)
	if err != nil {
		return fmt.Errorf("error computing lag: %v", err)
	}

	if lag < 0 {
		lag = -lag
	}

	fmt.Printf("%s %d %d\n", plugin.Scheme, lag, timestamp)

	return nil
}
