package nats

import (
	"github.com/nats-io/nats.go"
	"github.com/spiral/roadrunner/service"
	"github.com/spiral/roadrunner/service/env"
	"log"
	"sync"
)

// ID contains default service name.
const ID = "nats"

// Service is NATS client.
type Service struct {
	cfg     *Config
	channel	chan *nats.Msg
	mu      sync.Mutex
	serving bool
}

// Init rpc service. Must return true if service is enabled.
func (s *Service) Init(cfg *Config, c service.Container, env env.Environment) (bool, error) {
	if !cfg.Enable {
		return false, nil
	}
	s.cfg = cfg
	return true, nil
}

// Serve serves the service.
func (s *Service) Serve() error {
	s.mu.Lock()
	s.serving = true
	s.channel = make(chan *nats.Msg, 64)
	s.mu.Unlock()

	nc, err := nats.Connect(s.cfg.URLs)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	sub, err := nc.ChanQueueSubscribe(s.cfg.Subject, s.cfg.Group, s.channel)

	for msg := range s.channel {
		log.Printf("[%s] %s", msg.Subject, msg.Data)
	}

	// Unsubscribe
	sub.Unsubscribe()

	// Drain
	sub.Drain()

	s.mu.Lock()
	s.serving = false
	s.mu.Unlock()

	return nil
}

// Stop stops the service.
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.serving {
		close(s.channel)
	}
}
