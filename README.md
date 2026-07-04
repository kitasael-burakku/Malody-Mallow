# maly

Reproductor de música local para terminal, estilo btop/lazygit: TUI con
paneles, demonio con socket Unix y CLI tipo `mpc`/`playerctl`, todo en un
solo binario.

```
╭─ Biblioteca (142) ─────────────╮╭─ Cola (12) ────────────────────╮
│▾ kaisoyeon                     ││▶   1. kaisoyeon — La Presiento │
│  ▾ (sin álbum)                 ││    2. Proporción Áurea         │
│     La Presiento               ││    3. Amanecer                 │
╰────────────────────────────────╯╰────────────────────────────────╯
╭─ Visualizador ─────────────────────────────────────────────────╮
│            ▂▄▆█▅▂      ▁▃▅▂        ▂▄▂                         │
│▃▃▂▂▂▄▅▆▇███████████▆▅▄▅█████▅▄▃▃▄▅█████▄▃▂▂▂▂▁▁▁               │
╰────────────────────────────────────────────────────────────────╯
╭─ Ahora suena ──────────────────────────────────────────────────╮
│ ▶ kaisoyeon — La Presiento        01:23/03:17  vol 80%  ⇄ ⟲    │
│━━━━━━━━━━━━━━━━━━━─────────────────────────────────────────────│
╰────────────────────────────────────────────────────────────────╯
```

## Características

- **Backend mpv**: MP3, FLAC, OGG, OPUS, M4A, WAV sin esfuerzo.
- **Demonio + cliente**: la música sigue sonando aunque cierres la TUI
  (si lanzaste `maly daemon` aparte). Control desde cualquier terminal.
- **Biblioteca SQLite**: escaneo de tags (artista/álbum/título/año/género),
  búsqueda insensible a acentos y mayúsculas ("aurea" encuentra "Áurea").
- **Visualizador de espectro**: FFT en vivo del monitor de
  PipeWire/PulseAudio, con gradiente de color; las barras siguen la
  amplitud suavizada (estilo CAVA).
- **Paleta Ctrl+P**: consola integrada de comandos (`maly next`, `vol +5`,
  `status`…) con la salida dentro de la propia paleta.
- **Selector Ctrl+O**: búsqueda difusa sobre toda la biblioteca
  (`enter` reproduce, `tab` agrega a la cola).
- **Bilingüe**: interfaz en English/Español; se elige al primer arranque
  (clave `language` del config).
- **Playlists**, shuffle, repeat (off/all/one), cola en vivo.
- Tema y keybindings configurables por TOML; fondo transparente (usa el
  color de tu terminal).

## Instalación

Dependencias de sistema: `mpv` (audio) y, para el visualizador, PipeWire
o PulseAudio con sus herramientas de línea de comandos.

**Arch Linux**

```sh
sudo pacman -S mpv pipewire pipewire-pulse   # pw-record viene con pipewire
```

**Ubuntu / Debian**

```sh
sudo apt install mpv pipewire-bin      # o: sudo apt install mpv pulseaudio-utils
```

**Compilar maly** (Go ≥ 1.22):

```sh
go build -o maly ./cmd/maly
install -Dm755 maly ~/.local/bin/maly   # o donde prefieras en tu PATH
```

Sin `pw-record`/`parec` maly funciona igual; el visualizador degrada a una
animación y te lo avisa una vez.

## Uso

```sh
maly scan            # indexa ~/Music (o music_dir del config) en SQLite
maly                 # abre la TUI; arranca el demonio embebido si no hay uno
```

La primera vez se crea `~/.config/maly/config.toml` con los defaults.

### TUI

| Tecla | Acción |
|---|---|
| `espacio` | reproducir / pausar |
| `n` / `p` | siguiente / anterior |
| `+` / `-` | volumen ±5% |
| `←` / `→` | seek ±5s |
| `tab` | cambiar de panel |
| `enter` | reproducir pista / expandir nodo |
| `a` | agregar a la cola (pista, álbum o artista) |
| `d` | quitar de la cola |
| `/` | filtrar el panel actual |
| `s` / `r` | shuffle / repeat |
| `v` | alternar visualizador |
| `ctrl+p` | paleta de comandos (consola integrada) |
| `ctrl+o` | selector de canciones (fuzzy; `enter` reproduce, `tab` agrega) |
| `?` | ayuda |
| `q` | salir |

Todas remapeables en `[keys]` del config.

### CLI (estilo mpc)

```sh
maly daemon                    # demonio sin TUI (headless)
maly play [consulta]           # reproduce; con consulta busca en la biblioteca
maly pause | toggle | stop
maly next | prev
maly add <consulta|ruta>       # agrega a la cola (acepta archivos y carpetas)
maly queue                     # muestra la cola
maly status                    # qué suena, posición, volumen, modos
maly vol 80 | vol +5 | vol -5
maly seek +10 | seek -10 | seek 1:30
maly shuffle [on|off]
maly repeat [off|all|one]
maly search <consulta>         # busca en la biblioteca (funciona sin demonio)
maly scan [ruta]               # (re)escanea (funciona sin demonio)
maly playlist list
maly playlist create <nombre>
maly playlist add <nombre> <consulta>
maly playlist play <nombre>
maly playlist delete <nombre>
maly lang [en|es]              # cambia el idioma (sin arg abre el selector); alias -l
maly version | -v
```

Los comandos de biblioteca (`scan`, `search`, `playlist list/create/add/delete`)
operan directo sobre SQLite y no necesitan demonio. Los de reproducción piden
el demonio: ábrelo con `maly` o `maly daemon`.

## Configuración

`~/.config/maly/config.toml` (se genera con estos defaults):

```toml
music_dir = "~/Music"
language = ""             # "" = preguntar al abrir la TUI; "en" | "es"

[theme]
transparent = true        # sin fondo; usar el del terminal
accent = "#89b4fa"
border = "#45475a"
text = "#cdd6f4"
dim = "#6c7086"
playing = "#a6e3a1"

[visualizer]
enabled = true
color_low = "#89b4fa"     # color de la base de las barras
color_high = "#f38ba8"    # color de las puntas
bars_gravity = 0.92       # 0-1: cuánto tardan en caer las barras

[keys]
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
# shuffle = "s"
# repeat = "r"
# quit = "q"
# help = "?"
# palette = "ctrl+p"
# songs = "ctrl+o"
# toggle_viz = "v"
```

## Arquitectura

- `maly` (sin args) abre la TUI; si no hay demonio, lo embebe (muere al salir).
- `maly daemon` lo deja corriendo aparte; la TUI y el CLI se conectan a él.
- Socket: `$XDG_RUNTIME_DIR/maly/maly.sock`, protocolo JSON de una línea.
- Base de datos: `~/.local/share/maly/library.db` (SQLite puro Go, sin CGo).
- maly lanza y supervisa su propio `mpv --idle --no-video` y lo controla por
  IPC JSON; al cerrar maly, su mpv muere con él.
