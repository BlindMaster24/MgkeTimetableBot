package telegram

import (
	"context"
	"math"
	"strings"
	"testing"
)

func runMath(t *testing.T, b *Bot, userID int64, text string) {
	t.Helper()

	u := makeUpdate(userID, text)
	u.Bot = b
	if err := (&mathCmd{bot: b}).Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}
}

func TestMathKeepsTheOldAnswers(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")

	cases := []struct {
		text string
		want string
	}{
		{"/math", "/math <some>\n<some> - математический пример"},
		{"/math 2+2", "4"},
		{"/math (100+50)*2", "300"},
		{"/math 2^10", "1024"},
		{"/math 2×3", "6"},
		{"/math 10:2", "5"},
		{"/math 10÷4", "2.5"},
		{"/math 1к+2к", "3000"},
		{"/math 1,5+1,5", "3"},
		{"/math 2*2*", "Пример записан неправильно"},
		{"/math (2+2", "Пример записан неправильно"},
		{"/math (2)", "Пример записан неправильно"},
		{"/math (2+2)", "4"},
		{"/math +2", "Пример записан неправильно"},
		{"/math 2a", "В примере имеются лишние символы.\nСписок разрешённых: 0123456789+-*/():^×÷ekк,."},
		{"/math 1/0", "бесконечность"},
	}

	for _, c := range cases {
		caller.reset()
		runMath(t, b, userID, c.text)
		if got := caller.last(); got != c.want {
			t.Errorf("%q → %q, want %q", c.text, got, c.want)
		}
	}
}

func TestMathEvaluatesFractionsAndPowers(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")

	cases := []struct {
		text string
		want string
	}{
		{"/math 2+3*4", "14"},
		{"/math 2*(3+4)", "14"},
		{"/math 1e3+1", "1001"},
		{"/math 2**3", "8"},
	}

	for _, c := range cases {
		caller.reset()
		runMath(t, b, userID, c.text)
		if got := caller.last(); got != c.want {
			t.Errorf("%q → %q, want %q", c.text, got, c.want)
		}
	}

	caller.reset()
	runMath(t, b, userID, "/math 10/3")
	if got := caller.last(); !strings.HasPrefix(got, "3.3333333333333") {
		t.Errorf("10/3 → %q", got)
	}
}

func TestMathWhitespaceOnlyInput(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")

	caller.reset()
	runMath(t, b, userID, "/math ")
	if got := caller.last(); got != "/math <some>\n<some> - математический пример" {
		t.Errorf("single trailing space → %q", got)
	}

	caller.reset()
	runMath(t, b, userID, "/math   ")
	if got := caller.last(); got != "Пример записан неправильно" {
		t.Errorf("multiple trailing spaces → %q", got)
	}

	caller.reset()
	runMath(t, b, userID, "/math 0/0")
	if got := caller.last(); got != "NaN" {
		t.Errorf("0/0 → %q", got)
	}
}

func TestFormatMathResultMatchesJavaScript(t *testing.T) {
	cases := []struct {
		value float64
		want  string
	}{
		{4, "4"},
		{1000000, "1000000"},
		{-0.5, "-0.5"},
		{math.Inf(1), "+Inf"},
		{math.NaN(), "NaN"},
	}

	for _, c := range cases {
		if got := formatMathResult(c.value); got != c.want {
			t.Errorf("%v → %q, want %q", c.value, got, c.want)
		}
	}
}

func TestMathExpressionValidity(t *testing.T) {
	cases := []struct {
		expression string
		want       bool
	}{
		{"2+2", true},
		{"(2+2)", true},
		{"(2)", false},
		{"2+2)", false},
		{"+2", false},
		{"2*", false},
		{"", false},
	}

	for _, c := range cases {
		if got := mathExpressionLooksValid(c.expression); got != c.want {
			t.Errorf("%q → %v, want %v", c.expression, got, c.want)
		}
	}
}
