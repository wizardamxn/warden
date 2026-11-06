// Package core holds what the whole server shares: settings, logging and the
// database connection.
package core

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"
)

// LocalWebOrigin is where the website runs on this machine.
const LocalWebOrigin = "http://127.0.0.1:3000"

// Settings are the values the server needs to run.
type Settings struct {
	GeminiKey   string
	DatabaseURL string
	APIAddr     string
	WebOrigin   string
	LogLevel    slog.Level
}

// Load reads settings from the env file, if there is one.
// A real environment variable wins over the file.
func Load(envFile string) (Settings, error) {
	file, err := readEnvFile(envFile)
	if err != nil {
		return Settings{}, err
	}
	value := func(name, fallback string) string {
		if v := os.Getenv(name); v != "" {
			return v
		}
		if v := file[name]; v != "" {
			return v
		}
		return fallback
	}

	settings := Settings{
		GeminiKey:   value("GEMINI_API_KEY", ""),
		DatabaseURL: value("DATABASE_URL", ""),
		APIAddr:     value("API_ADDR", "127.0.0.1:8080"),
		WebOrigin:   strings.TrimRight(value("WEB_ORIGIN", LocalWebOrigin), "/"),
	}
	if err := settings.LogLevel.UnmarshalText([]byte(value("LOG_LEVEL", "info"))); err != nil {
		return Settings{}, errors.New("LOG_LEVEL must be debug, info, warn or error")
	}
	return settings, nil
}

// Require stops a program from starting without what it needs. It names the
// missing setting and never prints a value.
func (s Settings) Require(names ...string) error {
	values := map[string]string{"GEMINI_API_KEY": s.GeminiKey, "DATABASE_URL": s.DatabaseURL}
	var missing []string
	for _, name := range names {
		if values[name] == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s not set, add to server/.env", strings.Join(missing, " and "))
	}
	return nil
}

// readEnvFile understands KEY=value lines and # comments.
func readEnvFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	values := map[string]string{}
	// Windows Notepad can start a file with an invisible byte-order mark.
	text := strings.TrimPrefix(string(data), "\xEF\xBB\xBF")
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			// Only the line number: the line itself could hold a secret.
			return nil, fmt.Errorf("%s line %d: expected KEY=value", path, i+1)
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return values, nil
}
