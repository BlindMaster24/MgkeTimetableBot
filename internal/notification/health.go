package notification

import (
	"sync"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
)

const (
	healthService  = "telegram"
	healthCooldown = 30 * time.Minute
)

type alertState struct {
	Active map[string]string `json:"active,omitempty"`
}

type HealthNotifier struct {
	tracker  *health.Tracker
	log      *logger.Logger
	sender   EventSender
	chats    EventChatFinder
	cooldown time.Duration
	store    health.StateStore

	mu       sync.Mutex
	active   map[string]time.Time
	restored bool
}

func NewHealthNotifier(tracker *health.Tracker, log *logger.Logger, sender EventSender, chats EventChatFinder, cooldown time.Duration, store health.StateStore) *HealthNotifier {
	if cooldown <= 0 {
		cooldown = healthCooldown
	}
	return &HealthNotifier{
		tracker:  tracker,
		log:      log,
		sender:   sender,
		chats:    chats,
		cooldown: cooldown,
		store:    store,
		active:   make(map[string]time.Time),
	}
}

func (n *HealthNotifier) Check() {
	if n.tracker == nil {
		return
	}

	n.restore()

	alerts := n.tracker.Alerts()
	now := time.Now()

	pending, recovered := n.diff(alerts, now)
	if len(pending) == 0 && len(recovered) == 0 {
		return
	}
	n.flush()

	chats, err := n.chats.FindAdminChats(healthService)
	if err != nil {
		n.log.Error().Err(err).Msg("failed to find admin chats for health alert")
		return
	}
	if len(chats) == 0 {
		n.log.Warn().Msg("no admin chats for health alert")
		return
	}

	for _, alert := range pending {
		n.broadcast(chats, healthAlertMessage(alert))
	}
	for _, key := range recovered {
		n.broadcast(chats, healthRecoveryMessage(key))
	}
}

func (n *HealthNotifier) diff(alerts []health.Alert, now time.Time) ([]health.Alert, []string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	seen := make(map[string]bool, len(alerts))
	var pending []health.Alert

	for _, alert := range alerts {
		seen[alert.Key] = true
		sentAt, exists := n.active[alert.Key]
		if exists && now.Sub(sentAt) < n.cooldown {
			continue
		}
		n.active[alert.Key] = now
		pending = append(pending, alert)
	}

	var recovered []string
	for key := range n.active {
		if !seen[key] {
			delete(n.active, key)
			recovered = append(recovered, key)
		}
	}
	return pending, recovered
}

func (n *HealthNotifier) restore() {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.restored || n.store == nil {
		return
	}
	n.restored = true

	var state alertState
	if err := health.LoadState(n.store, health.AlertsStateKey, &state); err != nil {
		n.log.Warn().Err(err).Msg("failed to restore health alert state")
		return
	}

	for key, value := range state.Active {
		at, err := time.Parse(time.RFC3339, value)
		if err != nil {
			continue
		}
		n.active[key] = at
	}
}

func (n *HealthNotifier) flush() {
	n.mu.Lock()
	state := alertState{Active: make(map[string]string, len(n.active))}
	for key, at := range n.active {
		state.Active[key] = at.Format(time.RFC3339)
	}
	n.mu.Unlock()

	if err := health.SaveState(n.store, health.AlertsStateKey, state); err != nil {
		n.log.Warn().Err(err).Msg("failed to persist health alert state")
	}
}

func (n *HealthNotifier) broadcast(chats []*EventChat, message string) {
	for _, chat := range chats {
		id := chat.ID
		if chat.PeerID != 0 {
			id = chat.PeerID
		}
		if err := n.sender.SendText(id, message); err != nil {
			n.log.Error().Err(err).Int64("chat", id).Msg("failed to send health alert")
		}
	}
}

func healthAlertMessage(alert health.Alert) string {
	return "⚠️ " + healthAlertTitle(alert.Key) + "\n" + alert.Detail
}

func healthRecoveryMessage(key string) string {
	return "✅ " + healthAlertTitle(key) + " — восстановлено"
}

func healthAlertTitle(key string) string {
	switch key {
	case health.AlertParserFailures:
		return "Парсер расписания падает"
	case health.AlertParserStale:
		return "Расписание давно не обновлялось"
	case health.AlertParserLayout:
		return "Парсер перестал находить данные на сайте"
	case health.AlertParserGuard:
		return "Парсер резко потерял данные, кэш оставлен прежним"
	case health.AlertCalendarFailures:
		return "Ошибки синхронизации Google Calendar"
	case health.AlertCalendarStale:
		return "Google Calendar давно не синхронизировался"
	case health.AlertAPIErrors:
		return "Ошибки HTTP API"
	}
	return key
}
