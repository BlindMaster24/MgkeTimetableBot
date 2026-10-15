package health

import (
	"strings"
	"sync"
)

const (
	maxCounterKeys      = 128
	maxCounterKeyLength = 64
	overflowCounterKey  = "other"
)

type TelegramStats struct {
	Commands      map[string]int64 `json:"commands,omitempty"`
	Callbacks     map[string]int64 `json:"callbacks,omitempty"`
	Notifications map[string]int64 `json:"notifications,omitempty"`
	Transports    map[string]int64 `json:"transports,omitempty"`
}

type counterSet struct {
	counts map[string]int64
	excess int64
}

func (s *counterSet) add(key string) {
	if key == "" {
		return
	}
	if s.counts == nil {
		s.counts = make(map[string]int64)
	}
	if _, known := s.counts[key]; known {
		s.counts[key]++
		return
	}
	if len(s.counts) >= maxCounterKeys-1 {
		s.excess++
		return
	}
	s.counts[key] = 1
}

func (s counterSet) stats() map[string]int64 {
	if len(s.counts) == 0 && s.excess == 0 {
		return nil
	}

	out := make(map[string]int64, len(s.counts)+1)
	for key, value := range s.counts {
		out[key] = value
	}
	if s.excess > 0 {
		out[overflowCounterKey] += s.excess
	}
	return out
}

type TelegramCounters struct {
	mu            sync.Mutex
	commands      counterSet
	callbacks     counterSet
	notifications counterSet
	transports    counterSet
}

func NewTelegramCounters() *TelegramCounters {
	return &TelegramCounters{}
}

func (c *TelegramCounters) CountCommand(name string) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.commands.add(sanitizeCounterKey(name))
}

func (c *TelegramCounters) CountCallback(prefix string) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.callbacks.add(sanitizeCounterKey(prefix))
}

func (c *TelegramCounters) CountNotification(kind string) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.notifications.add(sanitizeCounterKey(kind))
}

func (c *TelegramCounters) CountSendError(transport string) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.transports.add(sanitizeCounterKey(transport))
}

func (c *TelegramCounters) TelegramStats() TelegramStats {
	if c == nil {
		return TelegramStats{}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return TelegramStats{
		Commands:      c.commands.stats(),
		Callbacks:     c.callbacks.stats(),
		Notifications: c.notifications.stats(),
		Transports:    c.transports.stats(),
	}
}

func sanitizeCounterKey(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return ""
	}

	var builder strings.Builder
	for _, symbol := range trimmed {
		if !counterKeySymbol(symbol) {
			continue
		}
		if builder.Len() >= maxCounterKeyLength {
			break
		}
		builder.WriteRune(symbol)
	}
	return builder.String()
}

func counterKeySymbol(symbol rune) bool {
	switch {
	case symbol >= 'a' && symbol <= 'z':
		return true
	case symbol >= '0' && symbol <= '9':
		return true
	case symbol == '_' || symbol == '-':
		return true
	}
	return false
}
