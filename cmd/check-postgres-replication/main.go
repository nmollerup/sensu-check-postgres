package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"

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
	Warning    int
	Critical   int
	Timeout    int
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-postgres-replication",
			Short:    "postgres replication check",
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
		&sensu.PluginConfigOption[int]{
			Path:      "warning",
			Argument:  "warning",
			Shorthand: "w",
			Usage:     "Warning threshold for replication lag (in MB)",
			Value:     &plugin.Warning,
			Default:   900,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "critical",
			Argument:  "critical",
			Shorthand: "c",
			Usage:     "Critical threshold for replication lag (in MB)",
			Value:     &plugin.Critical,
			Default:   1800,
		},
		&sensu.PluginConfigOption[int]{
			Path:     "timeout",
			Argument: "timeout",
			Usage:    "Connection timeout (seconds)",
			Value:    &plugin.Timeout,
			Default:  2,
		},
	}
)

func main() {
	check := sensu.NewCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(event *corev2.Event) (int, error) {
	if plugin.Port <= 1 || plugin.Port >= 65535 {
		return sensu.CheckStateCritical, fmt.Errorf("invalid port, should be a value between 1 and 65535")
	}
	if plugin.IniFile != "" {
		if _, err := os.Stat(plugin.IniFile); os.IsNotExist(err) {
			return sensu.CheckStateCritical, fmt.Errorf("unable to open the supplied config file %s", plugin.IniFile)
		}
	}
	if plugin.MasterHost == "" {
		return sensu.CheckStateCritical, fmt.Errorf("master-host is required")
	}
	if plugin.SlaveHost == "" {
		return sensu.CheckStateCritical, fmt.Errorf("slave-host is required")
	}
	if plugin.MasterHost == plugin.SlaveHost {
		return sensu.CheckStateCritical, fmt.Errorf("master and slave cannot be the same host")
	}

	return sensu.CheckStateOK, nil
}

func executeCheck(event *corev2.Event) (int, error) {
	ctx := context.Background()

	var dbUser, dbPass, dbDatabase string
	var dbPort int
	if plugin.IniFile != "" {
		iniFile, err := pgpassfile.ReadPassfile(plugin.IniFile)
		if err != nil {
			return sensu.CheckStateCritical, fmt.Errorf("error parsing ini file: %v", err)
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
		return sensu.CheckStateCritical, fmt.Errorf("error connecting to master postgres: %v", err)
	}
	defer func() {
		_ = connMaster.Close(ctx)
	}()

	// Check PostgreSQL version
	isPG9, err := pgutil.CheckVersionNewerThanPostgres9(ctx, connMaster)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error checking postgres version: %v", err)
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
		return sensu.CheckStateCritical, fmt.Errorf("error querying master WAL position: %v", err)
	}

	// Get WAL segment size
	var walSegmentSize string
	err = connMaster.QueryRow(ctx, "SHOW wal_segment_size").Scan(&walSegmentSize)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error querying wal_segment_size: %v", err)
	}

	// Parse wal_segment_size (e.g., "16MB" -> 16777216 bytes)
	re := regexp.MustCompile(`\d+`)
	sizeStr := re.FindString(walSegmentSize)
	if sizeStr == "" {
		return sensu.CheckStateCritical, fmt.Errorf("unable to parse wal_segment_size: %s", walSegmentSize)
	}
	segSize, _ := strconv.ParseInt(sizeStr, 10, 64)
	segBytes := segSize << 20 // Convert MB to bytes

	_ = connMaster.Close(ctx)

	// Connect to slave
	slaveDSN := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s&connect_timeout=%d",
		dbUser, dbPass, plugin.SlaveHost, dbPort, dbDatabase, plugin.Sslmode, plugin.Timeout)
	connSlave, err := pgx.Connect(ctx, slaveDSN)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error connecting to slave postgres: %v", err)
	}
	defer func() {
		_ = connSlave.Close(ctx)
	}()

	// Check slave PostgreSQL version
	isPG9Slave, err := pgutil.CheckVersionNewerThanPostgres9(ctx, connSlave)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error checking slave postgres version: %v", err)
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
		return sensu.CheckStateCritical, fmt.Errorf("error querying slave WAL position: %v", err)
	}

	_ = connSlave.Close(ctx)

	// Calculate lag
	lag, err := pgutil.ComputeLag(master, slave, segBytes)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error computing lag: %v", err)
	}

	lagInMB := float64(abs(lag)) / 1024 / 1024
	message := fmt.Sprintf("replication delayed by %.2fMB :: master:%s slave:%s m_segbytes:%d",
		lagInMB, master, slave, segBytes)

	if lagInMB >= float64(plugin.Critical) {
		return sensu.CheckStateCritical, fmt.Errorf("critical: %s", message)
	} else if lagInMB >= float64(plugin.Warning) {
		return sensu.CheckStateWarning, fmt.Errorf("warning: %s", message)
	}

	fmt.Print(message)
	return sensu.CheckStateOK, nil
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
