// Package config decides where Ldapper keeps its files. Each operating system
// has its own answer, and getting it wrong means a user's saved filters
// disappear on upgrade.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// appName is the directory Ldapper creates inside the platform's own
// configuration location.
const appName = "Ldapper"

// dirEnv overrides the location entirely. Tests use it, and so does anyone
// running Ldapper from a memory stick.
const dirEnv = "LDAPPER_CONFIG_DIR"

// Dir is where Ldapper's files live:
//
//	Windows  %AppData%\Ldapper
//	macOS    ~/Library/Application Support/Ldapper
//	Linux    $XDG_CONFIG_HOME/Ldapper, or ~/.config/Ldapper
func Dir() (string, error) {
	if override := os.Getenv(dirEnv); override != "" {
		return override, nil
	}

	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config: cannot find this system's configuration directory: %w", err)
	}
	return filepath.Join(base, appName), nil
}

// FiltersPath is where the filter overlay is stored.
func FiltersPath() (string, error) { return pathTo("filters.json") }

// ConnectionsPath is where saved connections are stored.
func ConnectionsPath() (string, error) { return pathTo("connections.json") }

func pathTo(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}
