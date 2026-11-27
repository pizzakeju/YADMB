package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
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
	case "autoplay":
		autoplayMessage(srv, e)
	case "backward":
		backwardMessage(srv, e)
	case "forward":
		forwardMessage(srv, e)
	case "join":
		joinMessage(srv, e)
	case "loop":
		loopMessage(srv, e)
	case "lyrics":
		lyricsMessage(srv, e)
	case "move":
		moveMessage(srv, e, fields[1:])
	case "nowplaying":
		nowPlayingMessage(srv, e)
	case "previous":
		previousMessage(srv, e)
	case "remove":
		removeMessage(srv, e, fields[1:])
	case "seek":
		seekMessage(srv, e, fields[1:])
	case "shuffle":
		shuffleMessage(srv, e)
	case "skipto":
		skiptoMessage(srv, e, fields[1:])
	case "volume":
		volumeMessage(srv, e, fields[1:])
	case "pl-create":
		playlistCreateMessage(srv, e, fields[1:])
	case "pl-delete":
		playlistDeleteMessage(srv, e, fields[1:])
	case "pl-info":
		playlistInfoMessage(srv, e, fields[1:])
	case "pl-list":
		playlistListMessage(srv, e)
	case "pl-play":
		playlistPlayMessage(srv, e, fields[1:], false)
	case "pl-playshuffle":
		playlistPlayMessage(srv, e, fields[1:], true)
	case "pl-removedupe":
		playlistDedupeMessage(srv, e, fields[1:])
	case "pl-removetrack":
		playlistRemoveTrackMessage(srv, e, fields[1:])
	case "pl-rename":
		playlistRenameMessage(srv, e, fields[1:])
	case "pl-savecurrent":
		playlistSaveMessage(srv, e, fields[1:], false)
	case "pl-savequeue":
		playlistSaveMessage(srv, e, fields[1:], true)
	}
}

// commandAliases maps shortforms to canonical prefix commands.
var commandAliases = map[string]string{
	"p":          "play",
	"pl":         "playlist",
	"st":         "stream",
	"s":          "skip",
	"c":          "clear",
	"clearqueue": "clear",
	"cq":         "clear",
	"pa":         "pause",
	"r":          "resume",
	"q":          "queue",
	"dc":         "disconnect",
	"leave":      "disconnect",
	"lv":         "disconnect",
	"stop":       "disconnect",
	"np":         "nowplaying",
	"prev":       "previous",
	"rm":         "remove",
	"sh":         "shuffle",
	"vol":        "volume",
	"h":          "help",
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
func autoplayMessage(srv *manager.Server, e *events.MessageCreate) {
	next := !srv.Autoplay.Load()
	srv.Autoplay.Store(next)
	status := "disabled"
	if next {
		status = "enabled"
	}
	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField("Autoplay", "Autoplay "+status, false).WithColor(0x7289DA), 5*time.Second)
}

func loopMessage(srv *manager.Server, e *events.MessageCreate) {
	next := !srv.LoopAll.Load()
	srv.LoopAll.Store(next)
	status := "disabled"
	if next {
		status = "enabled"
	}
	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField("Loop", "Loop "+status, false).WithColor(0x7289DA), 5*time.Second)
}

func joinMessage(srv *manager.Server, e *events.MessageCreate) {
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

	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField("Joined", "Joined the voice channel", false).WithColor(0x7289DA), 5*time.Second)
}

func forwardMessage(srv *manager.Server, e *events.MessageCreate) {
	if !srv.IsPlaying() {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, constants.NothingPlaying, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	target := int(srv.Frames.Load()) + int(math.Trunc(10*constants.FrameSeconds))
	if !srv.SeekFrame(target) {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.GotoTitle, constants.GotoInvalid, false).WithColor(0x7289DA), 5*time.Second)
		return
	}
	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.GotoTitle, "Forwarded 10 seconds", false).WithColor(0x7289DA), 5*time.Second)
}

func backwardMessage(srv *manager.Server, e *events.MessageCreate) {
	if !srv.IsPlaying() {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, constants.NothingPlaying, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	select {
	case srv.Restart <- struct{}{}:
	default:
	}
	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.GotoTitle, "Restarted the current song", false).WithColor(0x7289DA), 5*time.Second)
}

func nowPlayingMessage(srv *manager.Server, e *events.MessageCreate) {
	if !srv.IsPlaying() {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, constants.NothingPlaying, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	el := srv.Queue.GetFirstElement()
	if el == nil {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.QueueTitle, constants.QueueEmpty, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField("Now playing", fmt.Sprintf("[%s](%s) - %s/%s added by %s", el.Title, el.Link,
		manager.FormatDuration(float64(srv.Frames.Load())/constants.FrameSeconds), el.Duration, el.User), false).
		WithColor(0x7289DA).WithThumbnail(el.Thumbnail), 10*time.Second)
}

func previousMessage(srv *manager.Server, e *events.MessageCreate) {
	prev := srv.LastPlayed()
	if prev == nil {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "No previous song", false).WithColor(0x7289DA), 5*time.Second)
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

	srv.Play(manager.PlayEvent{
		Username:    prev.User,
		Song:        prev.Link,
		Clients:     &clients,
		Event:       nil,
		Random:      false,
		Loop:        prev.Loop,
		Priority:    true,
		IsDeferred:  nil,
		TextChannel: e.ChannelID,
	})
}

func seekMessage(srv *manager.Server, e *events.MessageCreate, args []string) {
	if len(args) == 0 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Usage: "+prefix+"seek <seconds|±seconds>", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	if !srv.IsPlaying() {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, constants.NothingPlaying, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	d, err := time.ParseDuration(args[0])
	if err != nil {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, constants.GotoInvalid, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	current := int(srv.Frames.Load())
	target := int(d.Seconds() * constants.FrameSeconds)
	if strings.HasPrefix(args[0], "+") || strings.HasPrefix(args[0], "-") {
		target = current + target
	} else if target <= current {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, constants.GotoBackwards, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	if !srv.SeekFrame(target) {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, constants.GotoInvalid, false).WithColor(0x7289DA), 5*time.Second)
		return
	}
	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.GotoTitle, constants.SkippedTo+args[0], false).WithColor(0x7289DA), 5*time.Second)
}

func moveMessage(srv *manager.Server, e *events.MessageCreate, args []string) {
	if len(args) != 2 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Usage: "+prefix+"move <from> <to>", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	from, err1 := strconv.Atoi(args[0])
	to, err2 := strconv.Atoi(args[1])
	if err1 != nil || err2 != nil || from < 2 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Positions start at 1 (the current song is 1)", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	if err := srv.Queue.Move(from-1, to-1); err != nil {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Position out of range", false).WithColor(0x7289DA), 5*time.Second)
		return
	}
	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.SuccessfulTitle, "Song moved", false).WithColor(0x7289DA), 5*time.Second)
}

func removeMessage(srv *manager.Server, e *events.MessageCreate, args []string) {
	if len(args) != 1 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Usage: "+prefix+"remove <position>", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	n, err := strconv.Atoi(args[0])
	if err != nil || n < 1 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Invalid position", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	queue := srv.Queue.GetAllQueue()
	if n >= len(queue) {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Position out of range", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	title := queue[n].Title
	if err := srv.Queue.RemoveAt(n); err != nil {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, err.Error(), false).WithColor(0x7289DA), 5*time.Second)
		return
	}
	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.SuccessfulTitle, "Removed: "+title, false).WithColor(0x7289DA), 5*time.Second)
}

func shuffleMessage(srv *manager.Server, e *events.MessageCreate) {
	srv.Queue.ShuffleRemaining()
	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField("Shuffle", "Queue shuffled", false).WithColor(0x7289DA), 5*time.Second)
}

func skiptoMessage(srv *manager.Server, e *events.MessageCreate, args []string) {
	if len(args) != 1 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Usage: "+prefix+"skipto <position>", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	n, err := strconv.Atoi(args[0])
	if err != nil || n < 1 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Invalid position", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	queue := srv.Queue.GetAllQueue()
	if n > len(queue) {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Position out of range", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	for i := 1; i < n; i++ {
		_ = srv.Queue.RemoveAt(1)
	}
	srv.SkipSong(manager.Skip)
	srv.Paused.Store(false)
	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.SkipTitle, fmt.Sprintf("Skipped to %d", n), false).WithColor(0x7289DA), 5*time.Second)
}

func volumeMessage(srv *manager.Server, e *events.MessageCreate, args []string) {
	if len(args) != 1 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Usage: "+prefix+"volume <0-200>", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	v, err := strconv.Atoi(args[0])
	if err != nil || v < 0 || v > 200 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Volume must be between 0 and 200", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	srv.Volume.Store(int32(v))
	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField("Volume", fmt.Sprintf("Volume set to %d%%", v), false).WithColor(0x7289DA), 5*time.Second)
}

func lyricsMessage(srv *manager.Server, e *events.MessageCreate) {
	if !srv.IsPlaying() {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, constants.NothingPlaying, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	el := srv.Queue.GetFirstElement()
	if el == nil {
		return
	}

	lyrics, err := fetchLyrics(el.Title)
	if err != nil {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, err.Error(), false).WithColor(0x7289DA), 10*time.Second)
		return
	}

	if len(lyrics) > 3900 {
		lyrics = lyrics[:3900] + "..."
	}
	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).WithDescription(lyrics).WithColor(0x7289DA), 30*time.Second)
}

func fetchLyrics(query string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://api.lyrics.ovh/v1/" + url.PathEscape(query))
	if err != nil {
		return "", errors.New("could not reach the lyrics service")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("lyrics not found")
	}

	var out struct {
		Lyrics string `json:"lyrics"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", errors.New("could not parse the lyrics response")
	}

	if strings.TrimSpace(out.Lyrics) == "" {
		return "", errors.New("lyrics not found")
	}
	return out.Lyrics, nil
}

func playlistCreateMessage(srv *manager.Server, e *events.MessageCreate, args []string) {
	if len(args) != 1 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Usage: "+prefix+"pl-create <name>", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	if err := clients.Database.CreatePlaylist(*e.GuildID, args[0]); err != nil {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Playlist already exists", false).WithColor(0x7289DA), 5*time.Second)
		return
	}
	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.SuccessfulTitle, "Playlist created: "+args[0], false).WithColor(0x7289DA), 5*time.Second)
}

func playlistDeleteMessage(srv *manager.Server, e *events.MessageCreate, args []string) {
	if len(args) != 1 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Usage: "+prefix+"pl-delete <name>", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	if err := clients.Database.DeletePlaylist(*e.GuildID, args[0]); err != nil {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, err.Error(), false).WithColor(0x7289DA), 5*time.Second)
		return
	}
	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.SuccessfulTitle, "Playlist deleted: "+args[0], false).WithColor(0x7289DA), 5*time.Second)
}

func playlistRenameMessage(srv *manager.Server, e *events.MessageCreate, args []string) {
	if len(args) != 2 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Usage: "+prefix+"pl-rename <old> <new>", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	if err := clients.Database.RenamePlaylist(*e.GuildID, args[0], args[1]); err != nil {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, err.Error(), false).WithColor(0x7289DA), 5*time.Second)
		return
	}
	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.SuccessfulTitle, "Playlist renamed", false).WithColor(0x7289DA), 5*time.Second)
}

func playlistListMessage(srv *manager.Server, e *events.MessageCreate) {
	playlists := clients.Database.ListPlaylists(*e.GuildID)
	if len(playlists) == 0 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField("Playlists", "No playlists yet. Use "+prefix+"pl-create <name>", false).WithColor(0x7289DA), 10*time.Second)
		return
	}

	var b strings.Builder
	for _, p := range playlists {
		fmt.Fprintf(&b, "%s (%d songs)\n", p.Name, p.Count)
	}
	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField("Playlists", b.String(), false).WithColor(0x7289DA), 15*time.Second)
}

func playlistInfoMessage(srv *manager.Server, e *events.MessageCreate, args []string) {
	if len(args) != 1 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Usage: "+prefix+"pl-info <name>", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	entries := clients.Database.GetPlaylistEntries(*e.GuildID, args[0])
	if len(entries) == 0 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Playlist is empty or does not exist", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	var b strings.Builder
	for i, entry := range entries {
		if i >= 20 {
			fmt.Fprintf(&b, "... and %d more\n", len(entries)-i)
			break
		}
		fmt.Fprintf(&b, "%d. %s\n", i+1, entry)
	}
	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField("Playlist: "+args[0], b.String(), false).WithColor(0x7289DA), 20*time.Second)
}

func playlistPlayMessage(srv *manager.Server, e *events.MessageCreate, args []string, forceShuffle bool) {
	if len(args) == 0 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Usage: "+prefix+"pl-play <name> [-shuffle] [-loop] [-priority]", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	clean, flags := parsePlayFlags(args[1:])
	name := args[0]
	if len(clean) > 0 {
		name = clean[0]
	}

	entries := clients.Database.GetPlaylistEntries(*e.GuildID, name)
	if len(entries) == 0 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Playlist is empty or does not exist", false).WithColor(0x7289DA), 5*time.Second)
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

	if forceShuffle || flags.random {
		rand.Shuffle(len(entries), func(i, j int) {
			entries[i], entries[j] = entries[j], entries[i]
		})
	}

	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.EnqueuedTitle, fmt.Sprintf("Enqueued %d songs from %s", len(entries), name), false).WithColor(0x7289DA), 5*time.Second)

	go func() {
		for _, song := range entries {
			if srv.Clear.Load() {
				break
			}
			srv.Play(manager.PlayEvent{
				Username:    e.Message.Author.Username,
				Song:        song,
				Clients:     &clients,
				Event:       nil,
				Random:      false,
				Loop:        flags.loop,
				Priority:    flags.priority,
				IsDeferred:  nil,
				TextChannel: e.ChannelID,
			})
		}
	}()
}

func playlistDedupeMessage(srv *manager.Server, e *events.MessageCreate, args []string) {
	if len(args) != 1 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Usage: "+prefix+"pl-removedupe <name>", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	if err := clients.Database.RemoveDuplicates(*e.GuildID, args[0]); err != nil {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, err.Error(), false).WithColor(0x7289DA), 5*time.Second)
		return
	}
	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.SuccessfulTitle, "Duplicates removed", false).WithColor(0x7289DA), 5*time.Second)
}

func playlistRemoveTrackMessage(srv *manager.Server, e *events.MessageCreate, args []string) {
	if len(args) != 2 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Usage: "+prefix+"pl-removetrack <name> <position>", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	pos, err := strconv.Atoi(args[1])
	if err != nil || pos < 1 {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, "Invalid position", false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	if err := clients.Database.RemoveFromPlaylist(*e.GuildID, args[0], pos); err != nil {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, err.Error(), false).WithColor(0x7289DA), 5*time.Second)
		return
	}
	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.SuccessfulTitle, "Track removed", false).WithColor(0x7289DA), 5*time.Second)
}

func playlistSaveMessage(srv *manager.Server, e *events.MessageCreate, args []string, saveQueue bool) {
	if len(args) != 1 {
		usage := "Usage: " + prefix + "pl-savecurrent <name>"
		if saveQueue {
			usage = "Usage: " + prefix + "pl-savequeue <name>"
		}
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, usage, false).WithColor(0x7289DA), 5*time.Second)
		return
	}

	var entries []string
	if saveQueue {
		queue := srv.Queue.GetAllQueue()
		if len(queue) == 0 {
			sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, constants.QueueEmpty, false).WithColor(0x7289DA), 5*time.Second)
			return
		}
		entries = make([]string, 0, len(queue))
		for _, el := range queue {
			entries = append(entries, el.Link)
		}
	} else {
		el := srv.Queue.GetFirstElement()
		if el == nil {
			sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, constants.NothingPlaying, false).WithColor(0x7289DA), 5*time.Second)
			return
		}
		entries = []string{el.Link}
	}

	if err := clients.Database.SavePlaylist(*e.GuildID, args[0], entries); err != nil {
		sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.ErrorTitle, err.Error(), false).WithColor(0x7289DA), 5*time.Second)
		return
	}
	sendMsg(e, discord.NewEmbed().WithTitle(manager.BotName).AddField(constants.SuccessfulTitle, fmt.Sprintf("Saved %d songs to %s", len(entries), args[0]), false).WithColor(0x7289DA), 5*time.Second)
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
