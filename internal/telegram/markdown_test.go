package telegram

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoapi"
)

func TestMarkdownVTurnsTheAuthoredHTMLIntoMarkers(t *testing.T) {
	cases := map[string]string{
		"":              "",
		"Математика":    "Математика",
		"1. Пара (лек)": `1\. Пара \(лек\)`,
		"<b>Жирный</b>": "*Жирный*",
		"<i>курсив</i>": "_курсив_",
		"<b>жирный <i>и курсив</i></b>":        "*жирный _и курсив_*",
		"<code>a.b(c)</code>":                  "`a.b(c)`",
		"<pre>a.b\nc</pre>":                    "```a.b\nc```",
		`<a href="https://x.y/?q=1">текст</a>`: "[текст](https://x.y/?q=1)",
		"&lt;some&gt;":                         `<some\>`,
		"<group>":                              `<group\>`,
		"стоимость 100%":                       "стоимость 100%",
		"a-b+c=d|e{f}!":                        `a\-b\+c\=d\|e\{f\}\!`,
		"<b>не закрыт":                         "*не закрыт*",
		"звёздочка * тут":                      `звёздочка \* тут`,
	}

	for input, want := range cases {
		if got := markdownV2(input); got != want {
			t.Errorf("markdownV2(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestMarkdownConstructorRewritesAnHTMLMessage(t *testing.T) {
	constructor := markdownConstructor{inner: telegoapi.DefaultConstructor{}}

	data, err := constructor.JSONRequest(&telego.SendMessageParams{
		ChatID:    telego.ChatID{ID: 42},
		Text:      "<b>Группа 100</b> (сегодня)",
		ParseMode: "HTML",
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload struct {
		Text      string `json:"text"`
		ParseMode string `json:"parse_mode"`
		ChatID    int64  `json:"chat_id"`
	}
	if err := json.Unmarshal(data.BodyRaw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ParseMode != parseMode {
		t.Fatalf("parse_mode = %q, want %q", payload.ParseMode, parseMode)
	}
	if payload.Text != `*Группа 100* \(сегодня\)` {
		t.Fatalf("text = %q", payload.Text)
	}
	if payload.ChatID != 42 {
		t.Fatalf("the other fields must survive the rewrite, chat_id = %d", payload.ChatID)
	}
}

func TestMarkdownConstructorKeepsOtherPayloads(t *testing.T) {
	constructor := markdownConstructor{inner: telegoapi.DefaultConstructor{}}

	plain, err := constructor.JSONRequest(&telego.SendMessageParams{
		ChatID: telego.ChatID{ID: 1},
		Text:   "без разметки",
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(plain.BodyRaw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Text != "без разметки" {
		t.Fatalf("plain text must stay as it is, got %q", payload.Text)
	}

	if _, ok := markdownJSON([]byte(`{"text":"x","parse_mode":"MarkdownV2"}`), FormatMarkdown); ok {
		t.Fatal("a MarkdownV2 payload must not be rewritten twice")
	}
	if _, ok := markdownJSON([]byte(`not json`), FormatMarkdown); ok {
		t.Fatal("a broken payload must not be rewritten")
	}
}

func TestMarkdownConstructorKeepsHTMLForAnHTMLChat(t *testing.T) {
	constructor := markdownConstructor{
		inner: telegoapi.DefaultConstructor{},
		formatOf: func(chatID int64) string {
			if chatID == 7 {
				return FormatHTML
			}
			return FormatMarkdown
		},
	}

	data, err := constructor.JSONRequest(&telego.SendMessageParams{
		ChatID:    telego.ChatID{ID: 7},
		Text:      "<b>Группа 100</b>",
		ParseMode: "HTML",
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload struct {
		Text      string `json:"text"`
		ParseMode string `json:"parse_mode"`
	}
	if err := json.Unmarshal(data.BodyRaw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ParseMode != "HTML" || payload.Text != "<b>Группа 100</b>" {
		t.Fatalf("an HTML chat must keep its payload, got %q / %q", payload.ParseMode, payload.Text)
	}
}

func TestMarkdownConstructorRewritesAMultipartCaption(t *testing.T) {
	constructor := markdownConstructor{inner: telegoapi.DefaultConstructor{}}

	data, err := constructor.MultipartRequest(map[string]string{
		"caption":    "<b>Снимок</b> 1.1",
		"parse_mode": "HTML",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(data.BodyStream)
	if err != nil {
		t.Fatal(err)
	}

	form := string(body)
	if !strings.Contains(form, `*Снимок* 1\.1`) {
		t.Fatalf("the caption was not converted:\n%s", form)
	}
	if !strings.Contains(form, "MarkdownV2") {
		t.Fatalf("parse_mode was not switched:\n%s", form)
	}
}

func TestFormatForReadsChatIDsOfBothShapes(t *testing.T) {
	var seen []int64
	constructor := markdownConstructor{formatOf: func(chatID int64) string {
		seen = append(seen, chatID)
		if chatID == 42 {
			return FormatHTML
		}
		return FormatMarkdown
	}}

	cases := []struct {
		name    string
		payload string
		want    string
	}{
		{"number", `{"chat_id": 42}`, FormatHTML},
		{"string", `{"chat_id": "42"}`, FormatHTML},
		{"padded string", `{"chat_id": " 42 "}`, FormatHTML},
		{"other chat", `{"chat_id": 7}`, FormatMarkdown},
		{"no chat", `{"text": "x"}`, FormatMarkdown},
		{"broken", `{`, FormatMarkdown},
		{"nonsense", `{"chat_id": "abc"}`, FormatMarkdown},
	}
	for _, tc := range cases {
		if got := constructor.formatFor([]byte(tc.payload)); got != tc.want {
			t.Errorf("%s: formatFor = %q, want %q", tc.name, got, tc.want)
		}
	}
	if len(seen) != 4 {
		t.Errorf("the chat callback must run only for parsable ids, got %v", seen)
	}
	if got := (markdownConstructor{}).formatFor([]byte(`{"chat_id": 42}`)); got != FormatMarkdown {
		t.Errorf("a constructor without a resolver must default to markdown, got %q", got)
	}
}
