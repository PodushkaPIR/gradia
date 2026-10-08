package paths

import (
	"os"
	"path/filepath"
	"runtime"
)

func DB(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	base, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "gradia", "gradia.db"), nil
}

func dataDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		if base := os.Getenv("LOCALAPPDATA"); base != "" && filepath.IsAbs(base) {
			return base, nil
		}
		return os.UserConfigDir()
	case "darwin":
		return os.UserConfigDir()
	default:
		if base := os.Getenv("XDG_DATA_HOME"); base != "" && filepath.IsAbs(base) {
			return base, nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share"), nil
	}
}
