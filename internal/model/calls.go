package model

type CallSlot [2][2]string

type CallsSchedule struct {
	Weekdays []CallSlot `json:"weekdays"`
	Saturday []CallSlot `json:"saturday"`
}
