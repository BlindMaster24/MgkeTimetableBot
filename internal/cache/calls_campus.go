package cache

func (c *RaspCache) ActiveCallsForCampus(name string, fallback CallsSchedule) CallsSchedule {
	c.mu.RLock()
	defer c.mu.RUnlock()

	schedule := c.Calls.Active.Schedule
	if c.Calls.Active.Source == "site" && name != "" {
		for _, variant := range c.Calls.SiteVariants {
			if variant.Name == name {
				schedule = variant.Schedule
				break
			}
		}
	}

	if len(schedule.Weekdays) == 0 {
		schedule = fallback
	}
	if len(schedule.Saturday) == 0 {
		schedule.Saturday = schedule.Weekdays
	}
	return schedule
}
