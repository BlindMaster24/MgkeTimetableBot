package telegram

func payloadFlag(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func yesNo(v bool) string {
	if v {
		return "да"
	}
	return "нет"
}

func yesNoCapital(v bool) string {
	if v {
		return "Да"
	}
	return "Нет"
}

func onOff(v bool) string {
	if v {
		return "включено"
	}
	return "выключено"
}

func orNone(value string) string {
	if value == "" {
		return "нет"
	}
	return value
}

func noYesSmile(v bool, text string) string {
	if v {
		return "✅ " + text
	}
	return "🚫 " + text
}

func noYesSmileVolume(v bool, text string) string {
	if v {
		return "🔈 " + text + ": " + yesNoCapital(true)
	}
	return "🔇 " + text + ": " + yesNoCapital(false)
}

func sourceCheck(label string, active bool) string {
	if active {
		return "✅ " + label
	}
	return label
}
