package model

type ChatMode string

const (
	ChatModeStudent ChatMode = "student"
	ChatModeTeacher ChatMode = "teacher"
	ChatModeParent  ChatMode = "parent"
	ChatModeGuest   ChatMode = "guest"
)
