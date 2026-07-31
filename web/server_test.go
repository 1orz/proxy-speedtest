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

type recordingWriter struct {
	calls atomic.Int64
	err   error
}

func (w *recordingWriter) WriteMessage(_ int, _ []byte) error {
	w.calls.Add(1)
	return w.err
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

func TestFanoutMessageWriterWritesAllDestinations(t *testing.T) {
	wantErr := &writerError{}
	first := &recordingWriter{err: wantErr}
	second := &recordingWriter{}
	w := &fanoutMessageWriter{writers: []MessageWriter{first, second}}

	if err := w.WriteMessage(1, []byte("sample")); err != wantErr {
		t.Fatalf("error=%v want %v", err, wantErr)
	}
	if first.calls.Load() != 1 || second.calls.Load() != 1 {
		t.Fatalf("fanout calls=%d/%d want 1/1", first.calls.Load(), second.calls.Load())
	}
}

type writerError struct{}

func (*writerError) Error() string { return "writer error" }
