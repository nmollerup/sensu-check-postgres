package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckArgs(t *testing.T) {
	origPort := plugin.Port
	origIniFile := plugin.IniFile
	origQuery := plugin.Query
	defer func() {
		plugin.Port = origPort
		plugin.IniFile = origIniFile
		plugin.Query = origQuery
	}()

	t.Run("valid with query", func(t *testing.T) {
		plugin.Port = 5432
		plugin.IniFile = ""
		plugin.Query = "SELECT 1"
		if err := checkArgs(nil); err != nil {
			t.Errorf("checkArgs() unexpected error: %v", err)
		}
	})

	t.Run("missing query", func(t *testing.T) {
		plugin.Port = 5432
		plugin.IniFile = ""
		plugin.Query = ""
		if err := checkArgs(nil); err == nil {
			t.Error("checkArgs() expected error for missing query")
		}
	})

	t.Run("invalid port", func(t *testing.T) {
		plugin.Port = 0
		plugin.IniFile = ""
		plugin.Query = "SELECT 1"
		if err := checkArgs(nil); err == nil {
			t.Error("checkArgs() expected error for port 0")
		}
	})

	t.Run("invalid port too high", func(t *testing.T) {
		plugin.Port = 65535
		plugin.IniFile = ""
		plugin.Query = "SELECT 1"
		if err := checkArgs(nil); err == nil {
			t.Error("checkArgs() expected error for port 65535")
		}
	})

	t.Run("valid ini file", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "pgpass")
		if err := os.WriteFile(tmpFile, []byte("localhost:5432:*:user:pass"), 0600); err != nil {
			t.Fatal(err)
		}
		plugin.Port = 5432
		plugin.IniFile = tmpFile
		plugin.Query = "SELECT 1"
		if err := checkArgs(nil); err != nil {
			t.Errorf("checkArgs() unexpected error: %v", err)
		}
	})

	t.Run("nonexistent ini file", func(t *testing.T) {
		plugin.Port = 5432
		plugin.IniFile = "/nonexistent/path/pgpass"
		plugin.Query = "SELECT 1"
		if err := checkArgs(nil); err == nil {
			t.Error("checkArgs() expected error for nonexistent ini file")
		}
	})
}
