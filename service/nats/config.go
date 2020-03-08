package nats

import (
	"errors"

	"github.com/spiral/roadrunner/service"
)

// Config defines NATS service config.
type Config struct {
	// Indicates if NATS connection is enabled.
	Enable bool

	// Comma separated list of cluster servers
	URLs string

	// Queue group name
	Group string

	// Subject to listen to
	Subject string
}

// Hydrate must populate Config values using given Config source. Must return error if Config is not valid.
func (c *Config) Hydrate(cfg service.Config) error {
	if err := cfg.Unmarshal(c); err != nil {
		return err
	}

	return c.Valid()
}

// InitDefaults allows to init blank config with pre-defined set of default values.
func (c *Config) InitDefaults() error {
	c.Enable = false
	c.URLs = "nats://localhost:4222"
	c.Group = "default"

	return nil
}

// Valid returns nil if config is valid.
func (c *Config) Valid() error {
	if c.Subject == "" {
		return errors.New("NATS Subject is required")
	}

	return nil
}
