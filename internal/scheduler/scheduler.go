package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type Config struct {
	Interval time.Duration
	Run      func(ctx context.Context) error
	Log      *slog.Logger
}

type Scheduler struct {
	cfg Config
}

func New(cfg Config) *Scheduler {
	return &Scheduler{cfg: cfg}
}

func (s *Scheduler) Run(ctx context.Context) error {
	if s.cfg.Run == nil {
		return fmt.Errorf("scheduler: run function is required")
	}
	if s.cfg.Interval <= 0 {
		return fmt.Errorf("scheduler: interval must be greater than zero")
	}

	s.run(ctx)
	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			s.run(ctx)
		}
	}
}

func (s *Scheduler) run(ctx context.Context) {
	if err := s.cfg.Run(ctx); err != nil && ctx.Err() == nil && s.cfg.Log != nil {
		s.cfg.Log.Warn("scheduled check failed", "err", err)
	}
}
