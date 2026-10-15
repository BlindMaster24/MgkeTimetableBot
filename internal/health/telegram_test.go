package health

import (
	"maps"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
)

var counterKeyRe = regexp.MustCompile(`^[a-z0-9_-]{0,64}$`)

func counterTotal(values map[string]int64) int64 {
	total := int64(0)
	for _, value := range values {
		total += value
	}
	return total
}

func TestTelegramCountersSanitizeKeys(t *testing.T) {
	cases := map[string]string{
		"  /Day  ":               "day",
		"DAY":                    "day",
		"/archive list":          "archivelist",
		"menu:main":              "menumain",
		"send_message":           "send_message",
		"answer-callback":        "answer-callback",
		"day\nweek":              "dayweek",
		"/start 63 \"evil\"":     "start63evil",
		"дай расписание":         "",
		"！！":                     "",
		"":                       "",
		"\n\t":                   "",
		strings.Repeat("a", 100): strings.Repeat("a", 64),
	}

	for raw, want := range cases {
		counters := NewTelegramCounters()
		counters.CountCommand(raw)

		stats := counters.TelegramStats()
		if want == "" {
			if len(stats.Commands) != 0 {
				t.Errorf("CountCommand(%q) must be ignored, got %v", raw, stats.Commands)
			}
			continue
		}
		if !maps.Equal(stats.Commands, map[string]int64{want: 1}) {
			t.Errorf("CountCommand(%q) = %v, want {%s: 1}", raw, stats.Commands, want)
		}
		if !counterKeyRe.MatchString(want) {
			t.Errorf("sanitized key %q leaves the allowed alphabet", want)
		}
	}
}

func TestTelegramCountersKeepDictionariesApart(t *testing.T) {
	counters := NewTelegramCounters()
	counters.CountCommand("/day")
	counters.CountCommand("/day")
	counters.CountCallback("day")
	counters.CountNotification("reminder")
	counters.CountSendError("send_message")
	counters.CountSendError("send_message")
	counters.CountSendError("send_photo")

	stats := counters.TelegramStats()

	if !maps.Equal(stats.Commands, map[string]int64{"day": 2}) {
		t.Errorf("commands = %v", stats.Commands)
	}
	if !maps.Equal(stats.Callbacks, map[string]int64{"day": 1}) {
		t.Errorf("callbacks = %v", stats.Callbacks)
	}
	if !maps.Equal(stats.Notifications, map[string]int64{"reminder": 1}) {
		t.Errorf("notifications = %v", stats.Notifications)
	}
	if !maps.Equal(stats.Transports, map[string]int64{"send_message": 2, "send_photo": 1}) {
		t.Errorf("transports = %v", stats.Transports)
	}
}

func TestTelegramCountersStayEmptyWithoutTraffic(t *testing.T) {
	stats := NewTelegramCounters().TelegramStats()

	if stats.Commands != nil || stats.Callbacks != nil || stats.Notifications != nil || stats.Transports != nil {
		t.Errorf("an idle sink must report nothing, got %+v", stats)
	}
}

func TestTelegramCountersTolerateAMissingSink(t *testing.T) {
	var counters *TelegramCounters

	counters.CountCommand("/day")
	counters.CountCallback("day")
	counters.CountNotification("reminder")
	counters.CountSendError("send_message")

	if stats := counters.TelegramStats(); stats.Commands != nil || stats.Transports != nil {
		t.Errorf("a nil sink must stay silent, got %+v", stats)
	}
}

func TestTelegramCountersCapTheKeySpace(t *testing.T) {
	const calls = maxCounterKeys * 3

	counters := NewTelegramCounters()
	for i := 0; i < calls; i++ {
		counters.CountCommand("cmd" + strconv.Itoa(i))
	}

	stats := counters.TelegramStats()
	if len(stats.Commands) > maxCounterKeys {
		t.Fatalf("keys = %d, want at most %d", len(stats.Commands), maxCounterKeys)
	}

	tracked := 0
	for key := range stats.Commands {
		if key != overflowCounterKey {
			tracked++
		}
	}
	if tracked != maxCounterKeys-1 {
		t.Errorf("tracked keys = %d, want %d", tracked, maxCounterKeys-1)
	}
	if want := int64(calls - tracked); stats.Commands[overflowCounterKey] != want {
		t.Errorf("other = %d, want %d", stats.Commands[overflowCounterKey], want)
	}
	if total := counterTotal(stats.Commands); total != calls {
		t.Errorf("the cap lost counts: %d of %d", total, calls)
	}
}

func TestTelegramCountersCountUnknownKeysIntoOther(t *testing.T) {
	counters := NewTelegramCounters()
	for i := 0; i < maxCounterKeys-1; i++ {
		counters.CountCommand("cmd" + strconv.Itoa(i))
	}
	counters.CountCommand("late")
	counters.CountCommand("later")
	counters.CountCommand("late")

	stats := counters.TelegramStats()
	if stats.Commands[overflowCounterKey] != 3 {
		t.Errorf("other = %d, want 3 (%v)", stats.Commands[overflowCounterKey], stats.Commands)
	}
	if _, tracked := stats.Commands["late"]; tracked {
		t.Errorf("a key over the cap must not be tracked: %v", stats.Commands)
	}
	if total := counterTotal(stats.Commands); total != maxCounterKeys+2 {
		t.Errorf("counts = %d, want %d", total, maxCounterKeys+2)
	}
}

func TestTelegramCountersSurviveConcurrentTraffic(t *testing.T) {
	counters := NewTelegramCounters()

	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				counters.CountCommand("cmd" + strconv.Itoa(i%10))
				counters.CountCallback("cb" + strconv.Itoa(i%5))
				counters.CountNotification("reminder")
				counters.CountSendError("send_message")
				_ = counters.TelegramStats()
			}
		}(worker)
	}
	wg.Wait()

	stats := counters.TelegramStats()
	for _, dictionary := range []map[string]int64{stats.Commands, stats.Callbacks, stats.Notifications, stats.Transports} {
		if total := counterTotal(dictionary); total != 400 {
			t.Errorf("dictionary lost counts under load: %d, want 400", total)
		}
	}
}

func FuzzCounterKeyStaysBounded(f *testing.F) {
	for _, seed := range []string{"/day", "DAY", "notification_reminder", "", "\n", "дай расписание", "send_message", strings.Repeat("x", 200)} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 256 {
			raw = raw[:256]
		}

		counters := NewTelegramCounters()
		counters.CountCommand(raw)
		counters.CountCallback(raw)
		counters.CountNotification(raw)
		counters.CountSendError(raw)

		sanitized := sanitizeCounterKey(raw)
		if sanitizeCounterKey(sanitized) != sanitized {
			t.Fatalf("sanitizing %q twice changes it: %q", raw, sanitizeCounterKey(sanitized))
		}

		stats := counters.TelegramStats()
		total := int64(0)
		for _, dictionary := range []map[string]int64{stats.Commands, stats.Callbacks, stats.Notifications, stats.Transports} {
			if len(dictionary) > maxCounterKeys {
				t.Fatalf("the sink kept %d keys, want at most %d", len(dictionary), maxCounterKeys)
			}
			for key, value := range dictionary {
				if !counterKeyRe.MatchString(key) {
					t.Fatalf("key %q is not sanitized from %q", key, raw)
				}
				total += value
			}
		}

		want := int64(0)
		if sanitized != "" {
			want = 4
		}
		if total != want {
			t.Fatalf("counted %d samples for %q, want %d", total, raw, want)
		}
	})
}
