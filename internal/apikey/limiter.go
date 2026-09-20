package apikey

import (
	"sync"
	"time"
)

const window = time.Second

type Limiter struct {
	mu   sync.Mutex
	hits map[int64][]time.Time
	now  func() time.Time
}

func NewLimiter(now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	return &Limiter{hits: make(map[int64][]time.Time), now: now}
}

func (l *Limiter) Allow(id int64, limit int) bool {
	if limit <= 0 {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	cutoff := now.Add(-window)

	kept := l.hits[id][:0]
	for _, at := range l.hits[id] {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}

	if len(kept) >= limit {
		l.hits[id] = kept
		return false
	}

	l.hits[id] = append(kept, now)
	return true
}
