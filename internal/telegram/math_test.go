package telegram

import (
	"math"
	"strings"
	"testing"
)

func testLoc(key string) string {
	switch key {
	case "cmdtext_27":
		return "empty expression"
	case "cmdtext_28":
		return "missing closing parenthesis"
	case "cmdtext_29":
		return "unexpected character %c"
	case "calls_parse_saturday":
		return "суббот"
	case "calls_parse_weekday":
		return "будн"
	}
	return key
}

func TestEvalMathFollowsTheUsualPrecedence(t *testing.T) {
	cases := map[string]float64{
		"2+2":       4,
		"2+3*4":     14,
		"(2+3)*4":   20,
		"2^3":       8,
		"2**3":      8,
		"10/4":      2.5,
		"1e2":       100,
		"2*-3":      -6,
		"-2^2":      -4,
		"2+3-4":     1,
		"((((1))))": 1,
		"1/3*3":     1,
		"0.1+0.2":   0.30000000000000004,
		"1000000*2": 2000000,
		"2^2^3":     64,
		"-(1+2)":    -3,
		"12/3/2":    2,
	}
	for expr, want := range cases {
		got, err := evalMath(expr, testLoc)
		if err != nil {
			t.Errorf("evalMath(%q) failed: %v", expr, err)
			continue
		}
		if got != want {
			t.Errorf("evalMath(%q) = %v, want %v", expr, got, want)
		}
	}
}

func TestEvalMathReportsBrokenExpressions(t *testing.T) {
	cases := map[string]string{
		"":     "empty expression",
		"2+":   "empty expression",
		"(2":   "missing closing parenthesis",
		"2)":   "unexpected character",
		"a":    "unexpected character",
		"2^^3": "unexpected character",
	}
	for expr, want := range cases {
		_, err := evalMath(expr, testLoc)
		if err == nil {
			t.Errorf("evalMath(%q) must fail", expr)
			continue
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("evalMath(%q) = %v, want an error about %q", expr, err, want)
		}
	}

	result, err := evalMath("2/0", testLoc)
	if err != nil {
		t.Fatalf("division by zero is left to the caller: %v", err)
	}
	if !math.IsInf(result, 1) {
		t.Fatalf("2/0 = %v, want +Inf", result)
	}
}

func TestMathExpressionLooksValid(t *testing.T) {
	valid := []string{"2+2", "(2+2)*3", "1", "0.5", "(1)"}
	for _, expr := range valid {
		if !mathExpressionLooksValid(expr) {
			t.Errorf("%q must look valid", expr)
		}
	}
	invalid := []string{"", "(2", "2)", "+2", "2+"}
	for _, expr := range invalid {
		if mathExpressionLooksValid(expr) {
			t.Errorf("%q must look invalid", expr)
		}
	}
}

func TestFormatMathResultKeepsWholeNumbersWhole(t *testing.T) {
	cases := map[float64]string{
		4:     "4",
		2.5:   "2.5",
		-3:    "-3",
		1e22:  "1e+22",
		0.001: "0.001",
	}
	for value, want := range cases {
		if got := formatMathResult(value); got != want {
			t.Errorf("formatMathResult(%v) = %q, want %q", value, got, want)
		}
	}
}
