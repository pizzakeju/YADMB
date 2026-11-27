package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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

// styled builds an embed with the shared footer.
func styled(build func(b discord.Embed) discord.Embed) discord.Embed {
	return build(discord.NewEmbed()).WithFooterText("GRUPI").WithTimestamp(time.Now())
}

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
		sendMsg(e, embed.Error("❌ Error", denied), 3*time.Second)
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
		builder := styled(func(b discord.Embed) discord.Embed {
			return b.WithTitle("🎧 GRUPI — Command Help").
				WithDescription("Control the bot with **prefix commands** in any text channel — shortforms are shown in parentheses. The same actions exist as slash commands.").
				WithColor(embed.ColorBlurple).
				AddField("🎵 Playback",
					"• `-play` / `-p` <link|query> [-shuffle] [-loop] [-priority]\n"+
						"• `-playlist` / `-pl` <link> [-shuffle] [-loop] [-priority]\n"+
						"• `-stream` / `-st` <url>\n"+
						"• `-previous` / `-prev` • `-nowplaying` / `-np`", false).
				AddField("🎛 Control",
					"• `-skip` / `-s` • `-clear` / `-c`\n"+
						"• `-pause` / `-pa` • `-resume` / `-r`\n"+
						"• `-disconnect` / `-dc` (leave, stop)\n"+
						"• `-seek` <s|±s> • `-forward` • `-backward` • `-skipto` <n>\n"+
						"• `-move` <from> <to> • `-remove` / `-rm` <n> • `-shuffle` / `-sh`\n"+
						"• `-loop` • `-autoplay` • `-volume` / `-vol` <0-200> • `-join`", false).
				AddField("📚 Playlists",
					"• `-pl-create` / `-pl-delete` <name>\n"+
						"• `-pl-rename` <old> <new> • `-pl-list` • `-pl-info` <name>\n"+
						"• `-pl-play` <name> [-shuffle] [-loop] [-priority] • `-pl-playshuffle` <name>\n"+
						"• `-pl-removedupe` <name> • `-pl-removetrack` <name> <pos>\n"+
						"• `-pl-savecurrent` <name> • `-pl-savequeue` <name>", false).
				AddField("💡 Tips",
					"• Queue position **1** = currently playing song.\n"+
						"• `-pl-play` accepts flags; Spotify links work everywhere.\n"+
						"• Use `-help` / `-h` anytime.", false)
		})
		sendMsg(e, builder, 30*time.Second)
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

// ensureVoice joins the channel and logs the underlying error on failure so
// why a join failed is diagnosable from the logs.
func ensureVoice(srv *manager.Server, channelID snowflake.ID) error {
	err := srv.EnsureVoice(channelID)
	if err != nil {
		slog.Warn("failed to join voice channel", "guild", srv.GuildID, "channel", channelID, "error", err)
	}
	return err
}

func playMessage(srv *manager.Server, e *events.MessageCreate, args []string, playlist bool) {
	if len(args) == 0 {
		sendMsg(e, embed.Error("❌ Error", "Usage: "+prefix+"play <link|query>"), 5*time.Second)
		return
	}

	if djBlocked(srv, e) {
		sendMsg(e, embed.Error("❌ Error", constants.DjNot), 5*time.Second)
		return
	}

	vs := manager.FindUserVoiceState(e.Client(), *e.GuildID, e.Message.Author.ID)
	if vs == nil {
		sendMsg(e, embed.Error("❌ Error", constants.NotInVC), 5*time.Second)
		return
	}

	if err := srv.EnsureVoice(*vs.ChannelID); err != nil {
		msg := constants.CantJoinVC
		if err == vc.ErrAlreadyConnected {
			msg = constants.BotInAnotherVC
		}
		sendMsg(e, embed.Error("❌ Error", msg), 5*time.Second)
		return
	}

	clean, flags := parsePlayFlags(args)
	link := strings.Join(clean, " ")

	var err error
	if !playlist {
		link, err = manager.FilterPlaylist(link)
	}
	if err != nil {
		sendMsg(e, embed.Error("❌ Error", "Playlist detected, but use "+prefix+"playlist instead."), 5*time.Second)
		return
	}

	sendMsg(e, embed.Success("✅ Enqueued", link), 5*time.Second)

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
		sendMsg(e, embed.Error("❌ Error", constants.InvalidURL), 5*time.Second)
		return
	}

	if djBlocked(srv, e) {
		sendMsg(e, embed.Error("❌ Error", constants.DjNot), 5*time.Second)
		return
	}

	vs := manager.FindUserVoiceState(e.Client(), *e.GuildID, e.Message.Author.ID)
	if vs == nil {
		sendMsg(e, embed.Error("❌ Error", constants.NotInVC), 5*time.Second)
		return
	}

	if err := srv.EnsureVoice(*vs.ChannelID); err != nil {
		sendMsg(e, embed.Error("❌ Error", constants.CantJoinVC), 5*time.Second)
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

	sendMsg(e, embed.Success("✅ Enqueued", url), 5*time.Second)
	srv.AddSong(false, el)
}

func skipMessage(srv *manager.Server, e *events.MessageCreate) {
	if manager.FindUserVoiceState(e.Client(), *e.GuildID, e.Message.Author.ID) == nil {
		sendMsg(e, embed.Error("❌ Error", constants.NotInVC), 5*time.Second)
		return
	}

	el := srv.Queue.GetFirstElement()
	if el != nil && srv.IsPlaying() {
		srv.SkipSong(manager.Skip)
		srv.Paused.Store(false)
		sendMsg(e, styled(func(b discord.Embed) discord.Embed {
			return b.WithTitle("⏭ Skipped").AddField("", el.Title+" - "+el.Duration+" added by "+el.User, false).
				WithColor(embed.ColorGreen).WithThumbnail(el.Thumbnail)
		}), 5*time.Second)
	} else {
		sendMsg(e, embed.Info("⏭ Skipped", constants.QueueEmpty), 5*time.Second)
	}
}

func clearMessage(srv *manager.Server, e *events.MessageCreate) {
	if manager.FindUserVoiceState(e.Client(), *e.GuildID, e.Message.Author.ID) == nil {
		sendMsg(e, embed.Error("❌ Error", constants.NotInVC), 5*time.Second)
		return
	}

	if srv.IsPlaying() {
		go srv.Clean()
		sendMsg(e, embed.Success("🧹 Queue cleared", constants.QueueCleared), 5*time.Second)
	} else {
		sendMsg(e, embed.Info("📋 Queue", constants.QueueEmpty), 5*time.Second)
	}
}

func pauseMessage(srv *manager.Server, e *events.MessageCreate, pause bool) {
	if manager.FindUserVoiceState(e.Client(), *e.GuildID, e.Message.Author.ID) == nil {
		sendMsg(e, embed.Error("❌ Error", constants.NotInVC), 5*time.Second)
		return
	}

	if !srv.IsPlaying() {
		sendMsg(e, embed.Info("⏸ Pause", constants.QueueEmpty), 5*time.Second)
		return
	}

	if pause {
		if srv.PausePlayback() {
			sendMsg(e, embed.Success("⏸ Paused", constants.Paused), 5*time.Second)
		} else {
			sendMsg(e, embed.Warning("⏸ Pause", constants.AlreadyPaused), 5*time.Second)
		}
	} else {
		if srv.ResumePlayback() {
			sendMsg(e, embed.Success("▶️ Resumed", constants.Resumed), 5*time.Second)
		} else {
			sendMsg(e, embed.Warning("▶️ Resume", constants.AlreadyResumed), 5*time.Second)
		}
	}
}

func queueMessage(srv *manager.Server, e *events.MessageCreate) {
	const maxQueue = 10

	if !srv.IsPlaying() {
		sendMsg(e, embed.Info("📋 Queue", constants.QueueEmpty), 5*time.Second)
		return
	}

	el := srv.Queue.GetAllQueue()
	if len(el) == 0 {
		sendMsg(e, embed.Info("📋 Queue", constants.QueueEmpty), 5*time.Second)
		return
	}

	builder := styled(func(b discord.Embed) discord.Embed {
		return b.WithTitle("📋 Queue").WithColor(embed.ColorBlurple).AddField("1", fmt.Sprintf("[%s](%s) - %s/%s added by %s\n", el[0].Title, el[0].Link,
			manager.FormatDuration(float64(srv.Frames.Load())/constants.FrameSeconds), el[0].Duration, el[0].User), false)
	})

	for j := 1; j < len(el) && j < maxQueue; j++ {
		builder = builder.AddField(strconv.Itoa(j+1), fmt.Sprintf("[%s](%s) - %s added by %s\n", el[j].Title, el[j].Link, el[j].Duration, el[j].User), false)
	}

	if len(el) > maxQueue {
		builder = builder.AddField("...", "And "+strconv.Itoa(len(el)-maxQueue)+" more", false)
	}

	sendMsg(e, builder, 20*time.Second)
}

func autoplayMessage(srv *manager.Server, e *events.MessageCreate) {
	next := !srv.Autoplay.Load()
	srv.Autoplay.Store(next)
	status := "disabled"
	if next {
		status = "enabled"
	}
	sendMsg(e, embed.Info("🔁 Autoplay", "Autoplay "+status), 5*time.Second)
}

func loopMessage(srv *manager.Server, e *events.MessageCreate) {
	next := !srv.LoopAll.Load()
	srv.LoopAll.Store(next)
	status := "disabled"
	if next {
		status = "enabled"
	}
	sendMsg(e, embed.Info("🔁 Loop", "Loop "+status), 5*time.Second)
}

func joinMessage(srv *manager.Server, e *events.MessageCreate) {
	vs := manager.FindUserVoiceState(e.Client(), *e.GuildID, e.Message.Author.ID)
	if vs == nil {
		sendMsg(e, embed.Error("❌ Error", constants.NotInVC), 5*time.Second)
		return
	}

	if err := srv.EnsureVoice(*vs.ChannelID); err != nil {
		msg := constants.CantJoinVC
		if err == vc.ErrAlreadyConnected {
			msg = constants.BotInAnotherVC
		}
		sendMsg(e, embed.Error("❌ Error", msg), 5*time.Second)
		return
	}

	sendMsg(e, embed.Success("🎧 Joined", "Joined the voice channel"), 5*time.Second)
}

func forwardMessage(srv *manager.Server, e *events.MessageCreate) {
	if !srv.IsPlaying() {
		sendMsg(e, embed.Error("❌ Error", constants.NothingPlaying), 5*time.Second)
		return
	}

	target := int(srv.Frames.Load()) + int(math.Trunc(10*constants.FrameSeconds))
	if !srv.SeekFrame(target) {
		sendMsg(e, embed.Error("❌ Error", constants.GotoInvalid), 5*time.Second)
		return
	}
	sendMsg(e, embed.Info("⏩ Forward", "Forwarded 10 seconds"), 5*time.Second)
}

func backwardMessage(srv *manager.Server, e *events.MessageCreate) {
	if !srv.IsPlaying() {
		sendMsg(e, embed.Error("❌ Error", constants.NothingPlaying), 5*time.Second)
		return
	}

	select {
	case srv.Restart <- struct{}{}:
	default:
	}
	sendMsg(e, embed.Info("⏪ Backward", "Restarted the current song"), 5*time.Second)
}

func nowPlayingMessage(srv *manager.Server, e *events.MessageCreate) {
	if !srv.IsPlaying() {
		sendMsg(e, embed.Error("❌ Error", constants.NothingPlaying), 5*time.Second)
		return
	}

	el := srv.Queue.GetFirstElement()
	if el == nil {
		sendMsg(e, embed.Info("📋 Queue", constants.QueueEmpty), 5*time.Second)
		return
	}

	sendMsg(e, styled(func(b discord.Embed) discord.Embed {
		return b.WithTitle("🎵 Now Playing").AddField("", fmt.Sprintf("[%s](%s) - %s/%s\nRequested by %s", el.Title, el.Link,
			manager.FormatDuration(float64(srv.Frames.Load())/constants.FrameSeconds), el.Duration, el.User), false).
			WithColor(embed.ColorBlurple).WithThumbnail(el.Thumbnail)
	}), 10*time.Second)
}

func previousMessage(srv *manager.Server, e *events.MessageCreate) {
	prev := srv.LastPlayed()
	if prev == nil {
		sendMsg(e, embed.Error("❌ Error", "No previous song"), 5*time.Second)
		return
	}

	vs := manager.FindUserVoiceState(e.Client(), *e.GuildID, e.Message.Author.ID)
	if vs == nil {
		sendMsg(e, embed.Error("❌ Error", constants.NotInVC), 5*time.Second)
		return
	}

	if err := srv.EnsureVoice(*vs.ChannelID); err != nil {
		sendMsg(e, embed.Error("❌ Error", constants.CantJoinVC), 5*time.Second)
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
		sendMsg(e, embed.Error("❌ Error", "Usage: "+prefix+"seek <seconds|±seconds>"), 5*time.Second)
		return
	}

	if !srv.IsPlaying() {
		sendMsg(e, embed.Error("❌ Error", constants.NothingPlaying), 5*time.Second)
		return
	}

	d, err := time.ParseDuration(args[0])
	if err != nil {
		sendMsg(e, embed.Error("❌ Error", constants.GotoInvalid), 5*time.Second)
		return
	}

	current := int(srv.Frames.Load())
	target := int(d.Seconds() * constants.FrameSeconds)
	if strings.HasPrefix(args[0], "+") || strings.HasPrefix(args[0], "-") {
		target = current + target
	} else if target <= current {
		sendMsg(e, embed.Error("❌ Error", constants.GotoBackwards), 5*time.Second)
		return
	}

	if !srv.SeekFrame(target) {
		sendMsg(e, embed.Error("❌ Error", constants.GotoInvalid), 5*time.Second)
		return
	}
	sendMsg(e, embed.Info("⏩ Goto", constants.SkippedTo+args[0]), 5*time.Second)
}

func moveMessage(srv *manager.Server, e *events.MessageCreate, args []string) {
	if len(args) != 2 {
		sendMsg(e, embed.Error("❌ Error", "Usage: "+prefix+"move <from> <to>"), 5*time.Second)
		return
	}

	from, err1 := strconv.Atoi(args[0])
	to, err2 := strconv.Atoi(args[1])
	if err1 != nil || err2 != nil || from < 2 {
		sendMsg(e, embed.Error("❌ Error", "Positions start at 1 (the current song is 1)"), 5*time.Second)
		return
	}

	if err := srv.Queue.Move(from-1, to-1); err != nil {
		sendMsg(e, embed.Error("❌ Error", "Position out of range"), 5*time.Second)
		return
	}
	sendMsg(e, embed.Success("✅ Success", "Song moved"), 5*time.Second)
}

func removeMessage(srv *manager.Server, e *events.MessageCreate, args []string) {
	if len(args) != 1 {
		sendMsg(e, embed.Error("❌ Error", "Usage: "+prefix+"remove <position>"), 5*time.Second)
		return
	}

	n, err := strconv.Atoi(args[0])
	if err != nil || n < 1 {
		sendMsg(e, embed.Error("❌ Error", "Invalid position"), 5*time.Second)
		return
	}

	queue := srv.Queue.GetAllQueue()
	if n >= len(queue) {
		sendMsg(e, embed.Error("❌ Error", "Position out of range"), 5*time.Second)
		return
	}

	title := queue[n].Title
	if err := srv.Queue.RemoveAt(n); err != nil {
		sendMsg(e, embed.Error("❌ Error", err.Error()), 5*time.Second)
		return
	}
	sendMsg(e, embed.Success("🗑 Removed", title), 5*time.Second)
}

func shuffleMessage(srv *manager.Server, e *events.MessageCreate) {
	srv.Queue.ShuffleRemaining()
	sendMsg(e, embed.Info("🔀 Shuffle", "Queue shuffled"), 5*time.Second)
}

func skiptoMessage(srv *manager.Server, e *events.MessageCreate, args []string) {
	if len(args) != 1 {
		sendMsg(e, embed.Error("❌ Error", "Usage: "+prefix+"skipto <position>"), 5*time.Second)
		return
	}

	n, err := strconv.Atoi(args[0])
	if err != nil || n < 1 {
		sendMsg(e, embed.Error("❌ Error", "Invalid position"), 5*time.Second)
		return
	}

	queue := srv.Queue.GetAllQueue()
	if n > len(queue) {
		sendMsg(e, embed.Error("❌ Error", "Position out of range"), 5*time.Second)
		return
	}

	for i := 1; i < n; i++ {
		_ = srv.Queue.RemoveAt(1)
	}
	srv.SkipSong(manager.Skip)
	srv.Paused.Store(false)
	sendMsg(e, embed.Info("⏭ Skipped", fmt.Sprintf("Skipped to %d", n)), 5*time.Second)
}

func volumeMessage(srv *manager.Server, e *events.MessageCreate, args []string) {
	if len(args) != 1 {
		sendMsg(e, embed.Error("❌ Error", "Usage: "+prefix+"volume <0-200>"), 5*time.Second)
		return
	}

	v, err := strconv.Atoi(args[0])
	if err != nil || v < 0 || v > 200 {
		sendMsg(e, embed.Error("❌ Error", "Volume must be between 0 and 200"), 5*time.Second)
		return
	}

	srv.Volume.Store(int32(v))
	sendMsg(e, embed.Success("🔊 Volume", fmt.Sprintf("Volume set to %d%%", v)), 5*time.Second)
}

func lyricsMessage(srv *manager.Server, e *events.MessageCreate) {
	if !srv.IsPlaying() {
		sendMsg(e, embed.Error("❌ Error", constants.NothingPlaying), 5*time.Second)
		return
	}

	el := srv.Queue.GetFirstElement()
	if el == nil {
		return
	}

	lyrics, err := fetchLyrics(el.Title)
	if err != nil {
		sendMsg(e, embed.Error("❌ Error", err.Error()), 10*time.Second)
		return
	}

	if len(lyrics) > 3900 {
		lyrics = lyrics[:3900] + "..."
	}
	sendMsg(e, styled(func(b discord.Embed) discord.Embed {
		return b.WithTitle("📜 Lyrics").WithDescription(lyrics).WithColor(embed.ColorBlurple)
	}), 30*time.Second)
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
		sendMsg(e, embed.Error("❌ Error", "Usage: "+prefix+"pl-create <name>"), 5*time.Second)
		return
	}

	if err := clients.Database.CreatePlaylist(*e.GuildID, args[0]); err != nil {
		sendMsg(e, embed.Error("❌ Error", "Playlist already exists"), 5*time.Second)
		return
	}
	sendMsg(e, embed.Success("✅ Playlist created", args[0]), 5*time.Second)
}

func playlistDeleteMessage(srv *manager.Server, e *events.MessageCreate, args []string) {
	if len(args) != 1 {
		sendMsg(e, embed.Error("❌ Error", "Usage: "+prefix+"pl-delete <name>"), 5*time.Second)
		return
	}

	if err := clients.Database.DeletePlaylist(*e.GuildID, args[0]); err != nil {
		sendMsg(e, embed.Error("❌ Error", err.Error()), 5*time.Second)
		return
	}
	sendMsg(e, embed.Success("✅ Playlist deleted", args[0]), 5*time.Second)
}

func playlistRenameMessage(srv *manager.Server, e *events.MessageCreate, args []string) {
	if len(args) != 2 {
		sendMsg(e, embed.Error("❌ Error", "Usage: "+prefix+"pl-rename <old> <new>"), 5*time.Second)
		return
	}

	if err := clients.Database.RenamePlaylist(*e.GuildID, args[0], args[1]); err != nil {
		sendMsg(e, embed.Error("❌ Error", err.Error()), 5*time.Second)
		return
	}
	sendMsg(e, embed.Success("✅ Playlist renamed", fmt.Sprintf("%s → %s", args[0], args[1])), 5*time.Second)
}

func playlistListMessage(srv *manager.Server, e *events.MessageCreate) {
	playlists := clients.Database.ListPlaylists(*e.GuildID)
	if len(playlists) == 0 {
		sendMsg(e, embed.Info("🎼 Playlists", "No playlists yet. Use "+prefix+"pl-create <name>"), 10*time.Second)
		return
	}

	var b strings.Builder
	for _, p := range playlists {
		fmt.Fprintf(&b, "%s (%d songs)\n", p.Name, p.Count)
	}
	sendMsg(e, embed.Info("🎼 Playlists", b.String()), 15*time.Second)
}

func playlistInfoMessage(srv *manager.Server, e *events.MessageCreate, args []string) {
	if len(args) != 1 {
		sendMsg(e, embed.Error("❌ Error", "Usage: "+prefix+"pl-info <name>"), 5*time.Second)
		return
	}

	entries := clients.Database.GetPlaylistEntries(*e.GuildID, args[0])
	if len(entries) == 0 {
		sendMsg(e, embed.Error("❌ Error", "Playlist is empty or does not exist"), 5*time.Second)
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
	sendMsg(e, embed.Info("🎼 "+args[0], b.String()), 20*time.Second)
}

func playlistPlayMessage(srv *manager.Server, e *events.MessageCreate, args []string, forceShuffle bool) {
	if len(args) == 0 {
		sendMsg(e, embed.Error("❌ Error", "Usage: "+prefix+"pl-play <name> [-shuffle] [-loop] [-priority]"), 5*time.Second)
		return
	}

	clean, flags := parsePlayFlags(args[1:])
	name := args[0]
	if len(clean) > 0 {
		name = clean[0]
	}

	entries := clients.Database.GetPlaylistEntries(*e.GuildID, name)
	if len(entries) == 0 {
		sendMsg(e, embed.Error("❌ Error", "Playlist is empty or does not exist"), 5*time.Second)
		return
	}

	if djBlocked(srv, e) {
		sendMsg(e, embed.Error("❌ Error", constants.DjNot), 5*time.Second)
		return
	}

	vs := manager.FindUserVoiceState(e.Client(), *e.GuildID, e.Message.Author.ID)
	if vs == nil {
		sendMsg(e, embed.Error("❌ Error", constants.NotInVC), 5*time.Second)
		return
	}

	if err := srv.EnsureVoice(*vs.ChannelID); err != nil {
		sendMsg(e, embed.Error("❌ Error", constants.CantJoinVC), 5*time.Second)
		return
	}

	if forceShuffle || flags.random {
		rand.Shuffle(len(entries), func(i, j int) {
			entries[i], entries[j] = entries[j], entries[i]
		})
	}

	sendMsg(e, embed.Success("✅ Enqueued", fmt.Sprintf("Enqueued %d songs from %s", len(entries), name)), 5*time.Second)

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
		sendMsg(e, embed.Error("❌ Error", "Usage: "+prefix+"pl-removedupe <name>"), 5*time.Second)
		return
	}

	if err := clients.Database.RemoveDuplicates(*e.GuildID, args[0]); err != nil {
		sendMsg(e, embed.Error("❌ Error", err.Error()), 5*time.Second)
		return
	}
	sendMsg(e, embed.Success("✅ Duplicates removed", args[0]), 5*time.Second)
}

func playlistRemoveTrackMessage(srv *manager.Server, e *events.MessageCreate, args []string) {
	if len(args) != 2 {
		sendMsg(e, embed.Error("❌ Error", "Usage: "+prefix+"pl-removetrack <name> <position>"), 5*time.Second)
		return
	}

	pos, err := strconv.Atoi(args[1])
	if err != nil || pos < 1 {
		sendMsg(e, embed.Error("❌ Error", "Invalid position"), 5*time.Second)
		return
	}

	if err := clients.Database.RemoveFromPlaylist(*e.GuildID, args[0], pos); err != nil {
		sendMsg(e, embed.Error("❌ Error", err.Error()), 5*time.Second)
		return
	}
	sendMsg(e, embed.Success("✅ Track removed", fmt.Sprintf("%s #%d", args[0], pos)), 5*time.Second)
}

func playlistSaveMessage(srv *manager.Server, e *events.MessageCreate, args []string, saveQueue bool) {
	if len(args) != 1 {
		usage := "Usage: " + prefix + "pl-savecurrent <name>"
		if saveQueue {
			usage = "Usage: " + prefix + "pl-savequeue <name>"
		}
		sendMsg(e, embed.Error("❌ Error", usage), 5*time.Second)
		return
	}

	var entries []string
	if saveQueue {
		queue := srv.Queue.GetAllQueue()
		if len(queue) == 0 {
			sendMsg(e, embed.Error("❌ Error", constants.QueueEmpty), 5*time.Second)
			return
		}
		entries = make([]string, 0, len(queue))
		for _, el := range queue {
			entries = append(entries, el.Link)
		}
	} else {
		el := srv.Queue.GetFirstElement()
		if el == nil {
			sendMsg(e, embed.Error("❌ Error", constants.NothingPlaying), 5*time.Second)
			return
		}
		entries = []string{el.Link}
	}

	if err := clients.Database.SavePlaylist(*e.GuildID, args[0], entries); err != nil {
		sendMsg(e, embed.Error("❌ Error", err.Error()), 5*time.Second)
		return
	}
	sendMsg(e, embed.Success("✅ Saved", fmt.Sprintf("Saved %d songs to %s", len(entries), args[0])), 5*time.Second)
}

func disconnectMessage(srv *manager.Server, e *events.MessageCreate) {
	if manager.FindUserVoiceState(e.Client(), *e.GuildID, e.Message.Author.ID) == nil {
		sendMsg(e, embed.Error("❌ Error", constants.NotInVC), 5*time.Second)
		return
	}

	// Stop playback and clear the queue, then leave the voice channel.
	go srv.Clean()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	srv.VC.Disconnect(ctx)
	cancel()

	sendMsg(e, embed.Success("👋 Disconnected", constants.Disconnected), 5*time.Second)
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
