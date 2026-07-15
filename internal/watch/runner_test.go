package watch

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"
)

func TestRunnerInitialChangeAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fetches, emits := 0, 0
	r := Runner[int]{Interval: time.Millisecond, Fetch: func(context.Context) (Snapshot[int], error) {
		fetches++
		if fetches == 4 {
			cancel()
		}
		return Snapshot[int]{Data: min(fetches, 2)}, nil
	}, Hash: func(s Snapshot[int]) string { return strconv.Itoa(s.Data) }, Emit: func(Snapshot[int]) error { emits++; return nil }}
	err := r.Run(ctx, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if emits != 2 {
		t.Fatalf("emits=%d", emits)
	}
}
