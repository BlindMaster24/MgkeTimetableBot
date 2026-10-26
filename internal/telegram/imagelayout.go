package telegram

import (
	"context"
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/image"
)

type imageLayoutCmd struct{ bot *Bot }

func (c *imageLayoutCmd) Name() string { return "/imagelayout" }

func (c *imageLayoutCmd) AdminOnly() bool { return true }

func (c *imageLayoutCmd) Description() string { return c.bot.loc("cmd_imagelayout") }

func (c *imageLayoutCmd) MatchText(text string) bool {
	return strings.EqualFold(strings.TrimSpace(text), "/imagelayout")
}

func (c *imageLayoutCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}
	return u.Bot.SendText(u.ChatID, c.bot.imageLayoutText())
}

func (b *Bot) imageLayoutText() string {
	checks, err := image.CheckGoldenLayout()
	return strings.Join(b.imageLayoutLines(image.FontPath(), checks, err), "\n")
}

func (b *Bot) imageLayoutLines(font string, checks []image.GoldenLayout, err error) []string {
	lines := []string{b.loc("image_layout_header")}

	if font == "" {
		lines = append(lines, b.loc("image_layout_font_missing"))
	} else {
		lines = append(lines, b.locData("image_layout_font", map[string]any{"Path": font}))
	}

	if err != nil {
		lines = append(lines, b.locData("image_layout_error", map[string]any{"Error": err.Error()}))
		lines = append(lines, b.loc("image_layout_drift"))
		return lines
	}

	drifted := false
	for _, check := range checks {
		sample := b.imageLayoutSample(check.Name)
		if check.Differing < 0 {
			drifted = true
			lines = append(lines, b.locData("image_layout_sample_moved", map[string]any{
				"Sample":     sample,
				"Width":      check.Width,
				"Height":     check.Height,
				"WantWidth":  check.WantWidth,
				"WantHeight": check.WantHeight,
			}))
			continue
		}
		if check.Differing > 0 {
			drifted = true
		}
		lines = append(lines, b.locData("image_layout_sample", map[string]any{
			"Sample":    sample,
			"Width":     check.Width,
			"Height":    check.Height,
			"Differing": check.Differing,
		}))
	}

	if font == "" {
		lines = append(lines, b.loc("image_layout_no_font"))
	}
	if drifted {
		lines = append(lines, b.loc("image_layout_drift"))
	}
	if font != "" && !drifted {
		lines = append(lines, b.loc("image_layout_ok"))
	}
	return lines
}

func (b *Bot) imageLayoutSample(name string) string {
	switch name {
	case "group":
		return b.loc("image_layout_sample_group")
	case "teacher":
		return b.loc("image_layout_sample_teacher")
	default:
		return name
	}
}
