package telegram

import "github.com/blindmaster24/MgkeTimetableBot/internal/formatter"

func (b *Bot) formatterLabels() map[string]string {
	keys := formatter.LabelKeys()

	labels := make(map[string]string, len(keys))
	for _, key := range keys {
		labels[key] = b.loc(key)
	}
	return labels
}
