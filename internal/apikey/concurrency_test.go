package apikey

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLimiterAllowsExactlyTheLimitUnderParallelLoad(t *testing.T) {
	const workers = 64
	const limit = 8

	frozen := time.UnixMilli(0)
	limiter := NewLimiter(func() time.Time { return frozen })

	var allowed atomic.Int64
	var denied atomic.Int64

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if limiter.Allow(7, limit) {
				allowed.Add(1)
				return
			}
			denied.Add(1)
		}()
	}
	close(start)
	wg.Wait()

	if got := allowed.Load(); got != limit {
		t.Fatalf("allowed = %d, want %d", got, limit)
	}
	if got := denied.Load(); got != workers-limit {
		t.Fatalf("denied = %d, want %d", got, workers-limit)
	}
}

func TestStoreSurvivesParallelReadsAndARotation(t *testing.T) {
	store := setupStore(t)

	key, _, err := store.FindOrCreate(4242)
	if err != nil {
		t.Fatal(err)
	}
	token, err := store.Token(key)
	if err != nil {
		t.Fatal(err)
	}

	var invalid atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 8; j++ {
				if _, err := store.ByToken(token); err != nil {
					if !errors.Is(err, ErrInvalidKey) {
						t.Errorf("unexpected error: %v", err)
					}
					invalid.Add(1)
				}
				if err := store.Touch(key.ID, time.UnixMilli(1)); err != nil {
					t.Errorf("touch: %v", err)
				}
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := store.Rotate(4242); err != nil {
			t.Errorf("rotate: %v", err)
		}
	}()

	wg.Wait()

	if _, err := store.ByToken(token); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("the token must be invalid after the rotation, got %v", err)
	}

	rotated, err := store.ByChatID(4242)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed, err := store.Token(rotated); err != nil || refreshed == token {
		t.Fatalf("the rotated key must be usable and different: %q (%v)", refreshed, err)
	}
}
