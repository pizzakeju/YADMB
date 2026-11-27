# Installation

## Natively

First thing, we need to install some packages

```Bash
sudo apt install build-essential golang-go git yt-dlp ffmpeg libopus-dev -y
```

After installation is done, we can clone the repo with

```Bash
git clone https://github.com/TheTipo01/YADMB
```

Enter the directory, and build the bot

```Bash
cd GRUPI
go build
```

libdave is now required, you can use the script over at [disgoorg/godave](https://github.com/disgoorg/godave/) to quickly install the required libraries
```Bash
wget https://raw.githubusercontent.com/disgoorg/godave/refs/heads/master/scripts/libdave_install.sh
chmod +x libdave_install.sh
./libdave_install.sh v1.3.0
```

Final things:

- make sure your PKG_CONFIG_PATH has been correctly updated with the new libdave library
- modify the `example_config.yml`, adding all required tokens and renaming it to `config.yml`
- for info about creating and adding the bot, see the following [page](hosting.md)

## Docker

- Clone the repo
- Modify the `example_config.yml`, by adding your discord bot token
  (see [here](hosting.md) if you don't know how to do it)
- Rename it in `config.yml` and move it in the `data` directory
- Run `docker-compose up -d`
- Enjoy your GRUPI instance!

Note: the docker image is available
on [Docker hub](https://hub.docker.com/r/thetipo01/yadmb), [Quay.io](https://quay.io/repository/thetipo01/yadmb)
and [Github packages](https://github.com/TheTipo01/YADMB/pkgs/container/yadmb).

## Configuration with environment variables

Instead of (or in addition to) `config.yml`, every setting can be provided as
an environment variable. This is the recommended way on Docker and platforms
like Dokploy, where you set variables in the UI and there is no config file.
Environment variables override `config.yml` values when both are present.

loaded automatically, so you can fill it and run the bot without touching the
environment.

| Variable | Description |
| --- | --- |
| `TOKEN` | Discord bot token (required) |
| `OWNERS` | Comma-separated list of bot owner IDs |
| `SPOTIFY_CLIENT_ID` | Spotify client ID, enables Spotify playlists |
| `SPOTIFY_CLIENT_SECRET` | Spotify client secret |
| `YOUTUBE_API_KEY` | YouTube Data API v3 key (optional, recommended) |
| `WHITELIST` | `true` to only respond to the guilds in `GUILD_LIST` |
| `GUILD_LIST` | Comma-separated list of guild IDs |
| `HTTP_ADDRESS` | Web server address, e.g. `:8080` or `unix:///tmp/yadmb.sock` (empty disables the API) |
| `ORIGIN` | Allowed origin for CORS and web UI links. Empty or `*` allows any origin |
| `API_TOKENS` | JSON array of long-lived API tokens, e.g. `[{"userid":"123","token":"correct-horse-battery-staple","textchannelid":"456","guildid":"789"}]` |
| `DATABASE_DSN` | Data source name, e.g. `./data/yadmb.db` (sqlite) or `user:pass@tcp(ip:3306)/database` (mysql) |
| `DATABASE_DRIVER` | `sqlite` or `mysql` |
| `LOG_LEVEL` | `error`, `warning`, `informational` or `debug` |

The `docker-compose.yaml` file forwards all of these variables from the host
environment, so on Dokploy you can just add them in the service settings.
