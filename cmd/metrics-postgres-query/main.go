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
	User        string
	Password    string
	IniFile     string
	Hostname    string
	Port        int
	Database    string
	Sslmode     string
	Scheme      string
	Timeout     int
	Query       string
	CountTuples bool
	Multirow    bool
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "metrics-postgres-query",
			Short:    "postgres query metric",
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
			Usage:    "Metric naming scheme, text to prepend to metric",
			Value:    &plugin.Scheme,
			Default:  "postgres",
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
			Path:      "query",
			Argument:  "query",
			Shorthand: "q",
			Usage:     "Database query to execute",
			Value:     &plugin.Query,
		},
		&sensu.PluginConfigOption[bool]{
			Path:      "count-tuples",
			Argument:  "tuples",
			Shorthand: "t",
			Usage:     "Count the number of tuples (rows) returned by the query",
			Value:     &plugin.CountTuples,
			Default:   false,
		},
		&sensu.PluginConfigOption[bool]{
			Path:      "multirow",
			Argument:  "multirow",
			Shorthand: "m",
			Usage:     "Return all rows instead of just the first",
			Value:     &plugin.Multirow,
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
	if plugin.Query == "" {
		return fmt.Errorf("query is required")
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

	rows, err := db.Query(ctx, plugin.Query)
	if err != nil {
		return fmt.Errorf("unable to query PostgreSQL: %v", err)
	}
	defer rows.Close()

	var results [][]interface{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return fmt.Errorf("error reading query results: %v", err)
		}
		results = append(results, values)
	}
	if rows.Err() != nil {
		return fmt.Errorf("error iterating query results: %v", rows.Err())
	}

	if plugin.CountTuples {
		fmt.Printf("%s %d %d\n", plugin.Scheme, len(results), timestamp)
		return nil
	}

	if plugin.Multirow {
		for _, row := range results {
			if len(row) >= 2 {
				fmt.Printf("%s.%v %v %d\n", plugin.Scheme, row[0], row[1], timestamp)
			}
		}
		return nil
	}

	// Default: output first row, first value
	if len(results) == 0 || len(results[0]) == 0 {
		return fmt.Errorf("query returned no results")
	}
	fmt.Printf("%s %v %d\n", plugin.Scheme, results[0][0], timestamp)

	return nil
}
