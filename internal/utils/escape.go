package utils

import "strings"

var htmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

// EscapeHTML makes free text safe for a Telegram HTML message: a stray
// bracket or ampersand in user input would otherwise make the API reject the
// whole message.
func EscapeHTML(text string) string {
	return htmlEscaper.Replace(text)
}
