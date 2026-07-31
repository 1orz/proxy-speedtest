package web

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type concurrentDetectWriter struct {
	active     atomic.Int64
	calls      atomic.Int64
	concurrent atomic.Bool
}

func (w *concurrentDetectWriter) WriteMessage(_ int, _ []byte) error {
	if w.active.Add(1) != 1 {
		w.concurrent.Store(true)
	}
	time.Sleep(time.Millisecond)
	w.calls.Add(1)
	w.active.Add(-1)
	return nil
}

func TestLockedMessageWriterSerializesWrites(t *testing.T) {
	underlying := &concurrentDetectWriter{}
	w := &lockedMessageWriter{writer: underlying}

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := w.WriteMessage(1, []byte("sample")); err != nil {
				t.Errorf("WriteMessage: %v", err)
			}
		}()
	}
	wg.Wait()

	if underlying.concurrent.Load() {
		t.Fatal("underlying writer was called concurrently")
	}
	if got := underlying.calls.Load(); got != 50 {
		t.Fatalf("calls=%d want 50", got)
	}
}
