[![Sensu Bonsai Asset](https://img.shields.io/badge/Bonsai-Download%20Me-brightgreen.svg?colorB=89C967&logo=sensu)](https://bonsai.sensu.io/assets/nmollerup/sensu-check-postgres)
[![goreleaser](https://github.com/nmollerup/sensu-check-postgres/actions/workflows/release.yml/badge.svg)](https://github.com/nmollerup/sensu-check-postgres/actions/workflows/release.yml) [![Go Test](https://github.com/nmollerup/sensu-check-postgres/actions/workflows/test.yml/badge.svg)](https://github.com/nmollerup/sensu-check-postgres/actions/workflows/test.yml)

# sensu-check-postgres

A Go reimplementation of [sensu-plugins-postgres](https://github.com/sensu-plugins/sensu-plugins-postgres) providing native PostgreSQL checks and metrics for [Sensu Go](https://sensu.io).

## Plugins

### Checks

- **check-postgres-alive** — Verify database connectivity
- **check-postgres-connections** — Alert on connection count thresholds
- **check-postgres-query** — Run arbitrary SQL with threshold or regex matching
- **check-postgres-replication** — Monitor master/slave replication lag

### Metrics

- **metric-postgres-connections** — Active, waiting, and total connections
- **metric-postgres-dbsize** — Database size in bytes
- **metric-postgres-locks** — Lock counts grouped by mode
- **metric-postgres-statsbgwriter** — Background writer checkpoint and buffer stats
- **metric-postgres-statsdb** — Per-database stats from `pg_stat_database`
- **metric-postgres-statsio** — I/O stats from `pg_statio_*_tables`
- **metric-postgres-statstable** — Table stats from `pg_stat_*_tables`
- **metric-postgres-graphite** — Replication lag as a Graphite metric
- **metric-postgres-relation-size** — Top N largest relations by total size
- **metrics-postgres-query** — Arbitrary SQL query output as metrics

All metrics are output in Graphite plaintext format: `<scheme>.<category>.<name> <value> <timestamp>`

## Usage

Every plugin supports `--help` for full option details. Common flags:

```
-u, --user        Postgres user
-p, --password    Password
-f, --pgpass      Path to .pgpass file
    --hostname    Host to connect to (default: localhost)
-P, --port        Port (default: 5432)
-d, --database    Database name (default: postgres)
-s, --sslmode     SSL mode (default: prefer)
-T, --timeout     Connection timeout in seconds (default: 10)
    --scheme      Metric naming prefix (default: postgresql)
```

### Examples

```bash
# Check if PostgreSQL is alive
check-postgres-alive -u sensu -p secret -d mydb

# Alert when connections exceed 80% (warning) or 90% (critical)
check-postgres-connections -u sensu -p secret -d mydb -w 80 -c 90

# Collect bgwriter metrics
metric-postgres-statsbgwriter -u sensu -p secret -d mydb

# Collect per-database stats for all databases
metric-postgres-statsdb -u sensu -p secret -a

# Run a custom query and output metrics
metrics-postgres-query -u sensu -p secret -d mydb -q "SELECT count(*) FROM orders"
```

### Pgpass file

All plugins support `.pgpass` files via `-f` / `--pgpass`. Format:

```
hostname:port:database:username:password
```

When specified, pgpass values override command-line connection arguments.

## Installation

Download the latest release from the [releases page](https://github.com/nmollerup/sensu-check-postgres/releases) or register as a Sensu Bonsai asset.

## Configuration

Plugins connect to the supplied PostgreSQL database using the provided credentials. Checks return standard Sensu exit codes (0=OK, 1=WARNING, 2=CRITICAL). Metric plugins output Graphite-formatted data to stdout for Sensu metric collection.
