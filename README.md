[![Sensu Bonsai Asset](https://img.shields.io/badge/Bonsai-Download%20Me-brightgreen.svg?colorB=89C967&logo=sensu)](https://bonsai.sensu.io/assets/nmollerup/sensu-check-postgres)
[![goreleaser](https://github.com/nmollerup/sensu-check-postgres/actions/workflows/release.yml/badge.svg)](https://github.com/nmollerup/sensu-check-postgres/actions/workflows/release.yml) [![Go Test](https://github.com/nmollerup/sensu-check-postgres/actions/workflows/test.yml/badge.svg)](https://github.com/nmollerup/sensu-check-postgres/actions/workflows/test.yml) 
# sensu-check-postgres

## Table of Contents

## Usage

### Help Text Output

```
postgres alive check

Usage:
  check-postgres-alive [flags]
  check-postgres-alive [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  version     Print the version number of this plugin

Flags:
  -d, --database string   Database schema to connect to (default "postgres")
  -h, --help              help for check-postgres-alive
      --hostname string   Hostname to login to (default "localhost")
  -p, --password string   Password for user
  -f, --pgpass string     Location of .pgpass file for access to postgres
      --port uint         Port to connect to (default 5432)
  -s, --sslmode string    SSL mode for connecting to postgres (default "prefer")
  -u, --user string       postgres user to connect

Use "check-postgres-alive [command] --help" for more information about a command.
```

### Configuration

The check connects to the supplied postgres database and returns OK if it works. Ini file overrides commandline arguments for user/pass.
