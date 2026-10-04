package reminder

import (
	"math/rand"
	"time"
)

func Jitter(window time.Duration, r *rand.Rand) time.Duration {
	if window <= 0 || r == nil {
		return 0
	}
	return time.Duration(r.Int63n(int64(window)))
}

func ReminderTime(start time.Time, lead, window time.Duration, r *rand.Rand) time.Time {
	return start.Add(-lead).Add(Jitter(window, r))
}
