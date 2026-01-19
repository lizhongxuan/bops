package audit

import (
	"encoding/json"
	"os"
	"sync"

	"bops/internal/core"
	"bops/internal/eventbus"
)

type Recorder struct {
	Path string
	mu   sync.Mutex
}

func (r *Recorder) Record(event core.Event) error {
	if r.Path == "" {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	file, err := os.OpenFile(r.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	return enc.Encode(event)
}

func (r *Recorder) Attach(bus *eventbus.Bus, buffer int) func() {
	sub := bus.Subscribe(buffer)
	stop := make(chan struct{})

	go func() {
		for {
			select {
			case event, ok := <-sub.C:
				if !ok {
					return
				}
				_ = r.Record(event)
			case <-stop:
				return
			}
		}
	}()

	return func() {
		close(stop)
		sub.Cancel()
	}
}
