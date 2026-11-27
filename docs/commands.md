# Commands

`/play <link> <shuffle> <loop> <priority>` - Plays a song from YouTube or spotify playlist.
If it's not a valid link, it will insert into the queue the first result for the given query.

- `shuffle` if set to true, it will shuffle the playlist.
- `loop` if set to true, it will loop the playlist.
- `priority` if set to true, it will insert the song at the top of the queue.

`/playlist <link> <shuffle> <loop> <priority>` - Same as play, but accepts YouTube playlist

`/skip` - Skips the currently playing song

`/clear` - Clears the entire queue

`/pause` - Pauses currently playing song

`/resume` - Resumes current song

`/queue` - Prints the currently playing song and the next songs

`/disconnect` - Stops playback and disconnects the bot from the voice channel

`/restart` - Restarts the bot (works only for the bot owner, see `config.yml`)

`/addcustom <custom_command> <song/playlist> <loop>` - Creates a custom command to play a song or playlist. Set `loop`
to true to play the song looped

`/rmcustom <custom_command>` - Removes a custom command from the DB

`/listcustom` - Lists all custom commands for the current server

`/custom <custom_command> <priority>` - Calls a custom command

`/stats` - Stats, like latency, and the size of the local cache

`/goto <time>` - Skips to a given time. Valid formats are: 1h10m3s, 3m, 4m10s...

`/stream <link> <priority>` - Streams a song from the given URL, useful for radios

`/update <link> <info> <song>` - Next time the song `<link>` is played, the bot will (if set to true):

- `<info>` - Update info about the song, like thumbnail, segments from SponsorBlock, title...
- `<song>` - Re-downloads the entire song

`/blacklist <user>` - Adds or remove a person from the bot blacklist

`/dj` - Enables or disable DJ mode for the current server, only users with the DJ role can use the bot

`/djrole <role>` - Sets the DJ role for the current server

`/djroletoggle <user>` - Adds or removes the DJ role from the given user
## Prefix commands

The same core actions are available as classic chat messages prefixed with `-`,
so the bot can be used without slash command autocompletion. Every command has
a shortform in parentheses:

- `-play` / `-p` <link|query> - Plays a song from YouTube or Spotify (or searches the query). Optional flags: `-shuffle`, `-loop`, `-priority`
- `-playlist` / `-pl` <link> [-shuffle] [-loop] [-priority] - Same as `-play`, but accepts playlists
- `-stream` / `-st` <url> - Streams a song from the given URL
- `-skip` / `-s` - Skips the currently playing song
- `-clear` / `-c` (alias `-clearqueue`) - Clears the queue
- `-pause` / `-pa` - Pauses the current song
- `-resume` / `-r` - Resumes the current song
- `-queue` / `-q` - Shows the current queue
- `-disconnect` / `-dc` (alias `-leave`, `-stop`) - Stops playback and leaves the voice channel
- `-autoplay` - Toggles automatically playing similar songs when the queue ends
- `-backward` - Restarts the current song
- `-forward` - Jumps forward 10 seconds
- `-join` - Joins your voice channel without playing
- `-loop` - Toggles looping the current song
- `-lyrics` - Shows lyrics for the current song (from lyrics.ovh)
- `-move <from> <to>` - Moves a queued song to another position
- `-nowplaying` / `-np` - Shows the current song and progress
- `-previous` / `-prev` - Re-plays the last finished/skipped song
- `-remove` / `-rm` <position> - Removes the song at that queue position (1 = first upcoming)
- `-seek <seconds|±seconds>` - Seeks to an absolute position or relative offset
- `-shuffle` / `-sh` - Shuffles the remaining queue
- `-skipto <position>` - Skips ahead to the song at that position
- `-volume` / `-vol` <0-200> - Sets playback volume in percent (100 = normal)
- `-help` / `-h` - Lists the available prefix commands

## Playlist commands (`pl-*`)

User-created playlists are stored per guild. Songs are saved as links, so the
same sources accepted by `-play` work (YouTube, Spotify, generic URLs).

- `-pl-create <name>` - Creates an empty playlist
- `-pl-delete <name>` - Deletes a playlist
- `-pl-rename <old> <new>` - Renames a playlist
- `-pl-list` - Lists all playlists of the guild
- `-pl-info <name>` - Shows the songs in a playlist
- `-pl-play <name> [-shuffle] [-loop] [-priority]` - Plays a playlist
- `-pl-playshuffle <name>` - Plays a playlist shuffled
- `-pl-removedupe <name>` - Removes duplicate entries
- `-pl-removetrack <name> <position>` - Removes one entry by position
- `-pl-savecurrent <name>` - Saves the currently playing song as a playlist
- `-pl-savequeue <name>` - Saves the whole queue as a playlist

Prefix and playlist commands respect the same restrictions as slash commands:
blacklist, whitelist and DJ mode apply. The bot needs the `MESSAGE CONTENT`
intent enabled in the Discord developer portal for `-play <query>` style
parsing.