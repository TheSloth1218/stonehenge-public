package history

import (
	"fmt"
	"log/slog"

	"stonehenge-middleware/internal/config"
)

func NewSink(cfg config.HistoryConfig, logger ...*slog.Logger) (Sink, error) {
	switch cfg.Adapter {
	case "", "memory":
		return NewInMemorySink(cfg.MaxEvents), nil
	case "noop":
		return NewNoOpSink(), nil
	case "postgres":
		var l *slog.Logger
		if len(logger) > 0 {
			l = logger[0]
		}
		return NewPostgresHistorySink(cfg, l)
	default:
		return nil, fmt.Errorf("unsupported history adapter %q", cfg.Adapter)
	}
}
