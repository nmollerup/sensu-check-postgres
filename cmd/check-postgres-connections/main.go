package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/dariubs/percent"
	"github.com/jackc/pgpassfile"
	"github.com/jackc/pgx/v5"
	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

type Config struct {
	sensu.PluginConfig
	User       string
	Password   string
	IniFile    string
	Hostname   string
	Port       int
	Database   string
	Sslmode    string
	Warning    int
	Critical   int
	Percentage bool
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-postgres-connections",
			Short:    "postgres connections check",
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
			Path:     "port",
			Argument: "port",
			Usage:    "Port to connect to",
			Value:    &plugin.Port,
			Default:  5432,
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
		&sensu.PluginConfigOption[int]{
			Path:      "warning",
			Argument:  "warning",
			Shorthand: "w",
			Usage:     "Warning threshold number or % of connections. (default: 200 connections)",
			Value:     &plugin.Warning,
			Default:   200,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "critical",
			Argument:  "critical",
			Shorthand: "c",
			Usage:     "Critical threshold number or % of connections. (default: 250 connections)",
			Default:   250,
			Value:     &plugin.Critical,
		},
		&sensu.PluginConfigOption[bool]{
			Path:     "percentage",
			Argument: "percentage",
			Usage:    "Use percentage of defined max connections instead of absolute value",
			Value:    &plugin.Percentage,
			Default:  false,
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

	return sensu.CheckStateOK, nil
}
func executeCheck(event *corev2.Event) (int, error) {
	var current_connections int
	var max_connections string
	var superuser_reserved_connections string
	var dataSourceName string

	var dbUser, dbPass, dbHost, dbDatabase string
	var dbPort int
	if plugin.IniFile != "" {
		iniFile, err := pgpassfile.ReadPassfile(plugin.IniFile)
		if err != nil {
			return sensu.CheckStateCritical, fmt.Errorf("error parsing ini file: %v", err)
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

	dataSourceName = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", dbUser, dbPass, dbHost, dbPort, dbDatabase, plugin.Sslmode)
	db, err := pgx.Connect(context.Background(), dataSourceName)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error connecting to postgres: %v", err)
	}
	defer func() {
		_ = db.Close(context.Background())
	}()

	err = db.Ping(context.Background())
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error pinging postgres: %v", err)
	}
	err = db.QueryRow(context.Background(), "SHOW max_connections").Scan(&max_connections)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error querying postgres connections: %v", err)
	}
	err = db.QueryRow(context.Background(), "SHOW superuser_reserved_connections").Scan(&superuser_reserved_connections)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error querying postgres superuser reserved connections: %v", err)
	}
	err = db.QueryRow(context.Background(), "SELECT count(*) from pg_stat_activity").Scan(&current_connections)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error querying postgres current connections: %v", err)
	}
	max_connections_i, _ := strconv.Atoi(max_connections)
	superuser_reserved_connections_i, _ := strconv.Atoi(superuser_reserved_connections)
	available_connections := max_connections_i - superuser_reserved_connections_i

	if plugin.Percentage {
		percentage := percent.PercentOf(current_connections, max_connections_i)
		if percentage >= float64(plugin.Critical) {
			return sensu.CheckStateCritical, fmt.Errorf("critical: postgres connections at %.2f%% out of %d connections", percentage, max_connections_i)
		}
		if percentage >= float64(plugin.Warning) {
			return sensu.CheckStateWarning, fmt.Errorf("warning: postgres connections at %.2f%% out of %d connections", percentage, max_connections_i)
		}
		fmt.Printf("postgres connections at %.2f%% out of %d connections.", percentage, available_connections)

	} else if !plugin.Percentage {
		if current_connections >= plugin.Critical {
			return sensu.CheckStateCritical, fmt.Errorf("critical: postgres connections at %d out of %d connections", current_connections, available_connections)
		}
		if current_connections >= plugin.Warning {
			return sensu.CheckStateWarning, fmt.Errorf("warning: postgres connections at %d out of %d connections", current_connections, available_connections)
		}
		fmt.Printf("postgres connections at %d out of %d connections.", current_connections, available_connections)
	}

	return sensu.CheckStateOK, nil
}
