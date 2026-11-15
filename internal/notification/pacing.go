package notification

import (
	"sync"
	"time"
)

const sendLimitPerSecond = 25

type sendPacer struct {
	mu    sync.Mutex
	limit int
	sent  []time.Time
}

func (p *sendPacer) wait(now time.Time) time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.limit <= 0 {
		p.limit = sendLimitPerSecond
	}

	cutoff := now.Add(-time.Second)
	kept := p.sent[:0]
	for _, at := range p.sent {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	p.sent = kept

	if len(p.sent) < p.limit {
		p.sent = append(p.sent, now)
		return 0
	}
	return p.sent[0].Add(time.Second).Sub(now)
}
