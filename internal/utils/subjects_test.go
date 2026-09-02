package utils

import "testing"

func TestGetFullSubjectName(t *testing.T) {
	if len(subjectsByShort) == 0 {
		t.Fatal("subjects.csv not loaded")
	}

	full := GetFullSubjectName("АвтомПр")
	if full == "АвтомПр" {
		t.Errorf("expected expansion of short name, got %q", full)
	}

	unknown := GetFullSubjectName("НесуществующийПредмет")
	if unknown != "НесуществующийПредмет" {
		t.Errorf("unknown subject should be returned as-is, got %q", unknown)
	}
}

func TestGetShortSubjectName(t *testing.T) {
	if len(subjectsByFull) == 0 {
		t.Fatal("subjects.csv not loaded")
	}

	unknown := GetShortSubjectName("НесуществующийПредмет")
	if unknown != "НесуществующийПредмет" {
		t.Errorf("unknown subject should be returned as-is, got %q", unknown)
	}
}
