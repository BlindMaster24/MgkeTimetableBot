package telegram

import (
	"context"
	"reflect"
	"testing"
)

func lookupCandidates() (map[string]any, map[string]string) {
	candidates := map[string]any{
		"Иванов И.И.":  struct{}{},
		"Иванова А.А.": struct{}{},
		"Петров П.П.":  struct{}{},
		"Сидоров С.С.": struct{}{},
	}
	fullNames := map[string]string{
		"Иванов И.И.": "Иванов Иван Иванович",
		"Петров П.П.": "Петров Пётр Петрович",
	}
	return candidates, fullNames
}

func TestLookupTeacherTypoFindsIvanov(t *testing.T) {
	candidates, fullNames := lookupCandidates()
	for _, query := range []string{"ивнаов", "ivnaov"} {
		matched, tooMany := matchTeacherList(query, candidates, fullNames, nil)
		if tooMany || len(matched) == 0 || matched[0] != "Иванов И.И." {
			t.Fatalf("query %q matched = %v tooMany = %v, want Иванов И.И. first", query, matched, tooMany)
		}
	}
}

func TestLookupTeacherTranslitResolvesSingle(t *testing.T) {
	candidates, fullNames := lookupCandidates()
	matched, tooMany := matchTeacherList("ivanov i.i.", candidates, fullNames, nil)
	if tooMany || len(matched) != 1 || matched[0] != "Иванов И.И." {
		t.Fatalf("matched = %v tooMany = %v, want single Иванов И.И.", matched, tooMany)
	}
}

func TestLookupTeacherKeepsExactFirst(t *testing.T) {
	candidates, fullNames := lookupCandidates()
	matched, tooMany := matchTeacherList("иванов", candidates, fullNames, nil)
	if tooMany || len(matched) < 2 || matched[0] != "Иванов И.И." {
		t.Fatalf("matched = %v tooMany = %v, want Иванов И.И. first", matched, tooMany)
	}
	first, _ := matchTeacherList("иванов", candidates, fullNames, nil)
	second, _ := matchTeacherList("иванов", candidates, fullNames, nil)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("first = %v second = %v, want identical order", first, second)
	}
}

func TestLookupTeacherResolvesAlias(t *testing.T) {
	b, _ := setupE2EBot(t)
	userID := int64(77)
	if err := b.aliasRepo.Add(userID, "мой препод", "Иванов И.И."); err != nil {
		t.Fatal(err)
	}
	candidates, fullNames := lookupCandidates()
	matched, tooMany := matchTeacherList("мой препод", candidates, fullNames, b.searchAliases(userID))
	if tooMany || len(matched) != 1 || matched[0] != "Иванов И.И." {
		t.Fatalf("matched = %v tooMany = %v, want single Иванов И.И.", matched, tooMany)
	}
}

func TestLookupGroupExactStaysFirst(t *testing.T) {
	b, _ := setupE2EBot(t)
	if group, ok := b.lookupGroup("100", 78); !ok || group != "100" {
		t.Fatalf("group = %q %v, want 100 true", group, ok)
	}
	if _, ok := b.lookupGroup("несуществующая", 78); ok {
		t.Error("gibberish must not resolve")
	}
}

func TestLookupGroupResolvesAlias(t *testing.T) {
	b, _ := setupE2EBot(t)
	userID := int64(79)
	if err := b.aliasRepo.Add(userID, "сотка", "100"); err != nil {
		t.Fatal(err)
	}
	if group, ok := b.lookupGroup("сотка", userID); !ok || group != "100" {
		t.Fatalf("group = %q %v, want 100 true", group, ok)
	}
}

func TestLookupGroupIgnoresFuzzyDigits(t *testing.T) {
	b, _ := setupE2EBot(t)
	if _, ok := b.lookupGroup("101", 78); ok {
		t.Error("unknown number 101 must not resolve to 100")
	}
	if group, ok := b.lookupGroup("1 00", 78); !ok || group != "100" {
		t.Fatalf("group = %q %v, want 100 true", group, ok)
	}
}

func TestSetupSceneAcceptsGroupTypo(t *testing.T) {
	b, repo := setupE2EBot(t)
	userID := int64(80)
	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Scene = sceneSetGroup
	repo.Save(chat)

	u := &Update{ChatID: userID, UserID: userID, Text: "1 00"}
	u.Bot = b
	if !b.dispatchInputScene(context.Background(), u, chat) {
		t.Fatal("set_group scene must dispatch")
	}
	loaded, _ := repo.FindOrCreate("telegram", userID)
	if loaded.Group != "100" || loaded.Scene != "" {
		t.Fatalf("chat = %+v, want group 100 with cleared scene", loaded)
	}
}

func TestSetupSceneAcceptsTeacherTranslitTypo(t *testing.T) {
	b, repo := setupE2EBot(t)
	userID := int64(81)
	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Scene = sceneSetTeacher
	repo.Save(chat)

	u := &Update{ChatID: userID, UserID: userID, Text: "ivnaov"}
	u.Bot = b
	if !b.dispatchInputScene(context.Background(), u, chat) {
		t.Fatal("set_teacher scene must dispatch")
	}
	loaded, _ := repo.FindOrCreate("telegram", userID)
	if loaded.Teacher != "Иванов И.И." || loaded.Scene != "" {
		t.Fatalf("chat = %+v, want teacher Иванов И.И. with cleared scene", loaded)
	}
}
