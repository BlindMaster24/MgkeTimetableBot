package telegram

import "github.com/blindmaster24/MgkeTimetableBot/internal/notification"

type EventChatFinder struct {
	repo     *Repository
	adminIDs []int64
}

func NewEventChatFinder(repo *Repository, adminIDs []int64) *EventChatFinder {
	return &EventChatFinder{repo: repo, adminIDs: adminIDs}
}

func (f *EventChatFinder) toEventChats(chats []*Chat) []*notification.EventChat {
	result := make([]*notification.EventChat, 0, len(chats))
	for _, c := range chats {
		result = append(result, &notification.EventChat{
			ID:               c.ID,
			PeerID:           c.PeerID,
			Mode:             string(c.Mode),
			Group:            c.Group,
			Teacher:          c.Teacher,
			NoticeChanges:    c.NoticeChanges,
			NoticeNextWeek:   c.NoticeNextWeek,
			NoticeCalls:      c.NoticeCalls,
			NoticeParserErrs: c.NoticeParserErrors,
			AllowSendMess:    c.AllowSendMess,
			Formatter:        c.Formatter,
			HidePastDays:     c.HidePastDays,
			ShowHints:        c.ShowHints,
			ShowParserTime:   c.ShowParserTime,
		})
	}
	return result
}

func (f *EventChatFinder) FindChatsByGroups(service string, groups []string, noticeChanges bool) ([]*notification.EventChat, error) {
	chats, err := f.repo.FindChatsByGroups(service, groups, noticeChanges)
	if err != nil {
		return nil, err
	}
	return f.toEventChats(chats), nil
}

func (f *EventChatFinder) FindChatsByTeachers(service string, teachers []string, noticeChanges bool) ([]*notification.EventChat, error) {
	chats, err := f.repo.FindChatsByTeachers(service, teachers, noticeChanges)
	if err != nil {
		return nil, err
	}
	return f.toEventChats(chats), nil
}

func (f *EventChatFinder) FindSubscribedChatsByGroup(service, group string, noticeChanges bool) ([]*notification.EventChat, error) {
	chats, err := f.repo.FindSubscribedChatsByGroup(service, group, noticeChanges)
	if err != nil {
		return nil, err
	}
	return f.toEventChats(chats), nil
}

func (f *EventChatFinder) FindSubscribedChatsByTeacher(service, teacher string, noticeChanges bool) ([]*notification.EventChat, error) {
	chats, err := f.repo.FindSubscribedChatsByTeacher(service, teacher, noticeChanges)
	if err != nil {
		return nil, err
	}
	return f.toEventChats(chats), nil
}

func (f *EventChatFinder) FindChatsWithNotice(service string, notice string) ([]*notification.EventChat, error) {
	chats, err := f.repo.FindChatsWithNotice(service, notice)
	if err != nil {
		return nil, err
	}
	return f.toEventChats(chats), nil
}

func (f *EventChatFinder) FindAdminChats(service string) ([]*notification.EventChat, error) {
	chats, err := f.repo.FindAdminChats(service, f.adminIDs)
	if err != nil {
		return nil, err
	}
	return f.toEventChats(chats), nil
}
