package telegram

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func TestStatsCountsEverySubgroupEntryOnce(t *testing.T) {
	b, caller, repo, userID := setupArchiveBot(t, ModeStudent, "100", "")

	day := time.Now().AddDate(0, 0, -7)
	lessons := []any{
		[]any{
			map[string]any{"lesson": "Матем", "subgroup": float64(1)},
			map[string]any{"lesson": "Матем", "subgroup": float64(2)},
		},
	}
	data, err := json.Marshal(lessons)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.DB().Exec(
		`INSERT INTO timetable_archive (day, "group", data) VALUES (?, ?, ?)`,
		int64(utils.DayIndexFromDate(day)), "100", string(data),
	); err != nil {
		t.Fatal(err)
	}

	u := makeUpdate(userID, "/stats")
	u.Bot = b
	if err := (&statsCmd{bot: b}).Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}

	text := caller.last()
	if !strings.Contains(text, "1. Матем - 1 пар") || !strings.Contains(text, "2. Матем - 1 пар") {
		t.Fatalf("each subgroup must count once, got %q", text)
	}
	if !strings.Contains(text, "(2 предметов): 2") {
		t.Fatalf("the total must be 2 lessons, got %q", text)
	}
}
