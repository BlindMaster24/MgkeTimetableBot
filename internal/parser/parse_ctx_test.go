package parser

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunContextCancelledStopsTheParse(t *testing.T) {
	doc := fuzzDocument(t, `<html><body>
		<h2>Группа - 100</h2>
		<table><tr><th>№</th><th>Понедельник, 07.09.2026</th></tr></table>
	</body></html>`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewGroupParser(doc).RunContext(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("RunContext() = %v, want context.Canceled", err)
	}
	if _, err := NewTeacherParser(doc).RunContext(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("RunContext() = %v, want context.Canceled", err)
	}
}

func TestCallsVariantsCtxCancelledParsesNothing(t *testing.T) {
	doc := fuzzDocument(t, `<html><body>
		<h1>Расписание звонков</h1>
		<table><tr><td>1 пара</td><td>8.00 – 8.45<br>8.55 – 9.40</td></tr></table>
	</body></html>`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	variants, _ := ParseCallsVariantsCtx(ctx, doc)
	if len(variants) != 0 {
		t.Errorf("variants = %d, want none for a cancelled parse", len(variants))
	}
}

func TestTeamReportCtxCancelledKeepsTheInput(t *testing.T) {
	doc := fuzzDocument(t, `<html><body>
		<div class="employee-card"><h5 class="employee-card-title">Иванов Иван Иванович</h5></div>
	</body></html>`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	team, _ := ParseTeamReportCtx(ctx, doc, map[string]string{"Петров П.П.": "Петров Петр Петрович"})
	if len(team) != 1 || team["Петров П.П."] == "" {
		t.Errorf("team = %v, want the input kept", team)
	}
}

func TestFetchDocumentCtxCancelledSkipsTheNetwork(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := FetchDocumentCtx(ctx, srv.Client(), srv.URL); !errors.Is(err, context.Canceled) {
		t.Errorf("FetchDocumentCtx() = %v, want context.Canceled", err)
	}
	if calls != 0 {
		t.Errorf("server saw %d requests, want none", calls)
	}
}
