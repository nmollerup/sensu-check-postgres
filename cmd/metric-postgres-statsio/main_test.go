package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckArgs(t *testing.T) {
	origPort := plugin.Port
	origIniFile := plugin.IniFile
	origScope := plugin.Scope
	defer func() {
		plugin.Port = origPort
		plugin.IniFile = origIniFile
		plugin.Scope = origScope
	}()

	t.Run("valid defaults", func(t *testing.T) {
		plugin.Port = 5432
		plugin.IniFile = ""
		plugin.Scope = "user"
		if err := checkArgs(nil); err != nil {
			t.Errorf("checkArgs() unexpected error: %v", err)
		}
	})

	t.Run("valid scope all", func(t *testing.T) {
		plugin.Port = 5432
		plugin.IniFile = ""
		plugin.Scope = "all"
		if err := checkArgs(nil); err != nil {
			t.Errorf("checkArgs() unexpected error: %v", err)
		}
	})

	t.Run("invalid scope", func(t *testing.T) {
		plugin.Port = 5432
		plugin.IniFile = ""
		plugin.Scope = "invalid"
		if err := checkArgs(nil); err == nil {
			t.Error("checkArgs() expected error for invalid scope")
		}
	})

	t.Run("empty scope", func(t *testing.T) {
		plugin.Port = 5432
		plugin.IniFile = ""
		plugin.Scope = ""
		if err := checkArgs(nil); err == nil {
			t.Error("checkArgs() expected error for empty scope")
		}
	})

	t.Run("invalid port", func(t *testing.T) {
		plugin.Port = 0
		plugin.IniFile = ""
		plugin.Scope = "user"
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
		plugin.Scope = "user"
		if err := checkArgs(nil); err != nil {
			t.Errorf("checkArgs() unexpected error: %v", err)
		}
	})

	t.Run("nonexistent ini file", func(t *testing.T) {
		plugin.Port = 5432
		plugin.IniFile = "/nonexistent/path/pgpass"
		plugin.Scope = "user"
		if err := checkArgs(nil); err == nil {
			t.Error("checkArgs() expected error for nonexistent ini file")
		}
	})
}
