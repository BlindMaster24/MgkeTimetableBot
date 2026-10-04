package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

type RaspEntry[T any] struct {
	Timetable     T      `json:"timetable"`
	Update        int64  `json:"update"`
	Changed       int64  `json:"changed"`
	LastWeekIndex int    `json:"lastWeekIndex"`
	Hash          string `json:"hash"`
}

type TeamCacheEntry struct {
	Names   map[string]string `json:"names"`
	Update  int64             `json:"update"`
	Changed int64             `json:"changed"`
	Hash    []string          `json:"hash"`
}

type CallsSource struct {
	Schedule     CallsSchedule `json:"schedule"`
	UpdatedAt    int64         `json:"updatedAt"`
	UpdatedAtRaw string        `json:"updatedAtRaw,omitempty"`
	Hash         string        `json:"hash"`
}

type CallsActive struct {
	Schedule     CallsSchedule `json:"schedule"`
	UpdatedAt    int64         `json:"updatedAt"`
	UpdatedAtRaw string        `json:"updatedAtRaw,omitempty"`
	Source       string        `json:"source"`
	Hash         string        `json:"hash"`
}

type CallsSchedule struct {
	Weekdays [][2][2]string `json:"weekdays"`
	Saturday [][2][2]string `json:"saturday"`
}

type CallsVariant struct {
	Name      string        `json:"name"`
	Schedule  CallsSchedule `json:"schedule"`
	UpdatedAt int64         `json:"updatedAt,omitempty"`
}

func (v CallsVariant) Slots() int {
	if len(v.Schedule.Saturday) > len(v.Schedule.Weekdays) {
		return len(v.Schedule.Saturday)
	}
	return len(v.Schedule.Weekdays)
}

type CallsCache struct {
	Site                CallsSource    `json:"site"`
	SiteVariants        []CallsVariant `json:"siteVariants,omitempty"`
	Manual              CallsSource    `json:"manual"`
	Active              CallsActive    `json:"active"`
	Update              int64          `json:"update"`
	Changed             int64          `json:"changed"`
	ManualReason        string         `json:"manualReason"`
	OverrideSource      string         `json:"overrideSource"`
	SiteEmptyNotifiedAt int64          `json:"siteEmptyNotifiedAt"`
}

type RaspCache struct {
	mu            sync.RWMutex
	saveMu        sync.Mutex
	dir           string
	Groups        *RaspEntry[GroupTimetable]   `json:"groups"`
	Teachers      *RaspEntry[TeacherTimetable] `json:"teachers"`
	Team          TeamCacheEntry               `json:"team"`
	Calls         CallsCache                   `json:"calls"`
	SuccessUpdate bool                         `json:"successUpdate"`

	callsPreferSite atomic.Bool

	groupsAny   map[string]any
	teachersAny map[string]any

	events     []Event
	dayChanges []DayChange

	hits   atomic.Int64
	misses atomic.Int64
}

type Stats struct {
	Hits           int64  `json:"hits"`
	Misses         int64  `json:"misses"`
	GroupsCount    int    `json:"groupsCount"`
	TeachersCount  int    `json:"teachersCount"`
	SuccessUpdate  bool   `json:"successUpdate"`
	GroupsUpdate   int64  `json:"groupsUpdate"`
	TeachersUpdate int64  `json:"teachersUpdate"`
	GroupsHash     string `json:"groupsHash"`
	TeachersHash   string `json:"teachersHash"`
}

func New(dir string) (*RaspCache, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}

	c := &RaspCache{
		dir: dir,
		Groups: &RaspEntry[GroupTimetable]{
			Timetable: make(GroupTimetable),
		},
		Teachers: &RaspEntry[TeacherTimetable]{
			Timetable: make(TeacherTimetable),
		},
		groupsAny:   make(map[string]any),
		teachersAny: make(map[string]any),
		Team: TeamCacheEntry{
			Names: make(map[string]string),
		},
		SuccessUpdate: true,
	}

	c.callsPreferSite.Store(true)
	c.load()
	return c, nil
}

func (c *RaspCache) GetGroups() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.groupsAny
}

func (c *RaspCache) GetTeachers() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.teachersAny
}

func (c *RaspCache) GetTeamNames() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Team.Names
}

func (c *RaspCache) GetCalls() CallsCache {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Calls
}

func (c *RaspCache) CallsDue(now time.Time, interval time.Duration) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return dueAt(now, c.Calls.Update, interval)
}

func (c *RaspCache) TeamDue(now time.Time, interval time.Duration) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return dueAt(now, c.Team.Update, interval)
}

func dueAt(now time.Time, updatedAt int64, interval time.Duration) bool {
	if interval <= 0 {
		return true
	}
	if updatedAt <= 0 {
		return true
	}
	return now.Sub(time.UnixMilli(updatedAt)) >= interval
}

func (c *RaspCache) GetGroupsChangedTime() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.UnixMilli(c.Groups.Changed)
}

func (c *RaspCache) GetTeachersChangedTime() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.UnixMilli(c.Teachers.Changed)
}

func (c *RaspCache) GetTeamUpdateTime() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.UnixMilli(c.Team.Update)
}

func (c *RaspCache) GetTeamChangedTime() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.UnixMilli(c.Team.Changed)
}

func (c *RaspCache) GetGroupsUpdateTime() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.UnixMilli(c.Groups.Update)
}

func (c *RaspCache) GetTeachersUpdateTime() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.UnixMilli(c.Teachers.Update)
}

func (c *RaspCache) SetGroups(ctx context.Context, groups map[string]any, hash string) {
	if err := ctx.Err(); err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixMilli()
	merged, maxWeek, changed := c.setTimetable(KindGroups, c.groupsAny, groups, c.Groups.LastWeekIndex)
	c.groupsAny = merged
	c.Groups.Timetable = GroupTimetableFromAny(merged)
	c.Groups.Update = now
	c.Groups.Hash = hash
	if changed {
		c.Groups.Changed = now
	}
	c.Groups.LastWeekIndex = maxWeek
}

func (c *RaspCache) SetTeachers(ctx context.Context, teachers map[string]any, hash string) {
	if err := ctx.Err(); err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixMilli()
	merged, maxWeek, changed := c.setTimetable(KindTeachers, c.teachersAny, teachers, c.Teachers.LastWeekIndex)
	c.teachersAny = merged
	c.Teachers.Timetable = TeacherTimetableFromAny(merged)
	c.Teachers.Update = now
	c.Teachers.Hash = hash
	if changed {
		c.Teachers.Changed = now
	}
	c.Teachers.LastWeekIndex = maxWeek
}

func (c *RaspCache) setTimetable(kind string, old map[string]any, data map[string]any, previousWeekIndex int) (map[string]any, int, bool) {
	todayIdx := utils.DayIndexFromDate(time.Now())

	var newEvents []Event
	var newChanges []DayChange
	for value, v := range data {
		vm, ok := v.(map[string]any)
		if !ok {
			continue
		}
		var oldEntryMap map[string]any
		if ov, ok := old[value]; ok {
			oldEntryMap, _ = ov.(map[string]any)
		}
		events, changes := collectEntryDayEvents(kind, value, oldEntryMap, vm, todayIdx)
		for _, out := range events {
			newEvents = append(newEvents, out.ev)
		}
		newChanges = append(newChanges, changes...)
	}

	weekEvents, maxWeek := collectWeekEvents(kind, previousWeekIndex, data)
	newEvents = append(newEvents, weekEvents...)

	var withdrawnEntries []string
	previousWeekIsFuture := previousWeekIndex > 0 && utils.WeekIndexFromNumber(previousWeekIndex).IsFutureWeek()
	if previousWeekIsFuture {
		previousEntries := collectWeekEntries(old, previousWeekIndex)
		currentEntries := collectWeekEntries(data, previousWeekIndex)
		prevSet := make(map[string]bool, len(currentEntries))
		for _, e := range currentEntries {
			prevSet[e] = true
		}
		for _, e := range previousEntries {
			if !prevSet[e] {
				withdrawnEntries = append(withdrawnEntries, e)
			}
		}
		if len(withdrawnEntries) > 0 {
			newEvents = append(newEvents, Event{Week: &WeekEvent{Kind: kind, Week: previousWeekIndex, Withdrawn: true, Entries: withdrawnEntries}})
		}
	}

	mergedTimetable := make(map[string]any, len(old)+len(data))
	for k, v := range old {
		om, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if _, exists := data[k]; !exists {
			md, _, _ := mergeDays(nil, entryDays(v))
			cp := make(map[string]any, len(om)+1)
			for fk, fv := range om {
				cp[fk] = fv
			}
			cp["days"] = md
			mergedTimetable[k] = cp
		}
	}
	for k, v := range data {
		var oldEntryMap map[string]any
		if ov, ok := old[k]; ok {
			oldEntryMap, _ = ov.(map[string]any)
		}
		if oldEntryMap != nil {
			lastNoticed := entryLastNoticedDayFromMap(oldEntryMap)
			if lastNoticed > 0 {
				if m, ok := v.(map[string]any); ok {
					m["lastNoticedDay"] = float64(lastNoticed)
				}
			}
		}
		mergedTimetable[k] = v
	}

	changed := !mapsEqual(old, mergedTimetable)

	c.events = append(c.events, newEvents...)
	c.dayChanges = append(c.dayChanges, newChanges...)

	return mergedTimetable, maxWeek, changed
}

func (c *RaspCache) SetTeam(ctx context.Context, names map[string]string, hashes []string) {
	if err := ctx.Err(); err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Team.Names = names
	c.Team.Update = time.Now().UnixMilli()
	c.Team.Hash = hashes
}

func (c *RaspCache) SetCalls(ctx context.Context, site Schedule, manual Schedule, source string) {
	c.setCallsInternal(ctx, site, manual, source, "", false)
}

func (c *RaspCache) SetCallsNotify(ctx context.Context, site Schedule, manual Schedule, source string, reason string) {
	c.setCallsInternal(ctx, site, manual, source, reason, true)
}

func (c *RaspCache) setCallsInternal(ctx context.Context, site Schedule, manual Schedule, source, reason string, skipNotify bool) {
	if err := ctx.Err(); err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixMilli()
	if source == "site" {
		c.Calls.ManualReason = ""
	}

	c.Calls.Site = CallsSource{
		Schedule:  CallsSchedule{Weekdays: site.Weekdays, Saturday: site.Saturday},
		UpdatedAt: now,
		Hash:      hashSchedule(site),
	}
	if len(manual.Weekdays) > 0 || len(manual.Saturday) > 0 {
		c.Calls.Manual = CallsSource{
			Schedule:  CallsSchedule{Weekdays: manual.Weekdays, Saturday: manual.Saturday},
			UpdatedAt: now,
			Hash:      hashSchedule(manual),
		}
	}
	c.Calls.Update = now

	c.selectActiveCallsLocked(skipNotify, reason)
}

func (c *RaspCache) SetCallsSiteUpdatedAt(ctx context.Context, raw string, at int64) {
	if err := ctx.Err(); err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if raw == "" {
		return
	}
	c.Calls.Site.UpdatedAtRaw = raw
	if at > 0 {
		c.Calls.Site.UpdatedAt = at
	}
	c.Calls.Active.UpdatedAtRaw = raw
	if at > 0 {
		c.Calls.Active.UpdatedAt = at
	}
}

func (c *RaspCache) SetCallsSiteVariants(ctx context.Context, variants []CallsVariant) {
	if err := ctx.Err(); err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Calls.SiteVariants = variants
}

func (c *RaspCache) GetCallsVariant(name string) (CallsSchedule, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, variant := range c.Calls.SiteVariants {
		if variant.Name == name {
			return variant.Schedule, true
		}
	}
	return CallsSchedule{}, false
}

func (c *RaspCache) SetCallsPreferSite(v bool) {
	c.callsPreferSite.Store(v)
}

func (c *RaspCache) selectActiveCallsLocked(skipNotify bool, reason string) {
	preferSite := c.callsPreferSite.Load()
	configSchedule := CallsSchedule{}

	activeSource := "config"
	activeSchedule := configSchedule
	activeUpdatedAt := time.Now().UnixMilli()
	activeUpdatedAtRaw := ""

	switch c.Calls.OverrideSource {
	case "site":
		if c.Calls.Site.UpdatedAt > 0 {
			activeSource = "site"
			activeSchedule = c.Calls.Site.Schedule
			activeUpdatedAt = c.Calls.Site.UpdatedAt
			activeUpdatedAtRaw = c.Calls.Site.UpdatedAtRaw
		}
	case "manual":
		if c.Calls.Manual.UpdatedAt > 0 {
			activeSource = "manual"
			activeSchedule = c.Calls.Manual.Schedule
			activeUpdatedAt = c.Calls.Manual.UpdatedAt
		}
	case "config":
		activeSource = "config"
	default:
		if preferSite && c.Calls.Site.UpdatedAt > 0 {
			activeSource = "site"
			activeSchedule = c.Calls.Site.Schedule
			activeUpdatedAt = c.Calls.Site.UpdatedAt
			activeUpdatedAtRaw = c.Calls.Site.UpdatedAtRaw
		}
		if c.Calls.Manual.UpdatedAt > 0 {
			manualIsNewer := c.Calls.Manual.UpdatedAt >= c.Calls.Site.UpdatedAt
			if !preferSite || manualIsNewer {
				activeSource = "manual"
				activeSchedule = c.Calls.Manual.Schedule
				activeUpdatedAt = c.Calls.Manual.UpdatedAt
				activeUpdatedAtRaw = ""
			}
		}
	}

	activeHash := hashSchedule(Schedule{Weekdays: activeSchedule.Weekdays, Saturday: activeSchedule.Saturday})
	sourceChanged := c.Calls.Active.Source != activeSource
	updatedChanged := c.Calls.Active.UpdatedAt != activeUpdatedAt
	if c.Calls.Active.Hash != "" && c.Calls.Active.Hash == activeHash && !sourceChanged && !updatedChanged {
		return
	}

	weekdaysChanged := schedulesNotEqual(c.Calls.Active.Schedule.Weekdays, activeSchedule.Weekdays)
	saturdayChanged := schedulesNotEqual(c.Calls.Active.Schedule.Saturday, activeSchedule.Saturday)

	c.Calls.Changed = time.Now().UnixMilli()
	c.Calls.Active = CallsActive{
		Schedule:     activeSchedule,
		UpdatedAt:    activeUpdatedAt,
		UpdatedAtRaw: activeUpdatedAtRaw,
		Source:       activeSource,
		Hash:         activeHash,
	}

	if skipNotify || (!weekdaysChanged && !saturdayChanged) {
		return
	}

	eventReason := reason
	if eventReason == "" && activeSource == "manual" {
		eventReason = c.Calls.ManualReason
	}

	c.events = append(c.events, Event{Calls: &CallsEvent{
		WeekdaysChanged: weekdaysChanged,
		SaturdayChanged: saturdayChanged,
		Reason:          eventReason,
		Schedule:        activeSchedule,
	}})
}

type Schedule struct {
	Weekdays [][2][2]string
	Saturday [][2][2]string
}

func (c *RaspCache) SetSuccessUpdate(ctx context.Context, ok bool) {
	if err := ctx.Err(); err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.SuccessUpdate = ok
}

func (c *RaspCache) Stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Stats{
		Hits:           c.hits.Load(),
		Misses:         c.misses.Load(),
		GroupsCount:    len(c.groupsAny),
		TeachersCount:  len(c.teachersAny),
		SuccessUpdate:  c.SuccessUpdate,
		GroupsUpdate:   c.Groups.Update,
		TeachersUpdate: c.Teachers.Update,
		GroupsHash:     c.Groups.Hash,
		TeachersHash:   c.Teachers.Hash,
	}
}

type EntryMeta struct {
	Update        int64  `json:"update"`
	Changed       int64  `json:"changed"`
	LastWeekIndex int    `json:"lastWeekIndex"`
	Hash          string `json:"hash"`
}

type TeamMeta struct {
	Update  int64    `json:"update"`
	Changed int64    `json:"changed"`
	Hash    []string `json:"hash"`
}

func entryMeta[T any](entry *RaspEntry[T]) EntryMeta {
	return EntryMeta{Update: entry.Update, Changed: entry.Changed, LastWeekIndex: entry.LastWeekIndex, Hash: entry.Hash}
}

func (c *RaspCache) GetGroupsMeta() EntryMeta {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return entryMeta(c.Groups)
}

func (c *RaspCache) GetTeachersMeta() EntryMeta {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return entryMeta(c.Teachers)
}

func (c *RaspCache) GetTeamMeta() TeamMeta {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return TeamMeta{Update: c.Team.Update, Changed: c.Team.Changed, Hash: c.Team.Hash}
}

func (c *RaspCache) LastSuccess() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.SuccessUpdate
}

func (c *RaspCache) RecordHit()  { c.hits.Add(1) }
func (c *RaspCache) RecordMiss() { c.misses.Add(1) }
func (c *RaspCache) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Groups = &RaspEntry[GroupTimetable]{Timetable: make(GroupTimetable)}
	c.Teachers = &RaspEntry[TeacherTimetable]{Timetable: make(TeacherTimetable)}
	c.groupsAny = make(map[string]any)
	c.teachersAny = make(map[string]any)
	c.Team = TeamCacheEntry{Names: make(map[string]string)}
	c.events = nil
	c.dayChanges = nil
}

func (c *RaspCache) Save(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.saveMu.Lock()
	defer c.saveMu.Unlock()
	c.mu.RLock()
	defer c.mu.RUnlock()

	type named struct {
		name string
		data any
	}

	entries := []named{
		{"groups.json", c.Groups},
		{"teachers.json", c.Teachers},
		{"team.json", c.Team},
		{"calls.json", c.Calls},
	}

	for _, e := range entries {
		path := filepath.Join(c.dir, e.name)
		data, err := json.MarshalIndent(e.data, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal %s: %w", e.name, err)
		}
		if err := writeFileAtomic(path, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", e.name, err)
		}
	}

	return nil
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func (c *RaspCache) load() {
	loadFile := func(name string, target any) {
		path := filepath.Join(c.dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		if err := json.Unmarshal(data, target); err != nil {
			os.Remove(path)
			return
		}
	}

	loadFile("groups.json", c.Groups)
	loadFile("teachers.json", c.Teachers)
	loadFile("team.json", &c.Team)
	loadFile("calls.json", &c.Calls)

	c.groupsAny = c.Groups.Timetable.AnyView()
	c.teachersAny = c.Teachers.Timetable.AnyView()
}

func mapsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		av, _ := json.Marshal(v)
		bvv, _ := json.Marshal(bv)
		if string(av) != string(bvv) {
			return false
		}
	}
	return true
}

func hashSchedule(s Schedule) string {
	data, _ := json.Marshal(s)
	h := fmt.Sprintf("%x", data)
	return h
}

func schedulesNotEqual(a, b [][2][2]string) bool {
	return !slicesEqual(a, b)
}

func slicesEqual(a, b [][2][2]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		for r := 0; r < 2; r++ {
			for cIdx := 0; cIdx < 2; cIdx++ {
				if a[i][r][cIdx] != b[i][r][cIdx] {
					return false
				}
			}
		}
	}
	return true
}

func (c *RaspCache) GetCallsWeekdays() [][2][2]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Calls.Active.Schedule.Weekdays
}

func (c *RaspCache) GetCallsSaturday() [][2][2]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Calls.Active.Schedule.Saturday
}

func (c *RaspCache) SetCallsOverride(source string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Calls.OverrideSource = source
	c.selectActiveCallsLocked(true, "")
}

func (c *RaspCache) ResetCallsOverride() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Calls.OverrideSource = ""
	c.selectActiveCallsLocked(true, "")
}

func (c *RaspCache) SetCallsFromCache(calls CallsCache) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Calls = calls
}

func (c *RaspCache) SetCallsManual(weekdays, saturday [][2][2]string, reason string) {
	c.SetCallsManualNotify(weekdays, saturday, reason, true)
}

func (c *RaspCache) SetCallsManualNotify(weekdays, saturday [][2][2]string, reason string, notify bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixMilli()
	c.Calls.Manual = CallsSource{
		Schedule:  CallsSchedule{Weekdays: weekdays, Saturday: saturday},
		UpdatedAt: now,
		Hash:      hashSchedule(Schedule{Weekdays: weekdays, Saturday: saturday}),
	}
	c.Calls.ManualReason = reason
	c.Calls.OverrideSource = "manual"
	c.selectActiveCallsLocked(!notify, reason)
}
