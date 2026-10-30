package scheduler

import (
	"context"
	"testing"
	"time"
)

func TestSchedulerRunsImmediately(t *testing.T) {
	calls := make(chan struct{}, 2)
	s := New(Config{
		Interval: time.Hour,
		Run: func(context.Context) error {
			calls <- struct{}{}
			return nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	select {
	case <-calls:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not run immediately")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop")
	}
}
