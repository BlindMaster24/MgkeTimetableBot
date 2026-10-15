package telegram

import (
	"context"

	"github.com/mymmrac/telego"
)

const (
	transportSendMessage  = "send_message"
	transportSendDocument = "send_document"
	transportSendPhoto    = "send_photo"
	transportEditMessage  = "edit_message"
)

type counterSink interface {
	CountCommand(name string)
	CountCallback(prefix string)
	CountSendError(transport string)
}

func (b *Bot) SetCounterSink(sink counterSink) {
	b.counters = sink
}

func (b *Bot) countCommand(cmd Command) {
	if b.counters == nil || cmd == nil {
		return
	}
	b.counters.CountCommand(cmd.Name())
}

func (b *Bot) countCallback(prefix string) {
	if b.counters == nil || prefix == "" {
		return
	}
	b.counters.CountCallback(prefix)
}

func (b *Bot) countSendError(transport string, err error) error {
	if err != nil && b.counters != nil {
		b.counters.CountSendError(transport)
	}
	return err
}

func (b *Bot) sendMessageForID(chatID int64, text string) (int, error) {
	message, err := b.client.SendMessage(context.Background(), &telego.SendMessageParams{
		ChatID:    telego.ChatID{ID: chatID},
		Text:      text,
		ParseMode: "HTML",
	})
	if err != nil {
		return 0, b.countSendError(transportSendMessage, err)
	}
	return message.MessageID, nil
}
