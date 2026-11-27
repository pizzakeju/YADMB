package embed

import (
	"time"

	"github.com/disgoorg/disgo/discord"
)

// Colors used across the bot embeds.
const (
	ColorBlurple = 0x7289DA
	ColorGreen   = 0x57F287
	ColorRed     = 0xED4245
	ColorYellow  = 0xFEE75C
)

// Kind selects a visual theme for a message embed.
type Kind uint8

const (
	KindInfo Kind = iota
	KindSuccess
	KindError
	KindWarning
)

// Build returns a themed embed with a title, a message field and a footer.
// The title should carry the action (e.g. "✅ Queue cleared"), the message the
// details.
func Build(kind Kind, title, msg string) discord.Embed {
	color := ColorBlurple
	switch kind {
	case KindSuccess:
		color = ColorGreen
	case KindError:
		color = ColorRed
	case KindWarning:
		color = ColorYellow
	}

	return discord.NewEmbed().WithTitle(title).AddField("", msg, false).
		WithColor(color).WithFooterText("GRUPI").WithTimestamp(time.Now())
}

// Info returns a blurple themed embed.
func Info(title, msg string) discord.Embed {
	return Build(KindInfo, title, msg)
}

// Success returns a green themed embed.
func Success(title, msg string) discord.Embed {
	return Build(KindSuccess, title, msg)
}

// Error returns a red themed embed.
func Error(title, msg string) discord.Embed {
	return Build(KindError, title, msg)
}

// Warning returns a yellow themed embed.
func Warning(title, msg string) discord.Embed {
	return Build(KindWarning, title, msg)
}
