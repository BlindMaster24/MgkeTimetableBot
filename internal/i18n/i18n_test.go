package i18n

import "testing"

func TestI18nKnownKey(t *testing.T) {
	loc := New("ru")

	result := loc.T("ru", "welcome", nil)
	if result == "" || result == "[welcome]" {
		t.Errorf("expected translated welcome text, got %s", result)
	}
}

func TestI18nUnknownKey(t *testing.T) {
	loc := New("ru")

	result := loc.T("ru", "nonexistent_key_xyz", nil)
	if result != "[nonexistent_key_xyz]" {
		t.Errorf("expected fallback [nonexistent_key_xyz], got %s", result)
	}
}

func TestI18nTemplateData(t *testing.T) {
	loc := New("ru")

	result := loc.T("ru", "enter_group_number", map[string]interface{}{"Group": "63ТП"})
	if result == "" || result == "[enter_group_number]" {
		t.Errorf("expected templated text, got %s", result)
	}
}

func TestI18nCommandDesc(t *testing.T) {
	loc := New("ru")

	keys := []string{"cmd_start", "cmd_help", "cmd_cancel", "cmd_setup", "cmd_day", "cmd_week", "cmd_calls", "cmd_about", "cmd_group", "cmd_teacher", "cmd_settings", "cmd_image"}

	for _, key := range keys {
		result := loc.T("ru", key, nil)
		if result == "" || result == "["+key+"]" {
			t.Errorf("key %s: expected translation, got %s", key, result)
		}
	}
}

func TestI18nButtonKeys(t *testing.T) {
	loc := New("ru")

	keys := []string{"button_day", "button_week", "button_calls", "button_about", "button_group", "button_teacher", "button_ics", "button_image", "button_settings", "button_cancel", "button_setup"}

	for _, key := range keys {
		result := loc.T("ru", key, nil)
		if result == "" || result == "["+key+"]" {
			t.Errorf("key %s: expected translation, got %s", key, result)
		}
	}
}

func TestI18nCallsKeys(t *testing.T) {
	loc := New("ru")

	keys := []string{"calls_not_configured", "calls_weekdays", "calls_saturday"}

	for _, key := range keys {
		result := loc.T("ru", key, nil)
		if result == "" || result == "["+key+"]" {
			t.Errorf("key %s: expected translation, got %s", key, result)
		}
	}
}
