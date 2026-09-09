package cache

import (
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
	Schedule  CallsSchedule `json:"schedule"`
	UpdatedAt int64         `json:"updatedAt"`
	Hash      string        `json:"hash"`
}

type CallsActive struct {
	Schedule  CallsSchedule `json:"schedule"`
	UpdatedAt int64         `json:"updatedAt"`
	Source    string        `json:"source"`
	Hash      string        `json:"hash"`
}

type CallsSchedule struct {
	Weekdays [][2][2]string `json:"weekdays"`
	Saturday [][2][2]string `json:"saturday"`
}

type CallsCache struct {
	Site         CallsSource `json:"site"`
	Manual       CallsSource `json:"manual"`
	Active       CallsActive `json:"active"`
	Update       int64       `json:"update"`
	Changed      int64       `json:"changed"`
	ManualReason string      `json:"manualReason"`
}

type RaspCache struct {
	mu            sync.RWMutex
	dir           string
	Groups        *RaspEntry[map[string]any] `json:"groups"`
	Teachers      *RaspEntry[map[string]any] `json:"teachers"`
	Team          TeamCacheEntry             `json:"team"`
	Calls         CallsCache                 `json:"calls"`
	SuccessUpdate bool                       `json:"successUpdate"`

	events []Event

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
		Groups: &RaspEntry[map[string]any]{
			Timetable: make(map[string]any),
		},
		Teachers: &RaspEntry[map[string]any]{
			Timetable: make(map[string]any),
		},
		Team: TeamCacheEntry{
			Names: make(map[string]string),
		},
		SuccessUpdate: true,
	}

	c.load()
	return c, nil
}

func (c *RaspCache) GetGroups() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Groups.Timetable
}

func (c *RaspCache) GetTeachers() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Teachers.Timetable
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

func (c *RaspCache) SetGroups(groups map[string]any, hash string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.setTimetable(KindGroups, c.Groups, groups, hash)
}

func (c *RaspCache) SetTeachers(teachers map[string]any, hash string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.setTimetable(KindTeachers, c.Teachers, teachers, hash)
}

func (c *RaspCache) setTimetable(kind string, entry *RaspEntry[map[string]any], data map[string]any, hash string) {
	now := time.Now().UnixMilli()
	old := entry.Timetable

	todayIdx := utils.DayIndexFromDate(time.Now())

	var newEvents []Event
	for value, v := range data {
		vm, ok := v.(map[string]any)
		if !ok {
			continue
		}
		var oldEntryMap map[string]any
		if ov, ok := old[value]; ok {
			oldEntryMap, _ = ov.(map[string]any)
		}
		for _, out := range collectEntryDayEvents(kind, value, oldEntryMap, vm, todayIdx) {
			newEvents = append(newEvents, out.ev)
		}
	}

	previousWeekIndex := entry.LastWeekIndex
	weekEvents, maxWeek := collectWeekEvents(kind, entry, data)
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
			om["days"] = md
			mergedTimetable[k] = om
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

	entry.Timetable = mergedTimetable
	entry.Update = now
	entry.Hash = hash

	if !mapsEqual(old, mergedTimetable) {
		entry.Changed = now
	}

	entry.LastWeekIndex = maxWeek

	c.events = append(c.events, newEvents...)
}

func (c *RaspCache) SetTeam(names map[string]string, hashes []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Team.Names = names
	c.Team.Update = time.Now().UnixMilli()
	c.Team.Hash = hashes
}

func (c *RaspCache) SetCalls(site Schedule, manual Schedule, source string) {
	c.setCallsInternal(site, manual, source, "", false)
}

func (c *RaspCache) SetCallsNotify(site Schedule, manual Schedule, source string, reason string) {
	c.setCallsInternal(site, manual, source, reason, true)
}

func (c *RaspCache) setCallsInternal(site Schedule, manual Schedule, source, reason string, skipNotify bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixMilli()
	if source == "site" {
		c.Calls.ManualReason = ""
	}

	prevActive := c.Calls.Active.Schedule
	weekdaysChanged := schedulesNotEqual(prevActive.Weekdays, site.Weekdays)
	saturdayChanged := schedulesNotEqual(prevActive.Saturday, site.Saturday)

	c.Calls.Site = CallsSource{
		Schedule:  CallsSchedule{Weekdays: site.Weekdays, Saturday: site.Saturday},
		UpdatedAt: now,
		Hash:      hashSchedule(site),
	}
	c.Calls.Manual = CallsSource{
		Schedule:  CallsSchedule{Weekdays: manual.Weekdays, Saturday: manual.Saturday},
		UpdatedAt: now,
		Hash:      hashSchedule(manual),
	}
	c.Calls.Active = CallsActive{
		Schedule:  CallsSchedule{Weekdays: site.Weekdays, Saturday: site.Saturday},
		UpdatedAt: now,
		Source:    source,
		Hash:      hashSchedule(site),
	}
	c.Calls.Update = now
	c.Calls.Changed = now

	if skipNotify {
		return
	}

	if weekdaysChanged || saturdayChanged {
		c.events = append(c.events, Event{Calls: &CallsEvent{
			WeekdaysChanged: weekdaysChanged,
			SaturdayChanged: saturdayChanged,
			Reason:          reason,
			Schedule:        CallsSchedule{Weekdays: site.Weekdays, Saturday: site.Saturday},
		}})
	}
}

type Schedule struct {
	Weekdays [][2][2]string
	Saturday [][2][2]string
}

func (c *RaspCache) SetSuccessUpdate(ok bool) {
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
		GroupsCount:    len(c.Groups.Timetable),
		TeachersCount:  len(c.Teachers.Timetable),
		SuccessUpdate:  c.SuccessUpdate,
		GroupsUpdate:   c.Groups.Update,
		TeachersUpdate: c.Teachers.Update,
		GroupsHash:     c.Groups.Hash,
		TeachersHash:   c.Teachers.Hash,
	}
}

func (c *RaspCache) RecordHit()  { c.hits.Add(1) }
func (c *RaspCache) RecordMiss() { c.misses.Add(1) }
func (c *RaspCache) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Groups = &RaspEntry[map[string]any]{Timetable: make(map[string]any)}
	c.Teachers = &RaspEntry[map[string]any]{Timetable: make(map[string]any)}
	c.Team = TeamCacheEntry{Names: make(map[string]string)}
	c.events = nil
}

func (c *RaspCache) Save() error {
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
		if err := os.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", e.name, err)
		}
	}

	return nil
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

func (c *RaspCache) GetGroupsHash() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Groups.Hash
}

func (c *RaspCache) GetTeachersHash() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Teachers.Hash
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

func (c *RaspCache) SetCallsFromCache(calls CallsCache) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Calls = calls
}

func (c *RaspCache) SetCallsManual(weekdays, saturday [][2][2]string, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixMilli()
	manuallySet := CallsSource{
		Schedule:  CallsSchedule{Weekdays: weekdays, Saturday: saturday},
		UpdatedAt: now,
		Hash:      hashSchedule(Schedule{Weekdays: weekdays, Saturday: saturday}),
	}
	weekdaysChanged := schedulesNotEqual(c.Calls.Active.Schedule.Weekdays, weekdays)
	saturdayChanged := schedulesNotEqual(c.Calls.Active.Schedule.Saturday, saturday)

	c.Calls.Manual = manuallySet
	c.Calls.ManualReason = reason
	c.Calls.Active = CallsActive{
		Schedule:  manuallySet.Schedule,
		UpdatedAt: now,
		Source:    "manual",
		Hash:      manuallySet.Hash,
	}
	c.Calls.Changed = now

	if weekdaysChanged || saturdayChanged {
		c.events = append(c.events, Event{Calls: &CallsEvent{
			WeekdaysChanged: weekdaysChanged,
			SaturdayChanged: saturdayChanged,
			Reason:          reason,
			Schedule:        manuallySet.Schedule,
		}})
	}
}
