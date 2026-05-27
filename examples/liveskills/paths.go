package main

import (
	"os"
	"path/filepath"
	"strings"
)

func envValue(env map[string]string, key string) string {
	if env != nil {
		return env[key]
	}
	return os.Getenv(key)
}

func defaultHome(env map[string]string) string {
	if value := envValue(env, "LIVESKILLS_HOME"); value != "" {
		return value
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".liveskills"
	}
	return filepath.Join(home, ".liveskills")
}

func expandHome(input string) string {
	if input == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
	}
	if strings.HasPrefix(input, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, strings.TrimPrefix(input, "~/"))
		}
	}
	return input
}

func resolvePath(cwd, input string) string {
	expanded := expandHome(input)
	if filepath.IsAbs(expanded) {
		return filepath.Clean(expanded)
	}
	return filepath.Clean(filepath.Join(cwd, expanded))
}

func homeRelative(input string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return input
	}
	clean := filepath.Clean(input)
	if clean == home {
		return "~"
	}
	if strings.HasPrefix(clean, home+string(os.PathSeparator)) {
		rel, err := filepath.Rel(home, clean)
		if err == nil {
			return "~/" + filepath.ToSlash(rel)
		}
	}
	return input
}
