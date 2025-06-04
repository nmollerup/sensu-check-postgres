package main

import (
	"context"
	"fmt"
	"os"

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
	Port     uint
	Database string
	Sslmode  string
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-postgres-alive",
			Short:    "postgres alive check",
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
		&sensu.PluginConfigOption[uint]{
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

func Check(f func() error) {
	if err := f(); err != nil {
		fmt.Println("Database closure failed:", err)
	}
}

func executeCheck(event *corev2.Event) (int, error) {
	var dataSourceName string

	var dbUser, dbPass string
	if plugin.IniFile != "" {
		//iniFile, err := ini.Load(plugin.IniFile)
		//if err != nil {
		//	return sensu.CheckStateCritical, fmt.Errorf("error parsing ini file: %v", err)
		//}
		// dbUser = iniFile.Section(plugin.IniSection).Key("user").String()
		//dbPass = iniFile.Section(plugin.IniSection).Key("password").String()
	} else {
		dbUser = plugin.User
		dbPass = plugin.Password
	}

	dataSourceName = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", dbUser, dbPass, plugin.Hostname, plugin.Port, plugin.Database, plugin.Sslmode)
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

	fmt.Printf("postgres server is alive.")
	return sensu.CheckStateOK, nil
}
