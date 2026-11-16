// Package model holds the timetable types shared by the parser, the cache and the archive.
package model

type CallsSchedule struct {
	Weekdays [][2][2]string `json:"weekdays"`
	Saturday [][2][2]string `json:"saturday"`
}
