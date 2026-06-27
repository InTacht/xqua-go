package service

import (
	"fmt"
	"time"

	"github.com/InTacht/xqua-go/pkg/logger"
)

// Config configures a service instance.
type Config struct {
	Name string
	ID   string

	ShutdownTimeout time.Duration // default 30 seconds
	Debug           bool          // default false

	Version   *string // optional
	BuildID   *string // optional
	BuildTime *string // optional

	Logger *logger.Logger // optional. When nil, a logger is created from Name and ID.
}

func (cfg Config) validate() error {
	// validate required fields
	if cfg.Name == "" {
		return fmt.Errorf("service: Name is required")
	}
	if cfg.ID == "" {
		return fmt.Errorf("service: ID is required")
	}

	// setup defaults
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 30 * time.Second
	}

	return nil
}

func (cfg Config) logger() (*logger.Logger, bool) {
	if cfg.Logger != nil {
		return cfg.Logger, false
	}

	return logger.New(&logger.Config{
		Name:  cfg.Name,
		ID:    cfg.ID,
		Label: "service",
		Debug: cfg.Debug,
	}), true
}
