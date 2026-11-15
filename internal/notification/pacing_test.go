package notification

import (
	"testing"
	"time"
)

func TestSendPacerKeepsABurstInsideTheSecond(t *testing.T) {
	p := sendPacer{}
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	for i := 0; i < sendLimitPerSecond; i++ {
		if delay := p.wait(now); delay != 0 {
			t.Fatalf("send %d inside the limit must not wait, got %s", i+1, delay)
		}
	}

	if delay := p.wait(now); delay <= 0 {
		t.Fatalf("the request over the limit must wait, got %s", delay)
	}

	if delay := p.wait(now.Add(time.Second)); delay != 0 {
		t.Fatalf("a second later the window must be free, got %s", delay)
	}
}
