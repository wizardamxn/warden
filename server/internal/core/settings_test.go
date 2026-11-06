package core

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeEnv(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		realEnv string
		want    string
	}{
		{name: "plain line", file: "GEMINI_API_KEY=abc\n", want: "abc"},
		{name: "windows line endings", file: "# my key\r\nGEMINI_API_KEY=abc\r\n", want: "abc"},
		{name: "quotes", file: `GEMINI_API_KEY="abc"`, want: "abc"},
		{name: "notepad byte-order mark", file: "\xEF\xBB\xBFGEMINI_API_KEY=abc", want: "abc"},
		{name: "value holds an equals sign", file: "GEMINI_API_KEY=ab=c", want: "ab=c"},
		{name: "real environment wins", file: "GEMINI_API_KEY=abc", realEnv: "real", want: "real"},
		{name: "key missing", file: "OTHER=abc", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GEMINI_API_KEY", tt.realEnv)
			got, err := Load(writeEnv(t, tt.file))
			if err != nil {
				t.Fatal(err)
			}
			if got.GeminiKey != tt.want {
				t.Errorf("GeminiKey = %q, want %q", got.GeminiKey, tt.want)
			}
		})
	}
}

func TestLoadWithoutFile(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "real")
	got, err := Load(filepath.Join(t.TempDir(), "missing.env"))
	if err != nil {
		t.Fatal(err)
	}
	if got.GeminiKey != "real" {
		t.Errorf("GeminiKey = %q, want %q", got.GeminiKey, "real")
	}
}

func TestLoadRejectsBadLine(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	if _, err := Load(writeEnv(t, "not a setting")); err == nil {
		t.Error("expected an error for a line with no =")
	}
}

func TestLoadDefaults(t *testing.T) {
	for _, name := range []string{"DATABASE_URL", "API_ADDR", "WEB_ORIGIN", "LOG_LEVEL"} {
		t.Setenv(name, "")
	}
	got, err := Load(writeEnv(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	if got.APIAddr != "127.0.0.1:8080" || got.WebOrigin != "http://127.0.0.1:3000" || got.LogLevel != slog.LevelInfo {
		t.Errorf("settings = %+v", got)
	}
}

func TestLoadReadsEverySetting(t *testing.T) {
	for _, name := range []string{"DATABASE_URL", "API_ADDR", "LOG_LEVEL"} {
		t.Setenv(name, "")
	}
	t.Setenv("WEB_ORIGIN", "https://warden.example/")
	got, err := Load(writeEnv(t, "DATABASE_URL=postgres://a\nAPI_ADDR=127.0.0.1:9\nLOG_LEVEL=debug\nWEB_ORIGIN=https://file.example\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got.DatabaseURL != "postgres://a" || got.APIAddr != "127.0.0.1:9" || got.LogLevel != slog.LevelDebug || got.WebOrigin != "https://warden.example" {
		t.Errorf("settings = %+v", got)
	}
}

func TestLoadRejectsBadLogLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "loud")
	if _, err := Load(writeEnv(t, "")); err == nil {
		t.Error("expected an error")
	}
}

func TestRequireNamesWhatIsMissingWithoutValues(t *testing.T) {
	tests := []struct {
		name     string
		settings Settings
		want     string
	}{
		{name: "all there", settings: Settings{DatabaseURL: "postgres://user:hunter2@host/db", GeminiKey: "secret"}},
		{name: "no database", settings: Settings{GeminiKey: "secret"}, want: "DATABASE_URL not set"},
		{name: "no key", settings: Settings{DatabaseURL: "postgres://user:hunter2@host/db"}, want: "GEMINI_API_KEY not set"},
		{name: "neither", want: "DATABASE_URL and GEMINI_API_KEY not set"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.settings.Require("DATABASE_URL", "GEMINI_API_KEY")
			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
			if strings.Contains(err.Error(), "hunter2") || strings.Contains(err.Error(), "secret") {
				t.Errorf("the error shows a value: %v", err)
			}
		})
	}
}

func TestOpenDBHidesABadURL(t *testing.T) {
	_, err := OpenDB(context.Background(), "postgres://user:hunter2@host:notaport/db")
	if err == nil || strings.Contains(err.Error(), "hunter2") {
		t.Errorf("err = %v", err)
	}
}

func TestLogWithCarriesFields(t *testing.T) {
	var out strings.Builder
	ctx := context.WithValue(context.Background(), logKey{}, slog.New(slog.NewTextHandler(&out, nil)))
	ctx = LogWith(LogWith(ctx, "video_id", "abc"), "step", "notes")
	Log(ctx).Info("step started")
	if !strings.Contains(out.String(), "video_id=abc step=notes") {
		t.Errorf("log = %q", out.String())
	}
}
