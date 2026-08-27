package i18n

import (
	"embed"
	"encoding/json"
	"fmt"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localesFS embed.FS

type Localizer struct {
	bundle *i18n.Bundle
}

func New(defaultLang string) *Localizer {
	bundle := i18n.NewBundle(language.Make(defaultLang))
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	entries, err := localesFS.ReadDir("locales")
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				data, _ := localesFS.ReadFile("locales/" + entry.Name())
				bundle.ParseMessageFileBytes(data, entry.Name())
			}
		}
	}

	return &Localizer{bundle: bundle}
}

func (l *Localizer) Localizer(lang string) *i18n.Localizer {
	return i18n.NewLocalizer(l.bundle, lang)
}

func (l *Localizer) T(lang, msgID string, data map[string]interface{}) string {
	loc := l.Localizer(lang)
	msg, err := loc.Localize(&i18n.LocalizeConfig{
		MessageID:    msgID,
		TemplateData: data,
	})
	if err != nil {
		return fmt.Sprintf("[%s]", msgID)
	}
	return msg
}
