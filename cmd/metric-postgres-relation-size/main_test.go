package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckArgs(t *testing.T) {
	origPort := plugin.Port
	origIniFile := plugin.IniFile
	origLimit := plugin.Limit
	defer func() {
		plugin.Port = origPort
		plugin.IniFile = origIniFile
		plugin.Limit = origLimit
	}()

	t.Run("valid defaults", func(t *testing.T) {
		plugin.Port = 5432
		plugin.IniFile = ""
		plugin.Limit = 25
		if err := checkArgs(nil); err != nil {
			t.Errorf("checkArgs() unexpected error: %v", err)
		}
	})

	t.Run("valid limit 1", func(t *testing.T) {
		plugin.Port = 5432
		plugin.IniFile = ""
		plugin.Limit = 1
		if err := checkArgs(nil); err != nil {
			t.Errorf("checkArgs() unexpected error: %v", err)
		}
	})

	t.Run("invalid limit zero", func(t *testing.T) {
		plugin.Port = 5432
		plugin.IniFile = ""
		plugin.Limit = 0
		if err := checkArgs(nil); err == nil {
			t.Error("checkArgs() expected error for limit 0")
		}
	})

	t.Run("invalid limit negative", func(t *testing.T) {
		plugin.Port = 5432
		plugin.IniFile = ""
		plugin.Limit = -5
		if err := checkArgs(nil); err == nil {
			t.Error("checkArgs() expected error for negative limit")
		}
	})

	t.Run("invalid port", func(t *testing.T) {
		plugin.Port = 0
		plugin.IniFile = ""
		plugin.Limit = 25
		if err := checkArgs(nil); err == nil {
			t.Error("checkArgs() expected error for port 0")
		}
	})

	t.Run("valid ini file", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "pgpass")
		if err := os.WriteFile(tmpFile, []byte("localhost:5432:*:user:pass"), 0600); err != nil {
			t.Fatal(err)
		}
		plugin.Port = 5432
		plugin.IniFile = tmpFile
		plugin.Limit = 25
		if err := checkArgs(nil); err != nil {
			t.Errorf("checkArgs() unexpected error: %v", err)
		}
	})

	t.Run("nonexistent ini file", func(t *testing.T) {
		plugin.Port = 5432
		plugin.IniFile = "/nonexistent/path/pgpass"
		plugin.Limit = 25
		if err := checkArgs(nil); err == nil {
			t.Error("checkArgs() expected error for nonexistent ini file")
		}
	})
}
