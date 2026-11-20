// Package ratelimit paces outgoing Bot API calls.
package ratelimit

import (
	"sync"
	"time"
)

// DefaultPerSecond is the ceiling Telegram documents for one bot: about 30
// messages per second across all chats. A single private chat also gets at
// most one message per second, which a broadcast never breaks because it sends
// one message per chat, and the same rule makes a flood control reply (429)
// with retry_after unlikely.
const DefaultPerSecond = 30

// Window is a sliding one-second window that keeps a burst under a limit.
// The zero value is ready to use and falls back to DefaultPerSecond.
type Window struct {
	mu    sync.Mutex
	limit int
	sent  []time.Time
}

// New returns a window with the given limit; a non-positive limit means
// DefaultPerSecond.
func New(limit int) *Window {
	return &Window{limit: limit}
}

// Reserve asks for a slot at now. When the window has room the slot is taken
// and the result is zero; otherwise nothing is taken and the result is how
// long the caller has to wait before asking again.
func (w *Window) Reserve(now time.Time) time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.limit <= 0 {
		w.limit = DefaultPerSecond
	}

	cutoff := now.Add(-time.Second)
	kept := w.sent[:0]
	for _, at := range w.sent {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	w.sent = kept

	if len(w.sent) < w.limit {
		w.sent = append(w.sent, now)
		return 0
	}
	return w.sent[0].Add(time.Second).Sub(now)
}

// Wait blocks until the window has room for one more call.
func (w *Window) Wait() {
	for {
		delay := w.Reserve(time.Now())
		if delay <= 0 {
			return
		}
		time.Sleep(delay)
	}
}
