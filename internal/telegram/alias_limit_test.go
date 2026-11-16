package telegram

import (
	"fmt"
	"testing"
)

func TestAliasLimitStopsGrowthButKeepsUpdates(t *testing.T) {
	repo, err := New(t.TempDir() + "/aliases.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })

	aliases := NewAliasRepository(repo)
	aliases.EnsureTable()

	for i := 0; i < MAX_ALIASES; i++ {
		if err := aliases.Add(7, fmt.Sprintf("k%02d", i), fmt.Sprintf("v%02d", i)); err != nil {
			t.Fatalf("alias %d: %v", i, err)
		}
	}

	if err := aliases.Add(7, "overflow", "value"); err != ErrAliasLimit {
		t.Fatalf("the alias over the limit must be refused, got %v", err)
	}
	if err := aliases.Add(7, "k00", "updated"); err != nil {
		t.Fatalf("updating an existing alias at the limit must work: %v", err)
	}

	list, err := aliases.List(7)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != MAX_ALIASES {
		t.Fatalf("alias count = %d, want %d", len(list), MAX_ALIASES)
	}
	if list[0].Value != "updated" {
		t.Fatalf("the updated alias = %+v", list[0])
	}
}
