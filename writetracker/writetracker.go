package writetracker

import (
	"io"
	"sync/atomic"
	"time"
)

/**
  WriteTracker is pass-through io.Writer that keeps track of when the last write occurred.
*/

type WriteTracker struct {
	lastUnixMilli *atomic.Int64
	w             io.Writer
}

// New created a new WriteTracker. startNow indicates whether it should start off pretending there was a write at instance creation.
func New(w io.Writer, startNow bool) *WriteTracker {
	wt := &WriteTracker{
		lastUnixMilli: &atomic.Int64{},
		w:             w,
	}
	if startNow {
		wt.updateLast()
	}
	return wt
}

func (w *WriteTracker) updateLast() {
	w.lastUnixMilli.Store(time.Now().UnixMilli())
}
func (w *WriteTracker) GetLastUnixMilli() int64 {
	return w.lastUnixMilli.Load()
}

func (w *WriteTracker) Write(p []byte) (n int, err error) {
	w.updateLast()
	return w.w.Write(p)
}
