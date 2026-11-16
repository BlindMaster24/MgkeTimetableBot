package telegram

import imagepkg "github.com/blindmaster24/MgkeTimetableBot/internal/image"

func (b *Bot) imageTexts() imagepkg.Texts {
	return imagepkg.Texts{
		GroupTitle:   b.loc("image_group_title"),
		TeacherTitle: b.loc("image_teacher_title"),
		Stamp:        b.loc("image_stamp"),
		Credit:       b.loc("image_credit"),
		Today:        b.loc("image_today"),
		Tomorrow:     b.loc("image_tomorrow"),
		GroupColumns: [5]string{
			b.loc("image_col_number"),
			b.loc("image_col_subject"),
			b.loc("image_col_type"),
			b.loc("image_col_room"),
			b.loc("image_col_teacher"),
		},
		TeacherColumns: [5]string{
			b.loc("image_col_number"),
			b.loc("image_col_subject"),
			b.loc("image_col_type"),
			b.loc("image_col_room"),
			b.loc("image_col_group"),
		},
	}
}
