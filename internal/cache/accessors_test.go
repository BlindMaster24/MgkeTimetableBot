package cache

import (
	"context"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func TestDueGettersFollowTheStoredMoment(t *testing.T) {
	c := mergedCache(t)
	now := utils.SiteNow()

	if !c.TeamDue(now, time.Hour) {
		t.Fatal("a team that was never loaded is due")
	}
	c.SetTeam(t.Context(), map[string]string{"Иванов": "x"}, nil)
	if c.TeamDue(now, time.Hour) {
		t.Fatal("a team loaded at this moment is not due")
	}
	if !c.TeamDue(now.Add(2*time.Hour), time.Hour) {
		t.Fatal("a team loaded two hours ago is due in an hour")
	}
}

func TestChangeAndUpdateStampsReadBack(t *testing.T) {
	c := mergedCache(t)
	today := utils.SiteNow().Format("02.01.2006")

	c.SetGroups(t.Context(), map[string]any{"100": weekEntry(today, map[string]any{"lesson": "Математика"})}, "h1")
	if got := c.GetGroupsUpdateTime(); got.UnixMilli() <= 0 {
		t.Fatal("the update stamp must be set when the dump is stored")
	}
	if got := c.GetGroupsChangedTime(); got.UnixMilli() <= 0 {
		t.Fatal("the change stamp must be set for the first dump")
	}

	c.SetTeachers(t.Context(), map[string]any{"Иванов": weekEntry(today, map[string]any{"lesson": "Физика"})}, "h1")
	if got := c.GetTeachersUpdateTime(); got.UnixMilli() <= 0 {
		t.Fatal("the teacher update stamp must be set when the dump is stored")
	}

	c.SetTeam(t.Context(), map[string]string{"Иванов": "x"}, nil)
	if got := c.GetTeamUpdateTime(); got.UnixMilli() <= 0 {
		t.Fatal("the team update stamp must be set when the names are stored")
	}

	reloaded, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.GetGroupsUpdateTime(); got.UnixMilli() != 0 {
		t.Fatalf("a fresh cache has no update stamp, got %d", got.UnixMilli())
	}
}

func TestSetCallsSiteUpdatedAtIgnoresACancelledContext(t *testing.T) {
	c := mergedCache(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c.SetCallsSiteUpdatedAt(ctx, "07.09.2026", 123)
	if got := c.GetCalls().Site.UpdatedAtRaw; got == "07.09.2026" {
		t.Fatalf("a cancelled context must not change the stamp, got %q", got)
	}

	c.SetCallsSiteUpdatedAt(context.Background(), "07.09.2026", 123)
	if got := c.GetCalls().Active.UpdatedAtRaw; got != "07.09.2026" {
		t.Fatalf("the active source must follow the site stamp, got %q", got)
	}
	c.SetCallsSiteUpdatedAt(context.Background(), "", 456)
	if got := c.GetCalls().Active.UpdatedAt; got != 123 {
		t.Fatalf("an empty stamp must be ignored, got %d", got)
	}
}
