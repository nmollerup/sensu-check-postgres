package pgutil

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// CheckVersionNewerThanPostgres9 returns true if the PostgreSQL version is 9.x
// (between 9.0 and less than 10.0)
func CheckVersionNewerThanPostgres9(ctx context.Context, conn *pgx.Conn) (bool, error) {
	var pgVersion string
	err := conn.QueryRow(ctx, "SELECT current_setting('server_version')").Scan(&pgVersion)
	if err != nil {
		return false, fmt.Errorf("error querying server version: %v", err)
	}

	// Extract version number (e.g., "9.6.24" or "14.5")
	versionParts := strings.Split(pgVersion, " ")
	if len(versionParts) == 0 {
		return false, fmt.Errorf("unable to parse version string: %s", pgVersion)
	}

	version := versionParts[0]
	majorMinor := strings.Split(version, ".")
	if len(majorMinor) < 2 {
		return false, fmt.Errorf("unable to parse version number: %s", version)
	}

	major, err := strconv.Atoi(majorMinor[0])
	if err != nil {
		return false, fmt.Errorf("unable to parse major version: %v", err)
	}

	// PostgreSQL 9.x versions
	if major == 9 {
		return true, nil
	}

	// PostgreSQL 10+ versions
	return false, nil
}

// ComputeLag calculates the replication lag in bytes between master and slave
// WAL positions. Returns the lag in bytes.
func ComputeLag(master, slave string, segBytes int64) (int64, error) {
	// WAL position format: segment/offset (e.g., "B0/B4031000")
	masterParts := strings.Split(master, "/")
	slaveParts := strings.Split(slave, "/")

	if len(masterParts) != 2 || len(slaveParts) != 2 {
		return 0, fmt.Errorf("invalid WAL position format")
	}

	// Parse hexadecimal values
	mSegment, err := strconv.ParseInt(masterParts[0], 16, 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing master segment: %v", err)
	}

	mOffset, err := strconv.ParseInt(masterParts[1], 16, 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing master offset: %v", err)
	}

	sSegment, err := strconv.ParseInt(slaveParts[0], 16, 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing slave segment: %v", err)
	}

	sOffset, err := strconv.ParseInt(slaveParts[1], 16, 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing slave offset: %v", err)
	}

	// Calculate lag: (segment_diff * segment_size) + offset_diff
	lag := ((mSegment - sSegment) * segBytes) + (mOffset - sOffset)
	return lag, nil
}
