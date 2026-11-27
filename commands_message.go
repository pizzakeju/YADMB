package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/TheTipo01/YADMB/constants"
	"github.com/TheTipo01/YADMB/embed"
	"github.com/TheTipo01/YADMB/manager"
	"github.com/TheTipo01/YADMB/queue"
	"github.com/TheTipo01/YADMB/vc"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

// prefix introduces classic message commands, e.g. "-play <link>".
const prefix = "-"

// messageCreate handles prefix commands in guild text channels.
func messageCreate(e *events.MessageCreate) {
	if e.Message.Author.Bot || !strings.HasPrefix(e.Message.Content, prefix) {
		return
	}

	if e.GuildID == nil {
		return // Commands are not available in DM
	}
	guild := *e.GuildID

	if ok, denied := guildAccess(e.Message.Author.ID, guild); !ok {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, denied, false).WithColor(0x7289DA), 3*time.Second)
		return
	}

	fields := strings.Fields(strings.TrimPrefix(e.Message.Content, prefix))
	if len(fields) == 0 {
		return
	}

	cmd := strings.ToLower(fields[0])
	if canonical, ok := commandAliases[cmd]; ok {
		cmd = canonical
	}

	srv := getServer(guild)

	switch cmd {
	case "help":
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).
			AddField("Prefix commands", "-play / -p <link|query>\n-playlist / -pl <link> [-shuffle] [-loop] [-priority]\n-stream / -st <url>\n-skip / -s | -clear / -c | -pause / -pa | -resume / -r\n-queue / -q | -disconnect / -dc | -help / -h", false).
			WithColor(0x7289DA), 15*time.Second)
	case "play":
		playMessage(srv, e, fields[1:], false)
	case "playlist":
		playMessage(srv, e, fields[1:], true)
	case "stream":
		streamMessage(srv, e, fields[1:])
	case "skip":
		skipMessage(srv, e)
	case "clear":
		clearMessage(srv, e)
	case "pause":
		pauseMessage(srv, e, true)
	case "resume":
		pauseMessage(srv, e, false)
	case "queue":
		queueMessage(srv, e)
	case "disconnect":
		disconnectMessage(srv, e)
	}
}

// commandAliases maps shortforms to canonical prefix commands.
var commandAliases = map[string]string{
	"p":  "play",
	"pl": "playlist",
	"st": "stream",
	"s":  "skip",
	"c":  "clear",
	"pa": "pause",
	"r":  "resume",
	"q":  "queue",
	"dc": "disconnect",
	"h":  "help",
}

// guildAccess returns whether the user may run commands in this guild, plus a
// message describing the denial when not allowed.
func guildAccess(user, guild snowflake.ID) (bool, string) {
	if _, ok := blacklist.Load(user.String()); ok {
		return false, constants.UserInBlacklist
	}

	if whitelist {
		if _, ok := guildList.Load(guild.String()); ok {
			return true, ""
		}
		return false, constants.ServerNotInWhitelist
	}

	if _, ok := guildList.Load(guild.String()); !ok {
		return true, ""
	}
	return false, constants.ServerInBlacklist
}

// djBlocked reports whether the author is blocked by DJ mode.
func djBlocked(srv *manager.Server, e *events.MessageCreate) bool {
	if !srv.DJEnabled() {
		return false
	}

	author := e.Message.Author.ID
	if _, isOwner := owners[author]; isOwner {
		return false
	}

	member := e.Message.Member
	if member == nil {
		if m, ok := e.Client().Caches.Member(*e.GuildID, author); ok {
			member = &m
		} else {
			return false
		}
	}

	return !manager.HasRole(member.RoleIDs, srv.DJRoleID())
}

// sendMsg sends an embed in the channel of the triggering message.
func sendMsg(e *events.MessageCreate, emb discord.Embed, wait time.Duration) {
	embed.SendAndDeleteEmbed(e.Client(), emb, e.ChannelID, wait)
}

func playMessage(srv *manager.Server, e *events.MessageCreate, args []string, playlist bool) {
	if len(args) == 0 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Usage: "+prefix+"play <link|query>", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	if djBlocked(srv, e) {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, constants.DjNot, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	vs := manager.FindUserVoiceState(e.Client(), *e.GuildID, e.Message.Author.ID)
	if vs == nil {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, constants.NotInVC, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	if err := srv.EnsureVoice(*vs.ChannelID); err != nil {
		msg := constants.CantJoinVC
		if err == vc.ErrAlreadyConnected {
			msg = constants.BotInAnotherVC
		}
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, msg, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	clean, flags := parsePlayFlags(args)
	link := strings.Join(clean, " ")

	var err error
	if !playlist {
		link, err = manager.FilterPlaylist(link)
	}
	if err != nil {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Playlist detected, but use "+prefix+"playlist instead.", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.EnqueuedTitle, link, false).WithColor(0x7289DA), 5*time.Second)

	srv.Play(manager.PlayEvent{
		Username:    e.Message.Author.Username,
		Song:        link,
		Clients:     &clients,
		Event:       nil,
		Random:      flags.random,
		Loop:        flags.loop,
		Priority:    flags.priority,
		IsDeferred:  nil,
		TextChannel: e.ChannelID,
	})
}

func streamMessage(srv *manager.Server, e *events.MessageCreate, args []string) {
	if len(args) == 0 || !manager.IsValidURL(args[0]) {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, constants.InvalidURL, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	if djBlocked(srv, e) {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, constants.DjNot, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	vs := manager.FindUserVoiceState(e.Client(), *e.GuildID, e.Message.Author.ID)
	if vs == nil {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, constants.NotInVC, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	if err := srv.EnsureVoice(*vs.ChannelID); err != nil {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, constants.CantJoinVC, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	url := args[0]
	stdout, cmds := manager.Stream(url)
	el := queue.Element{
		ID:          url,
		Title:       "Stream",
		Duration:    "",
		Link:        url,
		User:        e.Message.Author.Username,
		TextChannel: e.ChannelID,
		BeforePlay: func() error {
			if errs := manager.CmdsStart(cmds); len(errs) > 0 {
				return errors.New("error while starting the stream: " + manager.StringifyErrors(errs...))
			}
			return nil
		},
		AfterPlay: func() error {
			manager.CmdsKill(cmds)
			return nil
		},
		Reader: stdout,
		Closer: stdout,
	}

	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.EnqueuedTitle, url, false).WithColor(0x7289DA), 5*time.Second)
	srv.AddSong(false, el)
}

func skipMessage(srv *manager.Server, e *events.MessageCreate) {
	if manager.FindUserVoiceState(e.Client(), *e.GuildID, e.Message.Author.ID) == nil {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, constants.NotInVC, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	el := srv.Queue.GetFirstElement()
	if el != nil && srv.IsPlaying() {
		srv.SkipSong(manager.Skip)
		srv.Paused.Store(false)
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.SkipTitle,
			el.Title+" - "+el.Duration+" added by "+el.User, false).WithColor(0x7289DA).WithThumbnail(el.Thumbnail), 5*time.Second)
	} else {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.SkipTitle, constants.QueueEmpty, false).WithColor(0x7289DA), 5*time.Second)
	}
}

func clearMessage(srv *manager.Server, e *events.MessageCreate) {
	if manager.FindUserVoiceState(e.Client(), *e.GuildID, e.Message.Author.ID) == nil {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, constants.NotInVC, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	if srv.IsPlaying() {
		go srv.Clean()
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.QueueTitle, constants.QueueCleared, false).WithColor(0x7289DA), 5*time.Second)
	} else {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.QueueTitle, constants.QueueEmpty, false).WithColor(0x7289DA), 5*time.Second)
	}
}

func pauseMessage(srv *manager.Server, e *events.MessageCreate, pause bool) {
	if manager.FindUserVoiceState(e.Client(), *e.GuildID, e.Message.Author.ID) == nil {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, constants.NotInVC, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	if !srv.IsPlaying() {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.PauseTitle, constants.QueueEmpty, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	if pause {
		if srv.PausePlayback() {
			sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.PauseTitle, constants.Paused, false).WithColor(0x7289DA), 5*time.Second)
		} else {
			sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.PauseTitle, constants.AlreadyPaused, false).WithColor(0x7289DA), 5*time.Second)
		}
	} else {
		if srv.ResumePlayback() {
			sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ResumeTitle, constants.Resumed, false).WithColor(0x7289DA), 5*time.Second)
		} else {
			sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ResumeTitle, constants.AlreadyResumed, false).WithColor(0x7289DA), 5*time.Second)
		}
	}
}

func queueMessage(srv *manager.Server, e *events.MessageCreate) {
	const maxQueue = 10

	if !srv.IsPlaying() {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.QueueTitle, constants.QueueEmpty, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	el := srv.Queue.GetAllQueue()
	if len(el) == 0 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.QueueTitle, constants.QueueEmpty, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	builder := discord.NewEmbed().WithTitle(manager.BotName).WithDescription(constants.QueueTitle).AddField("1", fmt.Sprintf("[%s](%s) - %s/%s added by %s\n", el[0].Title, el[0].Link,
		manager.FormatDuration(float64(srv.Frames.Load())/constants.FrameSeconds), el[0].Duration, el[0].User), false)

	for j := 1; j < len(el) && j < maxQueue; j++ {
		builder = builder.AddField(strconv.Itoa(j+1), fmt.Sprintf("[%s](%s) - %s added by %s\n", el[j].Title, el[j].Link, el[j].Duration, el[j].User), false)
	}

	if len(el) > maxQueue {
		builder = builder.AddField("...", "And "+strconv.Itoa(len(el)-maxQueue)+" more", false)
	}

	sendMsg(e, builder.WithColor(0x7289DA), 20*time.Second)
}

func disconnectMessage(srv *manager.Server, e *events.MessageCreate) {
	if manager.FindUserVoiceState(e.Client(), *e.GuildID, e.Message.Author.ID) == nil {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, constants.NotInVC, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	// Stop playback and clear the queue, then leave the voice channel.
	go srv.Clean()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	srv.VC.Disconnect(ctx)
	cancel()

	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.DisconnectedTitle, constants.Disconnected, false).WithColor(0x7289DA), 5*time.Second)
}

type playFlags struct {
	random   bool
	loop     bool
	priority bool
}

// parsePlayFlags extracts flag tokens like "-shuffle"/"--shuffle" from the
// argument list and returns the remaining words.
func parsePlayFlags(args []string) ([]string, playFlags) {
	var flags playFlags
	clean := make([]string, 0, len(args))

	for _, arg := range args {
		switch arg {
		case "-shuffle", "--shuffle":
			flags.random = true
		case "-loop", "--loop":
			flags.loop = true
		case "-priority", "--priority":
			flags.priority = true
		default:
			clean = append(clean, arg)
		}
	}

	return clean, flags
}
