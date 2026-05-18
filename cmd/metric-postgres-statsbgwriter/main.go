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
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "metric-postgres-statsbgwriter",
			Short:    "postgres bgwriter metric",
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
			Usage:     "Database schema to connect to",
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

	query := `SELECT checkpoints_timed, checkpoints_req,
		checkpoint_write_time, checkpoint_sync_time,
		buffers_checkpoint, buffers_clean,
		maxwritten_clean, buffers_backend, buffers_backend_fsync,
		buffers_alloc
		FROM pg_stat_bgwriter`

	var checkpointsTimed, checkpointsReq int64
	var checkpointWriteTime, checkpointSyncTime float64
	var buffersCheckpoint, buffersClean, maxwrittenClean int64
	var buffersBackend, buffersBackendFsync, buffersAlloc int64

	err = db.QueryRow(ctx, query).Scan(
		&checkpointsTimed, &checkpointsReq,
		&checkpointWriteTime, &checkpointSyncTime,
		&buffersCheckpoint, &buffersClean,
		&maxwrittenClean, &buffersBackend, &buffersBackendFsync,
		&buffersAlloc,
	)
	if err != nil {
		return fmt.Errorf("error querying bgwriter stats: %v", err)
	}

	fmt.Printf("%s.bgwriter.checkpoints_timed %d %d\n", plugin.Scheme, checkpointsTimed, timestamp)
	fmt.Printf("%s.bgwriter.checkpoints_req %d %d\n", plugin.Scheme, checkpointsReq, timestamp)
	fmt.Printf("%s.bgwriter.checkpoints_write_time %g %d\n", plugin.Scheme, checkpointWriteTime, timestamp)
	fmt.Printf("%s.bgwriter.checkpoints_sync_time %g %d\n", plugin.Scheme, checkpointSyncTime, timestamp)
	fmt.Printf("%s.bgwriter.buffers_checkpoint %d %d\n", plugin.Scheme, buffersCheckpoint, timestamp)
	fmt.Printf("%s.bgwriter.buffers_clean %d %d\n", plugin.Scheme, buffersClean, timestamp)
	fmt.Printf("%s.bgwriter.maxwritten_clean %d %d\n", plugin.Scheme, maxwrittenClean, timestamp)
	fmt.Printf("%s.bgwriter.buffers_backend %d %d\n", plugin.Scheme, buffersBackend, timestamp)
	fmt.Printf("%s.bgwriter.buffers_backend_fsync %d %d\n", plugin.Scheme, buffersBackendFsync, timestamp)
	fmt.Printf("%s.bgwriter.buffers_alloc %d %d\n", plugin.Scheme, buffersAlloc, timestamp)

	return nil
}
