package telegram

import "testing"

func TestRepositoryStateRoundTrip(t *testing.T) {
	repo, err := New(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	if _, ok, err := repo.LoadState("missing"); err != nil || ok {
		t.Fatalf("missing key = %v, %v", ok, err)
	}

	if err := repo.SaveState("health.tracker", `{"parserRuns":3}`); err != nil {
		t.Fatalf("save: %v", err)
	}
	value, ok, err := repo.LoadState("health.tracker")
	if err != nil || !ok {
		t.Fatalf("load = %q, %v, %v", value, ok, err)
	}
	if value != `{"parserRuns":3}` {
		t.Errorf("value = %q", value)
	}

	if err := repo.SaveState("health.tracker", `{"parserRuns":7}`); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	value, _, err = repo.LoadState("health.tracker")
	if err != nil {
		t.Fatal(err)
	}
	if value != `{"parserRuns":7}` {
		t.Errorf("overwritten value = %q", value)
	}
}

func TestRepositoryStateSurvivesReopen(t *testing.T) {
	path := t.TempDir() + "/test.db"

	repo, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveState("health.alerts", `{"active":{"parser_stale":"2026-01-01T00:00:00Z"}}`); err != nil {
		t.Fatalf("save: %v", err)
	}
	repo.Close()

	reopened, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	value, ok, err := reopened.LoadState("health.alerts")
	if err != nil || !ok {
		t.Fatalf("load after reopen = %q, %v, %v", value, ok, err)
	}
	if value == "" {
		t.Error("stored state should survive reopening the database")
	}
}
