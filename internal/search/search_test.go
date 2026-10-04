package search

import (
	"reflect"
	"testing"
)

var teachersFixture = []string{"Иванов И.И.", "Петров П.П.", "Сидоров С.С."}

var groupsFixture = []string{"Т-123", "ИТ-456", "А-789"}

func TestSearchTeachersExactMatchScoresThree(t *testing.T) {
	got := SearchTeachers("Иванов И.И.", teachersFixture, nil, 5)
	if len(got) == 0 || got[0].Name != "Иванов И.И." || got[0].Score != 3 {
		t.Fatalf("results = %+v, want Иванов И.И. at 3", got)
	}
}

func TestSearchTeachersFindsLatinTranslit(t *testing.T) {
	got := SearchTeachers("ivanov", teachersFixture, nil, 5)
	if len(got) == 0 || got[0].Name != "Иванов И.И." || got[0].Score != 2 {
		t.Fatalf("results = %+v, want Иванов И.И. at 2", got)
	}
	full := SearchTeachers("ivanov i.i.", teachersFixture, nil, 5)
	if len(full) == 0 || full[0].Name != "Иванов И.И." || full[0].Score != 3 {
		t.Fatalf("results = %+v, want Иванов И.И. at 3", full)
	}
}

func TestSearchTeachersFindsTransliteratedTypo(t *testing.T) {
	got := SearchTeachers("ivnaov", teachersFixture, nil, 5)
	if len(got) == 0 || got[0].Name != "Иванов И.И." {
		t.Fatalf("results = %+v, want Иванов И.И. first", got)
	}
}

func TestSearchTeachersFindsCyrillicTypo(t *testing.T) {
	got := SearchTeachers("ивнаов", teachersFixture, nil, 5)
	if len(got) == 0 || got[0].Name != "Иванов И.И." {
		t.Fatalf("results = %+v, want Иванов И.И. first", got)
	}
}

func TestSearchTeachersPrefixScoresTwo(t *testing.T) {
	got := SearchTeachers("иван", teachersFixture, nil, 5)
	if len(got) == 0 || got[0].Name != "Иванов И.И." || got[0].Score != 2 {
		t.Fatalf("results = %+v, want Иванов И.И. at 2", got)
	}
}

func TestSearchTeachersSubstringScoresOneAndHalf(t *testing.T) {
	got := SearchTeachers("ванов", teachersFixture, nil, 5)
	if len(got) == 0 || got[0].Name != "Иванов И.И." || got[0].Score != 1.5 {
		t.Fatalf("results = %+v, want Иванов И.И. at 1.5", got)
	}
}

func TestSearchTeachersTokenPrefixMatchesInitials(t *testing.T) {
	got := SearchTeachers("и.и.", teachersFixture, nil, 5)
	if len(got) == 0 || got[0].Name != "Иванов И.И." || got[0].Score != 1.5 {
		t.Fatalf("results = %+v, want Иванов И.И. at 1.5", got)
	}
}

func TestSearchGroupsIgnoresCaseSpaceAndDash(t *testing.T) {
	for _, query := range []string{"т-123", "Т-123", "т 123", "Т 123", "т—123"} {
		got := SearchGroups(query, groupsFixture, nil, 5)
		if len(got) == 0 || got[0].Name != "Т-123" {
			t.Fatalf("query %q results = %+v, want Т-123 first", query, got)
		}
	}
	got := SearchGroups("т 123", groupsFixture, nil, 5)
	if got[0].Score != 2.5 {
		t.Fatalf("results = %+v, want compact exact at 2.5", got)
	}
}

func TestSearchGroupsFindsLatinTranslit(t *testing.T) {
	got := SearchGroups("it-456", []string{"ИТ-456", "Т-123"}, nil, 5)
	if len(got) == 0 || got[0].Name != "ИТ-456" {
		t.Fatalf("results = %+v, want ИТ-456 first", got)
	}
}

func TestSearchResolvesChatAlias(t *testing.T) {
	aliases := []Alias{{Key: "любимая группа", Value: "Т-123"}}
	got := SearchGroups("любимая группа", groupsFixture, aliases, 5)
	if len(got) == 0 || got[0].Name != "Т-123" || got[0].ViaAlias != "любимая группа" {
		t.Fatalf("results = %+v, want Т-123 via alias", got)
	}
}

func TestSearchAliasKeyPrefixStillResolves(t *testing.T) {
	aliases := []Alias{{Key: "вечерняя", Value: "Т-123"}}
	got := SearchGroups("вечерн", groupsFixture, aliases, 5)
	found := false
	for _, r := range got {
		if r.Name == "Т-123" && r.ViaAlias == "вечерняя" {
			found = true
		}
	}
	if !found {
		t.Fatalf("results = %+v, want Т-123 via вечерняя", got)
	}
}

func TestSearchSkipsBlankNamesAndAliases(t *testing.T) {
	got := SearchGroups("т-123", []string{"", "   ", "Т-123"}, []Alias{{Key: "", Value: "Т-123"}, {Key: "x", Value: ""}, {Key: "   ", Value: "   "}}, 5)
	if len(got) != 1 || got[0].Name != "Т-123" {
		t.Fatalf("results = %+v, want only Т-123", got)
	}
}

func TestSearchIgnoresUnmatchedAlias(t *testing.T) {
	aliases := []Alias{{Key: "чужой", Value: "Неизвестная"}}
	got := SearchGroups("т-123", groupsFixture, aliases, 0)
	for _, r := range got {
		if r.Name == "Неизвестная" {
			t.Fatalf("results = %+v, want no unmatched alias", got)
		}
	}
}

func TestSearchResultsAreDeterministic(t *testing.T) {
	aliases := []Alias{{Key: "мой препод", Value: "Петров П.П."}}
	first := SearchTeachers("ов", teachersFixture, aliases, 0)
	second := SearchTeachers("ов", teachersFixture, aliases, 0)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("first = %+v, second = %+v, want identical", first, second)
	}
	for i := 1; i < len(first); i++ {
		if first[i-1].Score < first[i].Score {
			t.Fatalf("results = %+v, want score order", first)
		}
		if first[i-1].Score == first[i].Score && first[i-1].Name > first[i].Name {
			t.Fatalf("results = %+v, want name order on ties", first)
		}
	}
}

func TestSearchTiesBreakByName(t *testing.T) {
	got := SearchTeachers("и", []string{"Би", "Аи"}, nil, 0)
	if len(got) != 2 || got[0].Name != "Аи" || got[1].Name != "Би" {
		t.Fatalf("results = %+v, want Аи before Би", got)
	}
}

func TestSearchLimitTruncatesTopResults(t *testing.T) {
	got := SearchTeachers("ов", teachersFixture, nil, 1)
	if len(got) != 1 {
		t.Fatalf("results = %+v, want exactly one", got)
	}
	full := SearchTeachers("ов", teachersFixture, nil, 0)
	if len(full) == 0 || got[0] != full[0] {
		t.Fatalf("limited = %+v, full = %+v, want top result", got, full)
	}
}

func TestSearchEmptyQueryReturnsNothing(t *testing.T) {
	for _, q := range []string{"", "   "} {
		if got := SearchGroups(q, groupsFixture, nil, 5); len(got) != 0 {
			t.Errorf("query %q results = %+v, want none", q, got)
		}
		if got := SearchTeachers(q, teachersFixture, nil, 5); len(got) != 0 {
			t.Errorf("query %q results = %+v, want none", q, got)
		}
	}
}

func TestSearchGibberishReturnsNothing(t *testing.T) {
	if got := SearchTeachers("zzzqqqw", teachersFixture, nil, 5); len(got) != 0 {
		t.Errorf("results = %+v, want none", got)
	}
	if got := SearchGroups("zzzqqqw", groupsFixture, nil, 5); len(got) != 0 {
		t.Errorf("results = %+v, want none", got)
	}
	if got := SearchTeachers("---", teachersFixture, nil, 5); len(got) != 0 {
		t.Errorf("results = %+v, want none", got)
	}
}

func TestSearchDegenerateSignsStaySilent(t *testing.T) {
	if got := SearchTeachers("ъ", []string{"ь", "ъ test"}, nil, 5); len(got) != 0 {
		for _, r := range got {
			if r.Name == "ь" {
				t.Fatalf("results = %+v, want no empty-form match", got)
			}
		}
	}
}
