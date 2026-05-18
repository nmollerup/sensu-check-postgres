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
	User         string
	Password     string
	IniFile      string
	Hostname     string
	Port         int
	Database     string
	Sslmode      string
	Scheme       string
	Timeout      int
	AllDatabases bool
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "metric-postgres-statsdb",
			Short:    "postgres database stats metric",
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
		&sensu.PluginConfigOption[bool]{
			Path:      "all-databases",
			Argument:  "all-databases",
			Shorthand: "a",
			Usage:     "Get stats for all available databases",
			Value:     &plugin.AllDatabases,
			Default:   false,
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

	query := "SELECT * FROM pg_stat_database"
	var rows pgx.Rows
	if plugin.AllDatabases {
		rows, err = db.Query(ctx, query)
	} else {
		query += " WHERE datname = $1"
		rows, err = db.Query(ctx, query, dbDatabase)
	}
	if err != nil {
		return fmt.Errorf("error querying database stats: %v", err)
	}
	defer rows.Close()

	skipColumns := map[string]bool{
		"datid":       true,
		"datname":     true,
		"stats_reset": true,
	}

	fieldDescs := rows.FieldDescriptions()

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return fmt.Errorf("error scanning row: %v", err)
		}

		// Find datname column index
		var database string
		for i, fd := range fieldDescs {
			if string(fd.Name) == "datname" {
				if values[i] != nil {
					database = fmt.Sprintf("%v", values[i])
				}
				break
			}
		}

		if database == "" {
			continue
		}

		for i, fd := range fieldDescs {
			colName := string(fd.Name)
			if skipColumns[colName] {
				continue
			}
			if values[i] != nil {
				fmt.Printf("%s.statsdb.%s.%s %v %d\n", plugin.Scheme, database, colName, values[i], timestamp)
			}
		}
	}

	return nil
}
