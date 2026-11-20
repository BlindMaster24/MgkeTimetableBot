package notification

import (
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/ratelimit"
)

// sendLimitPerSecond follows the Bot API ceiling for one bot.
const sendLimitPerSecond = ratelimit.DefaultPerSecond

// sendPacer keeps a notification burst inside the Bot API rate limit.
type sendPacer struct {
	window ratelimit.Window
}

func (p *sendPacer) wait(now time.Time) time.Duration {
	return p.window.Reserve(now)
}
