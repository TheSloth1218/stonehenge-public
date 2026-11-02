package fragments

import (
	"fmt"

	"stonehenge-middleware/internal/config"
)

func NewSource(cfg config.Config) (Source, error) {
	switch cfg.Fragments.Source {
	case "", "internal-rng":
		return NewInternalRandomSource(cfg), nil
	case "ingest":
		return NewExternalIngestSource(cfg.Fragments.Ingest), nil
	default:
		return nil, fmt.Errorf("unsupported fragment source %q", cfg.Fragments.Source)
	}
}
