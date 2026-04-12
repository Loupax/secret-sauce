package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// DefaultVaultDir returns the platform-specific default vault directory path using
// the precedence: SAUCE_DIR → SECRET_SAUCE_DIR → platform data dir/secret-sauce.
// On Windows the platform default is %APPDATA%\secret-sauce.
// On Linux/macOS it follows XDG: $XDG_DATA_HOME/secret-sauce or ~/.local/share/secret-sauce.
func DefaultVaultDir() (string, error) {
	if v := os.Getenv("SAUCE_DIR"); v != "" {
		return v, nil
	}
	if v := os.Getenv("SECRET_SAUCE_DIR"); v != "" {
		return v, nil
	}
	base, err := PlatformDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "secret-sauce"), nil
}

// PlatformDataDir determines the base user data directory for the current OS.
func PlatformDataDir() (string, error) {
	if runtime.GOOS == "windows" {
		if v := os.Getenv("APPDATA"); v != "" {
			return v, nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("%%APPDATA%% is not set and home directory could not be determined: %w", err)
		}
		return filepath.Join(home, "AppData", "Roaming"), nil
	}
	
	// Linux / macOS: respect XDG_DATA_HOME, fall back to ~/.local/share
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return xdg, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %w", err)
	}
	return filepath.Join(home, ".local", "share"), nil
}
