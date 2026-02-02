package aiworkflow

import (
	"context"
	"testing"
	"time"
)

func TestSessionLaneSerial(t *testing.T) {
	lane := NewSessionLane()
	ctx := context.Background()
	startedFirst := make(chan struct{})
	startedSecond := make(chan struct{})
	release := make(chan struct{})

	go func() {
		_ = lane.Do(ctx, "session-1", func() error {
			close(startedFirst)
			<-release
			return nil
		})
	}()

	select {
	case <-startedFirst:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("first lane task did not start")
	}

	go func() {
		_ = lane.Do(ctx, "session-1", func() error {
			close(startedSecond)
			return nil
		})
	}()

	select {
	case <-startedSecond:
		t.Fatalf("second lane task started before release")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)

	select {
	case <-startedSecond:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("second lane task did not start after release")
	}
}

func TestGlobalLaneSerial(t *testing.T) {
	lane := NewGlobalLane(1)
	ctx := context.Background()
	startedFirst := make(chan struct{})
	startedSecond := make(chan struct{})
	release := make(chan struct{})

	go func() {
		_ = lane.Do(ctx, func() error {
			close(startedFirst)
			<-release
			return nil
		})
	}()

	select {
	case <-startedFirst:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("first global task did not start")
	}

	go func() {
		_ = lane.Do(ctx, func() error {
			close(startedSecond)
			return nil
		})
	}()

	select {
	case <-startedSecond:
		t.Fatalf("second global task started before release")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)

	select {
	case <-startedSecond:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("second global task did not start after release")
	}
}
