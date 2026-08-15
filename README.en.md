<div align="center">

# Malody Mallow

**A local music player that lives in your terminal, in the spirit of btop
and lazygit.**

[![version](https://img.shields.io/github/v/tag/kitasael-burakku/Malody-Mallow?sort=semver&label=version&color=blue)](https://github.com/kitasael-burakku/Malody-Mallow/releases)
[![Go](https://img.shields.io/badge/Go-%E2%89%A51.25-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)
[![License](https://img.shields.io/badge/License-GPLv3-blue)](LICENSE)
[![CI](https://github.com/kitasael-burakku/Malody-Mallow/actions/workflows/ci.yml/badge.svg)](https://github.com/kitasael-burakku/Malody-Mallow/actions/workflows/ci.yml)
[![AUR](https://img.shields.io/aur/version/maly?label=AUR&logo=archlinux&logoColor=white)](https://aur.archlinux.org/packages/maly)
[![Platform](https://img.shields.io/badge/Platform-Linux-333333?logo=linux&logoColor=white)](#requirements--compatibility)

🇪🇸 [Español](README.md) · 🇬🇧 [English](README.en.md)

<img src="pictures/tui-main.jpg" alt="Malody Mallow: library, queue, and Now Playing with cover art and lyrics, above the visualizer strip" width="850"/>

</div>

---

## What is Malody Mallow

**Malody Mallow** (`maly`) is a local music player for your own collection,
built to live entirely in the terminal. A single Go binary, no runtime and
no system dependency beyond [mpv](https://mpv.io/):

- A **TUI** with library and queue panels, embedded cover art, synced
  lyrics, and a live spectrum visualizer.
- A **background service** (`maly daemon`) that keeps playing after you
  close the terminal window, with a persistent session.
- An **`mpc`/`playerctl`-style CLI** to control it from any terminal or
  script.
- **MPRIS** desktop integration: `playerctl`, Waybar's `mpris` module, and
  media keys control it with zero configuration.

### Why use it

If you already live in the terminal and reach for tools like btop or
lazygit, Malody Mallow fits the same habit: no desktop app to open just to
play music, and playback doesn't depend on a window staying open. The
library is yours — your files, indexed in a local SQLite database, no
account and no streaming — and the player speaks the same protocols the
rest of your Linux desktop already uses (MPRIS, D-Bus).

### What makes it different

- **True gapless playback**, even with shuffle, repeat-one, or skipping a
  corrupted file — not just "no audible gap," but the next track already
  loaded into mpv before the current one ends.
- **The session survives**: queue, volume, shuffle/repeat, and playback
  position are restored if you restart the service.
- **Cover art as a real image** on terminals with kitty's graphics
  protocol (auto-detected), with automatic half-block fallback on any
  other truecolor terminal.
- **Bilingual by default** (English/Spanish), auto-detecting your system
  language on first run.
- **`maly get`**: downloads audio with yt-dlp straight into your library
  and re-indexes it — the same lazygit-style philosophy of coordinating
  external tools instead of reimplementing them.

---

## Highlights

- Plays MP3, FLAC, OGG, OPUS, M4A, and WAV via mpv.
- Service + client: music keeps playing even if you close the TUI.
- SQLite library with accent- and case-insensitive search.
- Live spectrum visualizer (FFT over the PipeWire/PulseAudio monitor).
- "Now Playing" screen with cover art, synced lyrics (`.lrc` or embedded), and visualizer.
- Playlists, fuzzy-search song picker, and an integrated command palette.
- Dynamic shell completion (bash/fish/zsh): completes commands, real track titles, playlists, and queue positions.

---

## Screenshots

<table>
<tr>
<td width="50%">
<img src="pictures/now-playing.png" alt="Now Playing screen with cover art and synced lyrics" width="100%"/>
<sub align="center"><b>Now Playing</b> — cover art, lyrics, and visualizer</sub>
</td>
<td width="50%">
<img src="pictures/command-palette.jpg" alt="Command palette (ctrl+p)" width="100%"/>
<sub align="center"><b>Command palette</b> — built-in console (ctrl+p)</sub>
</td>
</tr>
<tr>
<td width="50%">
<img src="pictures/song-picker.jpg" alt="Fuzzy-search song picker" width="100%"/>
<sub align="center"><b>Song picker</b> — fuzzy search (ctrl+o)</sub>
</td>
<td width="50%">
<img src="pictures/desktop-hyprland.png" alt="Malody Mallow integrated into a Hyprland desktop with Waybar" width="100%"/>
<sub align="center"><b>Desktop integration</b> — Waybar + MPRIS on Hyprland</sub>
</td>
</tr>
</table>

---

## Table of contents

- [What is Malody Mallow](#what-is-malody-mallow)
- [Highlights](#highlights)
- [Screenshots](#screenshots)
- [Where to start](#where-to-start)
- [Features](#features)
- [Architecture](#architecture)
- [Requirements & compatibility](#requirements--compatibility)
- [Installation](#installation)
- [First run](#first-run)
- [Usage](#usage)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)
- [Project structure](#project-structure)
- [Development](#development)
- [Project status](#project-status)
- [Credits](#credits)
- [License](#license)

---

## Where to start

| If you are... | Start with... |
|---|---|
| A new user who wants to try it now | [Installation](#installation) → [First run](#first-run) |
| Coming from mpc/playerctl | [CLI](#cli-mpc-style) |
| Integrating with your desktop (Waybar, media keys) | [MPRIS](#mpris-playerctl-waybar) |
| Something isn't working | [Troubleshooting](#troubleshooting) — or just run `maly doctor` |
| Trying to understand the internals | [Architecture](#architecture) |
| Building or contributing | [Development](#development) |

---

## Features

### Playback

- **mpv backend**: MP3, FLAC, OGG, OPUS, M4A, WAV, with no fuss.
- **Gapless**: the next queued track is appended to mpv ahead of time, so
  the switch happens without cutting the audio — also with repeat-one,
  with shuffle, and when skipping corrupted files.
- **Service + client**: music keeps playing even if you close the TUI (if
  you launched `maly daemon` separately, or left the systemd service
  running). Control it from any terminal.
- **The session survives**: queue, volume, shuffle/repeat, and the current
  track with its position are restored when the service restarts — paused,
  ready to resume with `maly play`.

### Library

- **Local SQLite**: tag scanning (artist/album/title/year/genre),
  accent- and case-insensitive search ("aurea" matches "Áurea").
- **Durations via ffprobe** (optional): scanning has a second phase that
  fills in missing durations in parallel; without ffprobe they're learned
  as each track plays.
- **Playlists**: create, list, add/remove tracks, play, export/import
  M3U — from the CLI, the console, or the `ctrl+l` panel.

### TUI

- **Three-column layout** that adapts to the terminal width: library (fixed
  width), queue (elastic — it takes all the leftover space), and "Now
  Playing" with cover art and synced lyrics. Below 120 columns, "Now
  Playing" collapses into the footer bar; below 90, a single column remains
  and `tab` cycles between library and queue.
- **Library and queue panels** with vim navigation (`h j k l`, `gg`, `G`,
  `ctrl+d`/`ctrl+u`; arrow keys also work) and control presets
  (`maly controls` → `default` | `vim`).
- **Sub-character progress bar**: the head advances in eighths of a cell, so
  playback progress moves continuously instead of jumping column to column.
- **Clean titles**: the suffixes yt-dlp leaves behind (`(Official Video)`,
  `[Lyric Video]`, `(Video Oficial)`…) and the artist repeated inside the
  title are hidden **for display only**. The file's tag is untouched, so
  `maly search` still finds tracks by their original title.
- **"Now Playing" screen (`ctrl+t`)**: a fullscreen view with the embedded
  cover art rendered in the terminal, lyrics synced to playback (a `.lrc`
  sidecar next to the audio file, or embedded in the track), and the
  visualizer strip.
- **`ctrl+p` palette**: an integrated command console (`maly next`,
  `vol +5`, `status`, `get`, `playlist`…) with output inside the palette
  itself.
- **`ctrl+o` picker / `maly select`**: fuzzy search across the whole
  library (`enter` plays, `tab` adds to the queue).
- **`ctrl+l` playlist panel**: manage your playlists without leaving the
  TUI, and `A` sends the current library or queue selection to one.
- **Bilingual**: English/Spanish interface; chosen on first run.

### Visualizer

- **Live FFT** over the PipeWire/PulseAudio audio monitor, with a color
  gradient; bars follow smoothed amplitude (CAVA-style).
- Without a capture backend, it falls back to **animation** instead of
  failing, retrying real capture every 15 seconds.

### Downloads (`maly get`)

- Wraps **yt-dlp** to pull audio straight into your library (MP3 with
  embedded metadata and cover art) and re-scans automatically.
- `maly get playlist <url> [name]` downloads an entire playlist into a
  subdirectory, with `--no-playlist` as the default behavior for plain
  URLs (so you don't accidentally drag a whole playlist along).
- Browser-cookie support (`[ytdlp] cookies_from_browser`) for content that
  requires an account.

### Desktop integration (MPRIS)

- The service announces itself as `org.mpris.MediaPlayer2.maly` on D-Bus —
  `playerctl`, Waybar's `mpris` module, and desktop media keys see and
  control it with zero configuration.
- The track's embedded cover art is published as `mpris:artUrl`.

### Shell

- **Dynamic completion** (bash/fish/zsh): TAB completes commands, real
  titles from your library, playlist names, and queue positions.

---

## Architecture

Daemon + clients talking over a Unix socket with line-delimited JSON. The
daemon is the sole owner of mpv, the queue, and the library; the CLI and
TUI are clients that talk to it over IPC. If no daemon is running, **the
TUI embeds one in its own process** and it dies with the TUI.

```text
   maly (TUI)         maly next · status        playerctl · Waybar
   maly select         maly play · vol           media keys
        │                     │                          │
        └──── Unix socket, line-delimited JSON ┐          │ D-Bus
                                                ▼          ▼ (session)
                        ┌───────────────────────────────────┐
                        │            maly daemon             │
                        │   queue · session · library · MPRIS│
                        └──────┬───────────┬────────────┬────┘
                               │           │             │
                          IPC (JSON)   SQLite (WAL)  pw-record / parec
                               ▼           ▼             ▼
                             mpv       library.db    system audio
                        (playback)   (tags, playlists) monitor (FFT)
```

| Component | Role |
|---|---|
| `cmd/maly` | CLI and TUI entry point; `commands.go` is the single command table (dispatch, help, and completions) |
| `internal/daemon` | Owns mpv, the queue, and the library; flock-based startup to claim identity, Unix-socket IPC, session persisted as JSON |
| `internal/player` | mpv wrapper over its own IPC socket; gapless via a two-track window |
| `internal/queue` | Queue with permutation-based shuffle and repeat |
| `internal/library` | SQLite (modernc, no CGo) — tags, search, playlists |
| `internal/mpris` | MPRIS2 integration over the session D-Bus (godbus) |
| `internal/viz` | Audio capture (pw-record/parec) + FFT for the visualizer |
| `internal/tui` | Bubble Tea interface: panels, console, pickers, "Now Playing" |
| `internal/ipc` | The socket's Request/Response protocol, shared by the CLI, TUI, and daemon |

---

## Requirements & compatibility

### Required

| Requirement | For |
|---|---|
| **Linux** | mpv + MPRIS/D-Bus + PipeWire/PulseAudio are the project's model; unverified on other operating systems |
| **[mpv](https://mpv.io/)** | audio engine — without it, the daemon and TUI won't start |

### To build from source

| Requirement | Detail |
|---|---|
| **Go ≥ 1.25** | the module is named plain `maly` (not `github.com/…`), so **`go install` doesn't work** — you need to clone and build |
| **git** | to clone the repository |

### Optional dependencies

Malody Mallow starts and plays with just mpv. Everything else degrades
silently and never breaks startup:

| Tool | Enables | Without it |
|---|---|---|
| **ffprobe** (from ffmpeg) | durations in the second scan phase | learned as each track plays instead; `maly doctor` flags it as info |
| **yt-dlp** + **ffmpeg** | `maly get` | the command fails with install hints; nothing else is affected |
| **pw-record** (PipeWire) or **parec** (PulseAudio) | visualizer with real system audio | animation mode + a one-time warning; retries real capture every 15s, but **only if it ever worked at least once** |
| **D-Bus session bus** | MPRIS: playerctl, Waybar, media keys | one stderr line on startup; the rest of the daemon runs normally |
| **git** | `maly update` and the new-release check | explicit error when applying an update; the TUI's background check just stays quiet |
| **curl** | applying an update via `maly update` | error with the installer URL so you can do it by hand |

`maly doctor` is the automated diagnostic for this entire table — check it
before filing a bug.

### Terminal capabilities

| Capability | Requires | Without it |
|---|---|---|
| Cover art as a real image | a kitty terminal (`KITTY_WINDOW_ID` or `TERM` containing `kitty`), **outside tmux** | drawn as half-blocks (`▀`) |
| Cover art as half-blocks | a truecolor terminal | no other color tier — no 256-color or monochrome mode |
| Under tmux | — | always half-blocks, even if the terminal behind it is kitty |

### Audio: PipeWire and PulseAudio

The visualizer captures the **monitor** of the default output sink —
literally whatever audio is playing on the system, not just maly's.
`pw-record` is tried first, `parec` second (`[visualizer] backend` in the
config forces one or the other if you have both and the automatic pick is
worse); `parec` works both on plain PulseAudio and on PipeWire via
`pipewire-pulse`.

### Download stack: yt-dlp and ffmpeg

`maly get` wraps yt-dlp — maly never talks to any website directly.
`ffmpeg` is used by yt-dlp to extract and convert to MP3; `ffprobe` (part
of the same package) is used by maly to read durations. On distros that
ship an outdated yt-dlp (Debian/Ubuntu), the installer resolves it via
`pipx` instead.

### Desktop integration: MPRIS and D-Bus

MPRIS2 runs over the D-Bus session bus; without it, there's no integration
with `playerctl`, Waybar's `mpris` module, or media keys, but the daemon
otherwise works exactly the same.

---

## Installation

| Method | Best for | Updates with | Also installs |
|---|---|---|---|
| **Mallow Install** | most users | `maly update` | shell completions + systemd service (asks first) |
| **AUR** (`maly`) | Arch Linux / CachyOS | your AUR helper | completions + systemd service (not enabled) |
| **Building by hand** | developers | `git pull` + `make build` | nothing else |

### Quick: Mallow Install (any distro)

```sh
curl -fsSL https://raw.githubusercontent.com/kitasael-burakku/Malody-Mallow/main/mallow-install.sh | sh
```

An interactive, screen-by-screen wizard: action (install/update/
uninstall), scope (user in `~/.local/bin`, or system in `/usr/local` with
sudo), source (latest stable tag by default, or `main` for the development
branch), and an optional-dependencies checklist. It auto-detects Go if
already installed with a sufficient version; otherwise it offers to
download the official one from go.dev into `~/.cache/mallow/go`, verifying
its published SHA-256 first.

Non-interactive flags are also supported:

```sh
./mallow-install.sh --install [--system]      # install
./mallow-install.sh --update                  # rebuild and reinstall
./mallow-install.sh --uninstall               # uninstall
./mallow-install.sh --ref=v1.13.0             # pin an exact tag/branch
```

`--ref=` takes priority over everything else — it's what `maly update` uses
internally to reinstall the exact announced tag.

### Arch Linux (AUR)

```sh
yay -S maly
# or
paru -S maly
```

Builds from the latest stable tag, with no CGo (SQLite is
[modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite), pure Go).
Depends only on `mpv`; yt-dlp, ffmpeg, pipewire, and pulseaudio are
`optdepends`. It also installs a `--user` systemd unit, **not enabled** —
a package shouldn't start services on its own.

See [`aur.archlinux.org/packages/maly`](https://aur.archlinux.org/packages/maly).

> **Note:** if you've ever used the installer and also have the AUR
> package, you'll end up with two binaries and two systemd units.
> `maly update` detects a packaged binary (via `/usr/bin` or the channel
> baked in at build time) and defers to the package manager instead of
> overwriting it — but it's best to pick one.

### By hand

```sh
git clone https://github.com/kitasael-burakku/Malody-Mallow.git
cd Malody-Mallow
make build              # equivalent to: go build -o maly ./cmd/maly
make install             # installs to ~/.local/bin/maly
```

`go build ./...` does **not** regenerate the `./maly` binary at the repo
root — it builds each package into its own cache without leaving anything
behind; that's why the `Makefile` uses `-o maly ./cmd/maly` explicitly. If
you'd rather skip `make`, the same two commands work directly:

```sh
go build -o maly ./cmd/maly
install -Dm755 maly ~/.local/bin/maly
```

<details>
<summary>Build dependencies per distro</summary>

```sh
# Arch / CachyOS
sudo pacman -S go git mpv

# Ubuntu / Debian
sudo apt install golang-go git mpv

# Fedora
sudo dnf install golang git mpv

# openSUSE
sudo zypper install go git mpv

# Void
sudo xbps-install go git mpv
```

Check your Go version with `go version` — you need ≥ 1.25. On distros
shipping an older Go, use [go.dev/dl/](https://go.dev/dl/) or let
`mallow-install.sh` offer to download one on the side, without touching
the system one.

</details>

### Completion (bash / fish / zsh)

The binary generates the scripts itself:

```sh
maly completions bash > ~/.local/share/bash-completion/completions/maly
maly completions fish > ~/.config/fish/completions/maly.fish
maly completions zsh  > ~/.local/share/zsh/site-functions/_maly
```

`mallow-install.sh` handles this automatically: it installs completions
only for the shells it finds on your `PATH` (in `--system` mode, it
installs all three, like a package would). Completion is **dynamic**: TAB
on `maly play <TAB>` searches real titles from your library rather than a
fixed list — it uses a bounded row limit (not the whole library) so it
stays fast even on collections with tens of thousands of tracks.

---

## First run

The first time you run any subcommand, maly creates
`~/.config/maly/config.toml` with the defaults (see
[Configuration](#configuration)) and detects your system language
(`LC_ALL`/`LC_MESSAGES`/`LANG`) for that session only — it isn't persisted
until you confirm it.

Opening the TUI for the first time (`maly`, no arguments):

1. If no daemon is running, the TUI embeds one in its own process.
2. With `language` unset in the config, a language picker appears
   (English / Español) — saved once you choose.
3. With an empty library, the panel says so explicitly:
   `(library empty — run maly scan)`.

```sh
maly scan      # index your music (music_dir, or a path you pass)
maly           # open the TUI
```

`music_dir` is resolved in this order: the config's `music_dir` key →
`$XDG_MUSIC_DIR` → `XDG_MUSIC_DIR` from `user-dirs.dirs` → `~/Music`.

---

## Usage

### TUI

| Key | Action |
|---|---|
| `space` | play / pause |
| `n` / `p` | next / previous |
| `+` / `-` | volume |
| `←` / `→` | seek |
| `tab` | switch panel |
| `enter` | play selection |
| `a` | add to queue |
| `d` | remove from queue |
| `K` / `J` | move track in the queue (up/down) |
| `/` | filter the active panel |
| `h j k l` | vim navigation |
| `gg` / `G` | jump to top / bottom |
| `ctrl+d` / `ctrl+u` | half-page down / up |
| `s` / `r` | shuffle / repeat |
| `v` | toggle visualizer |
| `ctrl+t` | "Now Playing" screen (cover art, lyrics, visualizer) |
| `ctrl+p` | command palette (built-in console) |
| `ctrl+o` | song picker (fuzzy search) |
| `ctrl+l` | playlist panel |
| `A` | send current selection to a playlist |
| `?` | full help (every key, including remapped ones) |
| `q` | quit |

`maly controls vim` changes three keys: `remove→x`, `next→>`, `prev→<` —
the vim navigation above is always active, under any preset.

### CLI (mpc-style)

**Playback** — require a running daemon or an open TUI:

| Command | Does |
|---|---|
| `maly play [<query>]` | resume, or search the library and play |
| `maly select` | pick a track with fuzzy search and play it |
| `maly pause` / `toggle` / `stop` | pause / toggle play-pause / stop |
| `maly next` / `prev` | next / previous track |
| `maly jump <pos>` | jump to a queue position |
| `maly move <from> <to>` | move a queue track to another position |
| `maly remove <pos>` | remove a track from the queue |
| `maly add <query\|path>` | add query results or a path to the queue |
| `maly queue` | show the queue |
| `maly clear` | clear the queue |
| `maly status` | show current status |
| `maly vol <0-100\|+N\|-N>` | set or adjust volume |
| `maly seek <+N\|-N\|mm:ss>` | seek within the track |
| `maly shuffle [on\|off]` | toggle or set shuffle |
| `maly repeat [off\|all\|one]` | cycle or set repeat mode |

**Library** — work without the daemon (except `playlist play`):

| Command | Does |
|---|---|
| `maly scan [<path>]` | (re)scan the music library |
| `maly search <query>` | search by title/artist/album |
| `maly get <url\|query>` | download audio with yt-dlp into the library |
| `maly get playlist <url> [name]` | download an entire playlist into a subdirectory |
| `maly playlist <sub> [args]` | manage playlists — see table below |

`maly playlist` subcommands:

| Subcommand | Args | Needs daemon |
|---|---|---|
| `list` | — | no |
| `show <name>` | name | no |
| `create <name>` | name | no |
| `delete <name>` | name (asks for confirmation in an interactive terminal) | no |
| `add <name> <query>` | name, query | no |
| `remove <name> <pos>` | name, position | no |
| `export <name> [file]` | name, optional file (defaults to `<name>.m3u`) | no |
| `import <file> [name]` | file, optional name | no |
| `play <name>` | name | **yes** — the only subcommand that talks to the daemon |

**Other** — no daemon needed:

| Command | Does |
|---|---|
| `maly controls [<preset>]` | show or set the controls preset |
| `maly logo [hex… \| default]` | show or set the banner gradient colors |
| `maly lang [en\|es]`, `-l` | change the interface language |
| `maly info` | show paths, versions, and library size |
| `maly doctor` | check that everything maly needs is in place |
| `maly config` | show the effective configuration (defaults + preset + `[keys]`) |
| `maly update` | check for a new release and update maly |
| `maly kill` | stop the daemon |
| `maly completions <shell>` | print the shell completion script |
| `maly version`, `-v` | show version (and the running daemon's) |
| `maly help`, `-h` | show help |

```sh
maly play luna
maly jump 3
maly move 3 1
maly vol +10
maly seek 1:23
maly shuffle on
maly playlist add favs luna
maly playlist export favs
maly get "aurora runaway"
maly get playlist https://youtube.com/playlist?list=... favs
```

### systemd --user service

To keep maly playing without depending on an open terminal, install the
user service (`mallow-install.sh` offers it automatically in user scope)
or create it by hand:

<details>
<summary>See the full unit (<code>~/.config/systemd/user/maly.service</code>)</summary>

```ini
[Unit]
Description=Maly Music Daemon
StartLimitIntervalSec=30
StartLimitBurst=3

[Service]
Type=simple
ExecStart=%h/.local/bin/maly daemon
Restart=on-failure
RestartSec=2

# Hardening: maly only speaks over Unix sockets (its own IPC, mpv's, and
# the session D-Bus for MPRIS) and needs nothing else. NO ProtectHome:
# music_dir can live outside $HOME (an external drive), and that key
# would block it entirely.
#
# RuntimeDirectory=/ConfigurationDirectory= (no manual ReadWritePaths for
# these two) are the correct way to grant access to $XDG_RUNTIME_DIR/maly
# and $XDG_CONFIG_HOME/maly under ProtectSystem=strict: systemd CREATES
# them before the process starts and they're exempted from the rest being
# read-only, on their own. The first version of this hardening used a
# manual ReadWritePaths=%t/maly, which REQUIRES the path to already exist
# when the mount namespace is set up — it worked warm (the runtime dir
# already existed from a previous run) but broke on a clean boot, with
# /run/user/$UID/maly still empty: "Failed to set up mount namespacing:
# ...: No such file or directory", status=226/NAMESPACE (found in
# production, not in the original verification — the test unit back then
# happened to have a wider parent directory already created, which masked
# it).
#
# $XDG_DATA_HOME (library/session) has no dedicated systemd directive
# (it only covers RUNTIME/CONFIGURATION/STATE/CACHE, and STATE points to
# $XDG_STATE_HOME, not $XDG_DATA_HOME), so it's still handled by hand —
# but with a leading "-", which systemd documents explicitly for this: if
# the path doesn't exist yet, it's skipped instead of aborting startup.
NoNewPrivileges=yes
ProtectSystem=strict
RuntimeDirectory=maly
RuntimeDirectoryMode=0700
ConfigurationDirectory=maly
ConfigurationDirectoryMode=0700
ReadWritePaths=-%h/.local/share/maly
PrivateTmp=yes
RestrictAddressFamilies=AF_UNIX
RestrictNamespaces=yes
LockPersonality=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectKernelLogs=yes
ProtectControlGroups=yes
ProtectClock=yes
ProtectHostname=yes

[Install]
WantedBy=default.target
```

</details>

```sh
systemctl --user daemon-reload
systemctl --user enable --now maly
```

`default.target` (instead of `graphical-session.target`) is intentional:
any `systemd --user` session reaches it, without depending on the
compositor activating it (Hyprland and other minimalist WMs don't, by
default). maly needs nothing graphical — mpv runs with `--no-video`, MPRIS
is D-Bus only.

> If you reinstall or update while the service is running, restart it with
> `systemctl --user restart maly` (or `maly kill` if you're not using
> systemd) so it picks up the new binary.

### MPRIS (playerctl, Waybar…)

With the service running, it appears on the session bus as
`org.mpris.MediaPlayer2.maly`:

```sh
playerctl -p maly play-pause
playerctl -p maly metadata
```

Waybar's `mpris` module (nwg-piotr) picks it up with no extra
configuration. Desktop media keys (Hyprland, GNOME, KDE…) control it
directly too, as long as an MPRIS handler is active (on Hyprland, for
instance, via a `bindl` to `playerctl`).

---

## Configuration

File: `${XDG_CONFIG_HOME:-~/.config}/maly/config.toml`, created
automatically (with `0600` permissions) on the first run of any
subcommand. `maly config` always shows the **effective** configuration
(defaults ← `controls` preset ← your `[keys]`).

| Key | Type | Default | Does |
|---|---|---|---|
| `music_dir` | string | auto-resolved (`~/Music` or an XDG-derived source) | root of your library |
| `language` | string | `""` | `""` = ask on TUI open; `"en"` \| `"es"` |
| `controls` | string | `"default"` | key preset: `default` \| `vim` |
| `update_check` | bool | `true` | the TUI warns about new releases |
| `scan_durations` | bool | `true` | fill missing durations with ffprobe while scanning |

`[theme]`:

| Key | Default | Does |
|---|---|---|
| `transparent` | `true` | no background of its own; uses the terminal's |
| `accent` | `#7ab8b8` | accent color: focused panel, cursor |
| `border` | `#3a4448` | rules and separators |
| `text` | `#d4dadb` | primary text |
| `dim` | `#6b7a7e` | secondary text |
| `playing` | `#b85c50` | now-playing highlight |
| `error` | `#c96f60` | error text (console, flashes) |
| `accent_dim` | derived from `accent` | border of unfocused panels |
| `surface` | derived from `accent` | background of the selected row |
| `progress_low` / `progress_high` | derived from `accent` | progress bar gradient |
| `progress_shadow` | derived from `accent` | shadow under the bar (only where there's height) |
| `banner` | `"splash"` | ASCII art: `splash` (at startup) \| `titlebar` (one row) \| `off` |
| `logo` | `["#7ab8b8", "#8098a8", "#b85c50"]` | banner gradient stops (2 to 8) |

The five **derived** keys are computed from `accent` as long as you don't
write them: change `accent` and borders, selection, and progress bar follow
along without touching anything else. To pin one by hand, just write it and
yours wins.

`[visualizer]`:

| Key | Default | Does |
|---|---|---|
| `enabled` | `true` | turns the visualizer on |
| `color_low` / `color_high` | `#7ab8b8` / `#b85c50` | bar gradient |
| `bars_gravity` | `0.92` | bar-decay smoothing |
| `backend` | `"auto"` | `auto` (tries pw-record, then parec) \| `pipewire` \| `pulse` |

`[ytdlp]`:

| Key | Default | Does |
|---|---|---|
| `cookies_from_browser` | `""` | passed straight through to yt-dlp's `--cookies-from-browser`; empty = flag omitted. Accepts `browser:profile` |

`[keys]` remaps any action to a Bubble Tea key (defaults in
[Usage → TUI](#tui)); next to the config, an optional `logo.txt` replaces
the banner's ASCII art (colors still come from `[theme] logo`).

<details>
<summary>See the default generated <code>config.toml</code></summary>

```toml
music_dir = "~/Music"
language = ""             # "" = ask when opening the TUI; "en" | "es"
controls = "default"      # key scheme: default | vim (maly controls)
update_check = true       # the TUI warns about new versions (maly update)
scan_durations = true     # while scanning, read missing durations with ffprobe (skipped if it's not installed)

[theme]
transparent = true        # no background; use the terminal's
accent = "#7ab8b8"        # logo teal: focused panel, cursor, accents
border = "#3a4448"
text = "#d4dadb"
dim = "#6b7a7e"
playing = "#b85c50"       # logo terracotta: the track that is playing
error = "#c96f60"         # error text (console, flashes)
# These five are DERIVED from accent while they stay commented out: change
# accent and the rest of the UI follows without touching anything else.
# Uncomment any you want to pin by hand (the example values are the ones
# that come out of the accent above).
# accent_dim = "#4c7373"      # border of UNFOCUSED panels
# surface = "#1c2225"         # background of the selected row
# progress_low = "#4c7373"    # progress bar: start of the gradient
# progress_high = "#7ab8b8"   # progress bar: head
# progress_shadow = "#2e4545" # shadow under the bar
banner = "splash"         # ASCII art: splash (at startup) | titlebar (one row) | off
logo = ["#7ab8b8", "#8098a8", "#b85c50"]  # banner gradient stops (2 or more)
# banner art: create a logo.txt next to this file with your own ASCII

[visualizer]
enabled = true
color_low = "#7ab8b8"
color_high = "#b85c50"
bars_gravity = 0.92
# audio capture backend: auto (default) | pipewire | pulse — force one if
# you have both installed and the automatic pick (pw-record, then parec) is worse
backend = "auto"

[ytdlp]
# Browser to read cookies from for downloads that require an account
# (age-restricted videos, etc). Empty = disabled.
# Examples: firefox, chrome, chromium, brave, edge, vivaldi
# Also accepts browser:profile (e.g. "firefox:default-release")
# if you have multiple profiles — yt-dlp supports it natively.
# Note: with Chromium-based browsers (chrome/brave/etc) yt-dlp may need
# to unlock the keyring, and with the browser open its cookie database
# may be locked — if it fails, close it and try again.
cookies_from_browser = ""

[keys]
# Remap actions to Bubble Tea keys, e.g.:
# play_pause = " "
# next = "n"
# prev = "p"
# vol_up = "+"
# vol_down = "-"
# seek_forward = "right"
# seek_back = "left"
# switch_panel = "tab"
# filter = "/"
# add = "a"
# remove = "d"
# move_up = "K"
# move_down = "J"
# shuffle = "s"
# repeat = "r"
# quit = "q"
# help = "?"
# palette = "ctrl+p"
# songs = "ctrl+o"
# playlists = "ctrl+l"
# playlist_add = "A"
# toggle_viz = "v"
# now_playing = "ctrl+t"
```

</details>

### Lyrics (`.lrc`)

Synced lyrics are read from an **`.lrc` sidecar in the same directory as
the audio file, with the same base name** — `song.mp3` looks for
`song.lrc` right next to itself, never in a central lyrics directory.
Without a sidecar, embedded (USLT) lyrics from the file itself are used
instead. If you want to keep `.lrc` files elsewhere without filling
`music_dir` with text files, the real option today is a symlink next to
each track, or embedding the lyrics directly in the MP3.

### Files maly creates

| File | Path |
|---|---|
| `config.toml` | `${XDG_CONFIG_HOME:-~/.config}/maly/` |
| `logo.txt` (optional, you create it) | `${XDG_CONFIG_HOME:-~/.config}/maly/` |
| `library.db` (+ `-wal` / `-shm`) | `${XDG_DATA_HOME:-~/.local/share}/maly/` |
| `session.json` | `${XDG_DATA_HOME:-~/.local/share}/maly/` |
| `update.json` | `${XDG_DATA_HOME:-~/.local/share}/maly/` |
| `maly.sock`, `mpv.sock`, `maly.lock`, `art/` | `${XDG_RUNTIME_DIR:-/tmp}/maly/` |

---

## Troubleshooting

First stop: **`maly doctor`** — runs with no daemon and no network, and
diagnoses most of the below automatically.

| Symptom | Likely cause | Fix |
|---|---|---|
| `maly: command not found` | `~/.local/bin` isn't in your `PATH` | add it to your `.bashrc`/`.zshrc`/fish config (the installer offers to do this) |
| "mpv is not installed" when opening the TUI | mpv is missing | install it — it's the only hard dependency |
| Empty library | you never ran a scan, or `music_dir` points at a directory with no music | `maly scan`, or check `maly info` → `music_path`/`music_src` |
| Visualizer in animation mode | no `pw-record` or `parec` on PATH | install `pipewire` or `pulseaudio-utils`; maly retries real capture every 15s |
| No playerctl / media key integration | no D-Bus session bus available | make sure your graphical session exports `DBUS_SESSION_BUS_ADDRESS` |
| Cover art shows as colored blocks instead of a real image | your terminal isn't kitty, or you're under tmux | expected — the half-blocks fallback works on any truecolor terminal |
| Durations show as zero in the queue | no ffprobe, or `scan_durations = false` | install `ffmpeg` (it brings ffprobe), or play the track once |
| `maly get` fails | yt-dlp/ffmpeg missing, or yt-dlp is outdated (YouTube changes often) | `maly doctor` flags it; if yt-dlp is via pipx, `pipx upgrade yt-dlp` |
| "runs vX.Y.Z, this binary is vA.B.C" | you updated the binary but the old service is still running | `systemctl --user restart maly`, or `maly kill` without systemd |
| Two binaries / two services installed | the installer and the AUR package both got used | `maly update` already detects a packaged install and won't overwrite it; uninstall one of the two |
| A key doesn't do what you expect | a `[keys]` conflict: two actions mapped to the same key | `maly doctor` reports it with the exact detail |
| `go build` fails on the Go version | your Go is older than 1.25 | use [go.dev/dl/](https://go.dev/dl/), or let `mallow-install.sh` offer one on the side |

---

## Project structure

```text
.
├── cmd/maly/              CLI, TUI entry point, commands
│   └── completions/       bash/fish/zsh scripts (embedded in the binary)
├── internal/
│   ├── config/             config.toml loading/writing, XDG paths
│   ├── daemon/             the service: queue, session, startup, IPC dispatch
│   ├── player/              mpv wrapper (gapless, socket control)
│   ├── queue/               permutation-based shuffle and repeat
│   ├── library/             SQLite: tags, search, playlists, M3U
│   ├── ipc/                 the Unix-socket protocol (Request/Response)
│   ├── mpris/                MPRIS2 integration (D-Bus)
│   ├── media/                embedded cover art/lyrics, .lrc
│   ├── viz/                  audio capture + visualizer FFT
│   ├── getter/                yt-dlp wrapper (`maly get`)
│   ├── probe/                 ffprobe wrapper (durations)
│   ├── update/                 release check against git tags
│   ├── i18n/                    en/es translation table
│   ├── safetext/                 sanitizing untrusted text (tags, lyrics)
│   ├── version/                   binary version
│   └── tui/                       the full Bubble Tea interface
├── pictures/               screenshots used in this README
├── .github/workflows/       CI (build+vet+test, and -race over library/mpris)
├── mallow-install.sh         bilingual interactive installer
├── Makefile                   build / vet / test / install / clean
├── CHANGELOG.md                release log (Spanish)
├── CLAUDE.md                    deep engineering doc (Spanish)
└── LICENSE                       GPLv3
```

---

## Development

```sh
git clone https://github.com/kitasael-burakku/Malody-Mallow.git
cd Malody-Mallow
make build      # go build -o maly ./cmd/maly — go build ./... does NOT regenerate this binary
make vet        # go vet ./...
make test       # go test ./...
```

- `internal/daemon` and `internal/player` tests use a real mpv and
  self-skip (`t.Skip`) if it's not found on PATH — no mocking needed to
  run the rest of the suite.
- `go test -race ./internal/library/ ./internal/mpris/` — the two packages
  with real goroutine concurrency that don't depend on mpv/ffprobe; it's
  exactly what CI's `race` job runs.
- CI (`.github/workflows/ci.yml`) runs two jobs on every push/PR to
  `main`: `test` (full build + vet + test) and `race` (the two packages
  above with `-race`).
- `maly completions <shell>` regenerates the scripts embedded under
  `cmd/maly/completions/` if you change the command table.
- To debug TUI or daemon startup in an isolated environment, `CLAUDE.md`
  documents the XDG sandbox used in development (short paths for mpv's
  socket, `ao=null` in `mpv.conf`, how to test under tmux).

`CLAUDE.md` is the project's full engineering document: detailed
per-package architecture, cross-cutting security/concurrency decisions,
and the reasoned history of every release. `CHANGELOG.md` is the short
version, release by release.

---

## Project status

Malody Mallow is under active development at a frequent release cadence
(see [`CHANGELOG.md`](CHANGELOG.md)). The core — gapless playback, a
persistent service, the SQLite library, MPRIS, the TUI, the CLI — is
stable and used daily by the author as their main player. The project has
gone through several self-driven security, performance, and UX audits,
documented in `CLAUDE.md`.

Mouse support in the TUI is deliberately out of scope. The project stays
focused on coordinating existing tools (mpv, yt-dlp, ffmpeg) rather than
reimplementing them.

---

## Credits

Malody Mallow coordinates and builds on:

- [mpv](https://mpv.io/) — audio playback engine.
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) and [ffmpeg](https://ffmpeg.org/) — audio download and processing (optional).
- [PipeWire](https://pipewire.org/) / [PulseAudio](https://www.freedesktop.org/wiki/Software/PulseAudio/) — audio capture for the visualizer.
- [MPRIS2](https://specifications.freedesktop.org/mpris-spec/latest/) over [D-Bus](https://www.freedesktop.org/wiki/Software/dbus/) — desktop integration.
- [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lipgloss](https://github.com/charmbracelet/lipgloss), and [Bubbles](https://github.com/charmbracelet/bubbles) (Charm) — the TUI.
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — pure-Go SQLite, no CGo.
- [gonum](https://www.gonum.org/) — the visualizer's FFT.
- [godbus](https://github.com/godbus/dbus) — the D-Bus client behind MPRIS.

---

## License

[GPLv3](LICENSE).
