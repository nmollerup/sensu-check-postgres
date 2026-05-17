package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

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
	Query        string
	RegexPattern string
	CheckTuples  bool
	Warning      string
	Critical     string
	Timeout      int
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-postgres-query",
			Short:    "postgres query check",
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
			Path:      "query",
			Argument:  "query",
			Shorthand: "q",
			Usage:     "Database query to execute",
			Value:     &plugin.Query,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "regex-pattern",
			Argument:  "regex-pattern",
			Shorthand: "r",
			Usage:     "Regex pattern to match on query results and alert on if it does not match",
			Value:     &plugin.RegexPattern,
		},
		&sensu.PluginConfigOption[bool]{
			Path:      "check-tuples",
			Argument:  "tuples",
			Shorthand: "t",
			Usage:     "Check against the number of tuples (rows) returned by the query",
			Value:     &plugin.CheckTuples,
			Default:   false,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "warning",
			Argument:  "warning",
			Shorthand: "w",
			Usage:     "Warning threshold expression (e.g. 'value > 5')",
			Value:     &plugin.Warning,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "critical",
			Argument:  "critical",
			Shorthand: "c",
			Usage:     "Critical threshold expression (e.g. 'value > 10')",
			Value:     &plugin.Critical,
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
	if plugin.Query == "" {
		return sensu.CheckStateCritical, fmt.Errorf("query is required")
	}

	return sensu.CheckStateOK, nil
}

func executeCheck(event *corev2.Event) (int, error) {
	ctx := context.Background()

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

	dataSourceName := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s&connect_timeout=%d",
		dbUser, dbPass, dbHost, dbPort, dbDatabase, plugin.Sslmode, plugin.Timeout)
	db, err := pgx.Connect(ctx, dataSourceName)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error connecting to postgres: %v", err)
	}
	defer func() {
		_ = db.Close(ctx)
	}()

	err = db.Ping(ctx)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error pinging postgres: %v", err)
	}

	// Execute query
	rows, err := db.Query(ctx, plugin.Query)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("unable to query PostgreSQL: %v", err)
	}
	defer rows.Close()

	// Collect all rows
	var results [][]interface{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return sensu.CheckStateCritical, fmt.Errorf("error reading query results: %v", err)
		}
		results = append(results, values)
	}

	if rows.Err() != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error iterating query results: %v", rows.Err())
	}

	// Determine the value to check
	var value float64
	if plugin.CheckTuples {
		value = float64(len(results))
	} else {
		if len(results) == 0 || len(results[0]) == 0 {
			return sensu.CheckStateCritical, fmt.Errorf("query returned no results")
		}
		// Convert first value to float
		firstValue := results[0][0]
		switch v := firstValue.(type) {
		case int64:
			value = float64(v)
		case float64:
			value = v
		case string:
			value, err = strconv.ParseFloat(v, 64)
			if err != nil {
				return sensu.CheckStateCritical, fmt.Errorf("unable to convert result to number: %v", err)
			}
		default:
			return sensu.CheckStateCritical, fmt.Errorf("unable to convert result to number: unsupported type %T", v)
		}
	}

	// Check critical threshold
	if plugin.Critical != "" {
		if matched, err := evaluateExpression(plugin.Critical, value); err != nil {
			return sensu.CheckStateCritical, fmt.Errorf("error evaluating critical threshold: %v", err)
		} else if matched {
			return sensu.CheckStateCritical, fmt.Errorf("critical: Results: %v", results)
		}
	}

	// Check warning threshold
	if plugin.Warning != "" {
		if matched, err := evaluateExpression(plugin.Warning, value); err != nil {
			return sensu.CheckStateCritical, fmt.Errorf("error evaluating warning threshold: %v", err)
		} else if matched {
			return sensu.CheckStateWarning, fmt.Errorf("warning: Results: %v", results)
		}
	}

	// Check regex pattern
	if plugin.RegexPattern != "" {
		if len(results) == 0 || len(results[0]) == 0 {
			return sensu.CheckStateCritical, fmt.Errorf("query result is empty, cannot match regex")
		}
		resultStr := fmt.Sprintf("%v", results[0][0])
		matched, err := regexp.MatchString(plugin.RegexPattern, resultStr)
		if err != nil {
			return sensu.CheckStateCritical, fmt.Errorf("error evaluating regex pattern: %v", err)
		}
		if !matched {
			return sensu.CheckStateCritical, fmt.Errorf("query result %s doesn't match configured regex %s", resultStr, plugin.RegexPattern)
		}
	}

	fmt.Print("Query OK")
	return sensu.CheckStateOK, nil
}

// evaluateExpression evaluates simple threshold expressions like "value > 5", "value <= 10"
// Supports operators: >, <, >=, <=, ==, !=
func evaluateExpression(expr string, value float64) (bool, error) {
	expr = strings.TrimSpace(expr)

	// Parse expression: "value <op> <threshold>"
	operators := []string{">=", "<=", "==", "!=", ">", "<"}
	var operator string
	var thresholdStr string

	for _, op := range operators {
		if strings.Contains(expr, op) {
			parts := strings.SplitN(expr, op, 2)
			if len(parts) != 2 {
				continue
			}
			// Check if "value" is on the left side
			if strings.TrimSpace(parts[0]) == "value" {
				operator = op
				thresholdStr = strings.TrimSpace(parts[1])
				break
			}
		}
	}

	if operator == "" {
		return false, fmt.Errorf("invalid expression format: %s (expected 'value <op> <number>')", expr)
	}

	threshold, err := strconv.ParseFloat(thresholdStr, 64)
	if err != nil {
		return false, fmt.Errorf("invalid threshold value: %s", thresholdStr)
	}

	// Evaluate the expression
	switch operator {
	case ">":
		return value > threshold, nil
	case "<":
		return value < threshold, nil
	case ">=":
		return value >= threshold, nil
	case "<=":
		return value <= threshold, nil
	case "==":
		return value == threshold, nil
	case "!=":
		return value != threshold, nil
	default:
		return false, fmt.Errorf("unsupported operator: %s", operator)
	}
}
