package telegram

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/mymmrac/telego/telegoapi"
)

// parseMode is the Bot API format of every outgoing text by default. The bot
// authors its texts in HTML — the localizer files are HTML — and markdownV2
// rewrites them into MarkdownV2 right before the send call. A chat that asked
// for HTML keeps the text as it is.
const parseMode = "MarkdownV2"

// FormatMarkdown and FormatHTML are the values of the per-chat text_format
// column; an empty value means MarkdownV2, the default.
const (
	FormatMarkdown = "md"
	FormatHTML     = "html"
)

var markdownEscaper = strings.NewReplacer(
	`\`, `\\`,
	"_", `\_`, "*", `\*`, "[", `\[`, "]", `\]`,
	"(", `\(`, ")", `\)`, "~", `\~`, "`", "\\`",
	">", `\>`, "#", `\#`, "+", `\+`, "-", `\-`,
	"=", `\=`, "|", `\|`, "{", `\{`, "}", `\}`,
	".", `\.`, "!", `\!`,
)

// insideCode keeps the escapes the MarkdownV2 parser still honours between
// backticks: everything else is literal there.
var insideCode = strings.NewReplacer(`\`, `\\`, "`", "\\`")

var htmlEntity = strings.NewReplacer(
	"&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'", "&apos;", "'", "&amp;", "&",
)

var markdownTag = regexp.MustCompile(`^<(/?)(b|strong|i|em|u|s|strike|del|code|pre|tg-spoiler|blockquote|a)(\s+href="([^"]*)")?>`)

type markdownFrame struct {
	close string
	code  bool
}

// markdownV2 rewrites the small HTML subset the bot authors — b, strong, i, em,
// u, s, strike, del, code, pre, tg-spoiler, blockquote and a href — into
// MarkdownV2 with the matching markers, and escapes every other character the
// parser would otherwise read as markup.
func markdownV2(text string) string {
	var out strings.Builder
	out.Grow(len(text) * 2)

	var frames []markdownFrame
	inCode := false
	writeText := func(chunk string) {
		if chunk == "" {
			return
		}
		chunk = htmlEntity.Replace(chunk)
		if inCode {
			out.WriteString(insideCode.Replace(chunk))
			return
		}
		out.WriteString(markdownEscaper.Replace(chunk))
	}

	for len(text) > 0 {
		if text[0] != '<' {
			next := strings.IndexByte(text, '<')
			if next < 0 {
				writeText(text)
				break
			}
			writeText(text[:next])
			text = text[next:]
			continue
		}

		match := markdownTag.FindStringSubmatch(text)
		if match == nil {
			writeText("<")
			text = text[1:]
			continue
		}
		text = text[len(match[0]):]

		name := strings.ToLower(match[2])
		closing := match[1] == "/"

		if closing {
			if len(frames) == 0 {
				continue
			}
			frame := frames[len(frames)-1]
			frames = frames[:len(frames)-1]
			out.WriteString(frame.close)
			if frame.code {
				inCode = false
			}
			continue
		}

		frame := markdownFrame{}
		switch name {
		case "b", "strong":
			out.WriteString("*")
			frame.close = "*"
		case "i", "em":
			out.WriteString("_")
			frame.close = "_"
		case "u":
			out.WriteString("__")
			frame.close = "__"
		case "s", "strike", "del":
			out.WriteString("~")
			frame.close = "~"
		case "tg-spoiler":
			out.WriteString("||")
			frame.close = "||"
		case "blockquote":
			out.WriteString(">")
			frame.close = ""
		case "code":
			out.WriteString("`")
			frame.close = "`"
			frame.code = true
			inCode = true
		case "pre":
			out.WriteString("```")
			frame.close = "```"
			frame.code = true
			inCode = true
		case "a":
			out.WriteString("[")
			frame.close = "](" + escapeMarkdownURL(match[4]) + ")"
		}
		frames = append(frames, frame)
	}

	// An unbalanced tag would leave a marker without its pair; close what is
	// still open so the message reaches the user instead of an API error.
	for i := len(frames) - 1; i >= 0; i-- {
		out.WriteString(frames[i].close)
	}

	return out.String()
}

// escapeMarkdownURL keeps a link target intact: only the backslash and the
// closing bracket need escaping inside the link part.
func escapeMarkdownURL(url string) string {
	return strings.NewReplacer(`\`, `\\`, ")", `\)`).Replace(url)
}

// markdownConstructor rewrites the formatted payloads on their way to the Bot
// API: the bot authors HTML, and MarkdownV2 is what the clients get unless the
// chat asked for HTML. The chat id travels inside the same payload, so the
// choice costs no extra round trip.
type markdownConstructor struct {
	inner    telegoapi.RequestConstructor
	formatOf func(chatID int64) string
}

func (c markdownConstructor) JSONRequest(parameters any) (*telegoapi.RequestData, error) {
	data, err := c.inner.JSONRequest(parameters)
	if err != nil {
		return nil, err
	}
	if rewritten, ok := markdownJSON(data.BodyRaw, c.formatFor(data.BodyRaw)); ok {
		data.BodyRaw = rewritten
	}
	return data, nil
}

func (c markdownConstructor) MultipartRequest(parameters map[string]string, files map[string]telegoapi.NamedReader) (*telegoapi.RequestData, error) {
	if c.formatFromString(parameters["chat_id"]) != FormatHTML && strings.EqualFold(parameters["parse_mode"], "HTML") {
		for _, key := range []string{"text", "caption"} {
			if text, ok := parameters[key]; ok {
				parameters[key] = markdownV2(text)
			}
		}
		parameters["parse_mode"] = parseMode
	}
	return c.inner.MultipartRequest(parameters, files)
}

// formatFor reads the chat id of a JSON payload and answers the format that
// chat asked for.
func (c markdownConstructor) formatFor(payload []byte) string {
	var head struct {
		ChatID json.RawMessage `json:"chat_id"`
	}
	if err := json.Unmarshal(payload, &head); err != nil || len(head.ChatID) == 0 {
		return FormatMarkdown
	}
	var id int64
	if err := json.Unmarshal(head.ChatID, &id); err == nil {
		return c.chatFormat(id)
	}
	var text string
	if err := json.Unmarshal(head.ChatID, &text); err == nil {
		return c.formatFromString(text)
	}
	return FormatMarkdown
}

func (c markdownConstructor) formatFromString(raw string) string {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return FormatMarkdown
	}
	return c.chatFormat(id)
}

func (c markdownConstructor) chatFormat(chatID int64) string {
	if c.formatOf == nil {
		return FormatMarkdown
	}
	if c.formatOf(chatID) == FormatHTML {
		return FormatHTML
	}
	return FormatMarkdown
}

// markdownJSON rewrites one JSON payload: only an HTML payload with a text or
// a caption is touched, everything else goes to the API as it is.
func markdownJSON(body []byte, format string) ([]byte, bool) {
	if format == FormatHTML {
		return nil, false
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, false
	}

	var mode string
	if raw, ok := fields["parse_mode"]; !ok || json.Unmarshal(raw, &mode) != nil || !strings.EqualFold(mode, "HTML") {
		return nil, false
	}

	changed := false
	for _, key := range []string{"text", "caption"} {
		raw, ok := fields[key]
		if !ok {
			continue
		}
		var text string
		if json.Unmarshal(raw, &text) != nil {
			continue
		}
		rewritten, err := json.Marshal(markdownV2(text))
		if err != nil {
			return nil, false
		}
		fields[key] = rewritten
		changed = true
	}
	if !changed {
		return nil, false
	}

	modeJSON, err := json.Marshal(parseMode)
	if err != nil {
		return nil, false
	}
	fields["parse_mode"] = modeJSON

	rewritten, err := json.Marshal(fields)
	if err != nil {
		return nil, false
	}
	return rewritten, true
}
