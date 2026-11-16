package utils

import "testing"

func TestEscapeHTMLNeutralizesTheTelegramMarkup(t *testing.T) {
	cases := map[string]string{
		"Математика":            "Математика",
		"мат <b>жирный</b>":     "мат &lt;b&gt;жирный&lt;/b&gt;",
		"a & b":                 "a &amp; b",
		`кавычки "тут"`:         `кавычки &quot;тут&quot;`,
		"<script>&\"'</script>": "&lt;script&gt;&amp;&quot;'&lt;/script&gt;",
	}
	for input, want := range cases {
		if got := EscapeHTML(input); got != want {
			t.Fatalf("EscapeHTML(%q) = %q, want %q", input, got, want)
		}
	}
}
