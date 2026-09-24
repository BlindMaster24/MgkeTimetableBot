package cache

import "testing"

func TestMetaAccessors(t *testing.T) {
	c, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c.SetGroups(map[string]any{"100": map[string]any{"days": []any{}}}, "groups-hash")
	c.SetTeachers(map[string]any{"T": map[string]any{"days": []any{}}}, "teachers-hash")
	c.SetTeam(map[string]string{"A": "A Full"}, []string{"team-hash"})

	groups := c.GetGroupsMeta()
	if groups.Hash != "groups-hash" || groups.Update == 0 {
		t.Errorf("groups meta = %+v", groups)
	}
	teachers := c.GetTeachersMeta()
	if teachers.Hash != "teachers-hash" || teachers.Update == 0 {
		t.Errorf("teachers meta = %+v", teachers)
	}
	team := c.GetTeamMeta()
	if len(team.Hash) != 1 || team.Hash[0] != "team-hash" {
		t.Errorf("team meta = %+v", team)
	}

	if !c.LastSuccess() {
		t.Error("a fresh cache must report last success")
	}
	c.SetSuccessUpdate(false)
	if c.LastSuccess() {
		t.Error("LastSuccess must follow SetSuccessUpdate")
	}
}
