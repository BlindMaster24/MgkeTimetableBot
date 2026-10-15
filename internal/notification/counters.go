package notification

const (
	kindDayNotice     = "day"
	kindDayChange     = "day_update"
	kindNewWeek       = "week"
	kindWeekWithdrawn = "week_withdrawn"
	kindCalls         = "calls"
	kindParserError   = "parser_error"
	kindReminder      = "reminder"
	kindHealth        = "health"
)

type notificationSink interface {
	CountNotification(kind string)
}

func countNotification(sink notificationSink, kind string, err error) {
	if sink == nil || err != nil {
		return
	}
	sink.CountNotification(kind)
}

func (n *EventNotifier) SetCounterSink(sink notificationSink) {
	n.counters = sink
}

func (n *EventNotifier) sendText(chatID int64, text, kind string) error {
	err := n.sender.SendText(chatID, text)
	countNotification(n.counters, kind, err)
	return err
}

func (n *EventNotifier) sendTextWithButtons(chatID int64, text string, buttons []KeyboardButton, kind string) error {
	err := n.sender.SendTextWithButtons(chatID, text, buttons)
	countNotification(n.counters, kind, err)
	return err
}

func (n *HealthNotifier) SetCounterSink(sink notificationSink) {
	n.counters = sink
}
