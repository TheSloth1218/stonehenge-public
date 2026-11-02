package plc

import (
	"fmt"
	"log/slog"

	"stonehenge-middleware/internal/config"
)

func NewAdapter(cfg config.PLCConfig, logger *slog.Logger) (Adapter, error) {
	switch cfg.Adapter {
	case "", "simulated":
		return NewSimulatedAdapter(), nil
	default:
		return nil, fmt.Errorf("unsupported plc adapter %q", cfg.Adapter)
	}
}
