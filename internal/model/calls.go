package model


type CallsSchedule struct {
	Weekdays [][2][2]string `json:"weekdays"`
	Saturday [][2][2]string `json:"saturday"`
}
