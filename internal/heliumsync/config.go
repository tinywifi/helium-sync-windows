package heliumsync

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

func DefaultProfile() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(base, "imput", "Helium", "User Data")
}

func DefaultConfig() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "AppData", "Roaming")
	}
	return filepath.Join(base, "helium-sync", "config.toml")
}

func ResolveRepo(cli string, env []string, configPath string) string {
	if cli != "" {
		return abs(expandHome(cli))
	}
	for _, entry := range env {
		if strings.HasPrefix(entry, "HELIUM_SYNC_REPO=") {
			value := strings.TrimPrefix(entry, "HELIUM_SYNC_REPO=")
			if value != "" {
				return abs(expandHome(value))
			}
		}
	}
	if raw, err := os.ReadFile(configPath); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "repo") && strings.Contains(line, "=") {
				value := strings.Trim(strings.TrimSpace(strings.SplitN(line, "=", 2)[1]), "\"'")
				if value != "" {
					return abs(expandHome(value))
				}
			}
		}
	}
	cwd, _ := os.Getwd()
	return cwd
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		home, _ := os.UserHomeDir()
		if path == "~" {
			return home
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func abs(path string) string {
	resolved, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return resolved
}

func nowISO() string {
	return time.Now().Format(time.RFC3339)
}

func nowStamp() string {
	return time.Now().Format("20060102-150405")
}
