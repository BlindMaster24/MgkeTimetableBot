package telegram

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/reminder"
	"github.com/blindmaster24/MgkeTimetableBot/internal/rooms"
	"github.com/blindmaster24/MgkeTimetableBot/internal/timetable"
)

type freeRoomsCmd struct{ bot *Bot }

func (c *freeRoomsCmd) Name() string        { return "/free" }
func (c *freeRoomsCmd) Description() string { return c.bot.loc("cmd_free") }

func (c *freeRoomsCmd) MatchText(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return lower == "/free" || strings.HasPrefix(lower, "/free ")
}

func (c *freeRoomsCmd) Handler(ctx context.Context, u *Update) error {
	now := c.bot.nowTime()
	universe, occupied := dayRooms(c.bot.cache, timetable.FormatDate(now))
	if len(universe) == 0 {
		return u.Bot.SendText(u.ChatID, c.bot.loc("free_no_data"))
	}

	calls := reminder.Schedule{
		Weekdays: c.bot.cache.GetCallsWeekdays(),
		Saturday: c.bot.cache.GetCallsSaturday(),
	}

	num, requested := freePairArgument(u.Text)
	if requested {
		if _, _, ok := reminder.Slot(now, now.Location(), calls, num); !ok {
			return u.Bot.SendText(u.ChatID, c.bot.loc("free_no_pair"))
		}
	} else {
		pair, ok := pairAround(now, calls)
		if !ok {
			return u.Bot.SendText(u.ChatID, c.bot.loc("free_no_pairs"))
		}
		num = pair
	}

	start, end, ok := reminder.Slot(now, now.Location(), calls, num)
	if !ok {
		return u.Bot.SendText(u.ChatID, c.bot.loc("free_no_pair"))
	}

	span := map[string]any{
		"Num":   num,
		"Start": start.Format("15:04"),
		"End":   end.Format("15:04"),
	}
	free := rooms.Free(universe, occupied[num])
	if len(free) == 0 {
		return u.Bot.SendText(u.ChatID, c.bot.locData("free_busy", span))
	}
	span["Rooms"] = strings.Join(free, ", ")
	return u.Bot.SendText(u.ChatID, c.bot.locData("free_header", span))
}

func dayRooms(c *cache.RaspCache, date string) (map[string]string, map[int]map[string]bool) {
	var listed []string
	occupied := map[int]map[string]bool{}
	for _, kind := range []string{cache.KindGroups, cache.KindTeachers} {
		for _, day := range c.TimetableDays(kind, date) {
			for _, lesson := range day.Lessons {
				room := strings.TrimSpace(lesson.Room)
				if room == "" {
					continue
				}
				listed = append(listed, room)
				if occupied[lesson.Num] == nil {
					occupied[lesson.Num] = map[string]bool{}
				}
				occupied[lesson.Num][rooms.Key(room)] = true
			}
		}
	}
	return rooms.Universe(listed), occupied
}

func freePairArgument(text string) (int, bool) {
	fields := strings.Fields(text)
	if len(fields) < 2 {
		return 0, false
	}
	num, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, true
	}
	return num, true
}

func pairAround(now time.Time, calls reminder.Schedule) (int, bool) {
	for num := 1; ; num++ {
		_, end, ok := reminder.Slot(now, now.Location(), calls, num)
		if !ok {
			return 0, false
		}
		if end.After(now) {
			return num, true
		}
	}
}
