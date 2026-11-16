package telegram

func payloadFlag(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func (b *Bot) yesNo(v bool) string {
	if v {
		return b.loc("hint_yes")
	}
	return b.loc("hint_no")
}

func (b *Bot) yesNoCapital(v bool) string {
	if v {
		return b.loc("hint_yes_capital")
	}
	return b.loc("hint_no_capital")
}

func (b *Bot) onOff(v bool) string {
	if v {
		return b.loc("hint_on")
	}
	return b.loc("hint_off")
}

func (b *Bot) orNone(value string) string {
	if value == "" {
		return b.loc("hint_no")
	}
	return value
}

func (b *Bot) noYesSmile(v bool, text string) string {
	if v {
		return "✅ " + text
	}
	return "🚫 " + text
}

func (b *Bot) noYesSmileVolume(v bool, text string) string {
	if v {
		return "🔈 " + text + ": " + b.yesNoCapital(true)
	}
	return "🔇 " + text + ": " + b.yesNoCapital(false)
}

func (b *Bot) sourceCheck(label string, active bool) string {
	if active {
		return "✅ " + label
	}
	return label
}
