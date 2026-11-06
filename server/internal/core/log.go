package core

import (
	"context"
	"log/slog"
	"os"
)

type logKey struct{}

// SetupLog makes readable text on stderr the log for this program. Stderr keeps
// stdout free for what a one-off run prints.
func SetupLog(level slog.Level) {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && len(groups) == 0 {
				return slog.String(a.Key, a.Value.Time().Format("15:04:05.000"))
			}
			return a
		},
	})))
}

// Log returns the logger for this piece of work, carrying its video_id, job_id,
// step or request_id, so the pipeline never has to pass them along by hand.
func Log(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(logKey{}).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

// LogWith adds fields to every line logged with the returned context.
func LogWith(ctx context.Context, args ...any) context.Context {
	return context.WithValue(ctx, logKey{}, Log(ctx).With(args...))
}
