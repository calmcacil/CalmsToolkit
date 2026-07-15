// Package watch provides the shared polling and change-detection lifecycle.
package watch

import (
	"context"
	"time"
)

// Snapshot carries usable data and source warnings from one fetch.
type Snapshot[T any] struct {
	Data     T
	Partial  bool
	Warnings []string
}

// Runner performs an initial fetch followed by cancellable polling.
type Runner[T any] struct {
	Interval time.Duration
	Fetch    func(context.Context) (Snapshot[T], error)
	Hash     func(Snapshot[T]) string
	Emit     func(Snapshot[T]) error
}

// Run emits the initial snapshot and subsequent changed snapshots.
func (r Runner[T]) Run(ctx context.Context, once bool) error {
	if r.Interval <= 0 {
		r.Interval = time.Second
	}
	last := ""
	for {
		snapshot, err := r.Fetch(ctx)
		if err != nil {
			return err
		}
		hash := r.Hash(snapshot)
		if hash != last {
			if err := r.Emit(snapshot); err != nil {
				return err
			}
			last = hash
		}
		if once {
			return nil
		}
		timer := time.NewTimer(r.Interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}
