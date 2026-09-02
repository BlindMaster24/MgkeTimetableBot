package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type RaspEntry[T any] struct {
	Timetable     T       `json:"timetable"`
	Update        int64   `json:"update"`
	Changed       int64   `json:"changed"`
	LastWeekIndex int     `json:"lastWeekIndex"`
	Hash          string  `json:"hash"`
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

	hits   atomic.Int64
	misses atomic.Int64
}

type Stats struct {
	Hits         int64 `json:"hits"`
	Misses       int64 `json:"misses"`
	GroupsCount  int   `json:"groupsCount"`
	TeachersCount int  `json:"teachersCount"`
	SuccessUpdate bool `json:"successUpdate"`
	GroupsUpdate  int64 `json:"groupsUpdate"`
	TeachersUpdate int64 `json:"teachersUpdate"`
	GroupsHash   string `json:"groupsHash"`
	TeachersHash string `json:"teachersHash"`
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

	now := time.Now().UnixMilli()
	old := c.Groups.Timetable

	c.Groups.Timetable = groups
	c.Groups.Update = now
	c.Groups.Hash = hash

	if !mapsEqual(old, groups) {
		c.Groups.Changed = now
	}
}

func (c *RaspCache) SetTeachers(teachers map[string]any, hash string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixMilli()
	old := c.Teachers.Timetable

	c.Teachers.Timetable = teachers
	c.Teachers.Update = now
	c.Teachers.Hash = hash

	if !mapsEqual(old, teachers) {
		c.Teachers.Changed = now
	}
}

func (c *RaspCache) SetTeam(names map[string]string, hashes []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Team.Names = names
	c.Team.Update = time.Now().UnixMilli()
	c.Team.Hash = hashes
}

func (c *RaspCache) SetCalls(site Schedule, manual Schedule, source string) {
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
		Hits:          c.hits.Load(),
		Misses:        c.misses.Load(),
		GroupsCount:   len(c.Groups.Timetable),
		TeachersCount: len(c.Teachers.Timetable),
		SuccessUpdate: c.SuccessUpdate,
		GroupsUpdate:  c.Groups.Update,
		TeachersUpdate: c.Teachers.Update,
		GroupsHash:    c.Groups.Hash,
		TeachersHash:  c.Teachers.Hash,
	}
}

func (c *RaspCache) RecordHit()   { c.hits.Add(1) }
func (c *RaspCache) RecordMiss()  { c.misses.Add(1) }
func (c *RaspCache) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Groups = &RaspEntry[map[string]any]{Timetable: make(map[string]any)}
	c.Teachers = &RaspEntry[map[string]any]{Timetable: make(map[string]any)}
	c.Team = TeamCacheEntry{Names: make(map[string]string)}
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
	c.Calls.Manual = manuallySet
	c.Calls.ManualReason = reason
	c.Calls.Active = CallsActive{
		Schedule:  manuallySet.Schedule,
		UpdatedAt: now,
		Source:    "manual",
		Hash:      manuallySet.Hash,
	}
	c.Calls.Changed = now
}
