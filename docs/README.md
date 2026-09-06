# Documentación de maly

`CLAUDE.md` (en la raíz) es el **contrato operativo**: filosofía, invariantes,
arquitectura de alto nivel y reglas de trabajo. Es lo único que se lee siempre.
Todo lo demás vive aquí y se consulta cuando hace falta.

Cada hecho tiene **un** dueño. Si algo está desarrollado en un archivo de
`docs/`, `CLAUDE.md` lleva la regla y remite aquí, no una segunda copia.

## `architecture/` — ficha por paquete

El resumen de una tabla está en `CLAUDE.md`; aquí está el desarrollo completo,
movido desde `CLAUDE.md` sin cambiar una palabra.

| Archivo | Paquetes |
| --- | --- |
| [`cli.md`](architecture/cli.md) | `cmd/maly` (CLI, `startOrAttach`, `get.go`, `info`/`doctor`) |
| [`daemon.md`](architecture/daemon.md) | `internal/ipc`, `internal/daemon` |
| [`playback.md`](architecture/playback.md) | `internal/player`, `internal/queue` |
| [`library.md`](architecture/library.md) | `internal/library`, `internal/probe` |
| [`mpris-media.md`](architecture/mpris-media.md) | `internal/mpris`, `internal/media`, `internal/safetext` |
| [`tui.md`](architecture/tui.md) | `internal/tui` (incluye el visualizador) |
| [`soporte.md`](architecture/soporte.md) | `internal/i18n`, `internal/update` |

Los invariantes de `internal/config` viven en la sección «Invariantes
transversales» de `CLAUDE.md`; los de `internal/getter`, repartidos entre
[`architecture/cli.md`](architecture/cli.md) (cómo se invoca) y
[`decisions/no-hacer.md`](decisions/no-hacer.md) (qué salida se parsea y cuál
no).

## `decisions/`

- [`no-hacer.md`](decisions/no-hacer.md) — todo lo que se evaluó y se
  descartó, con la medición o el experimento que lo cerró. **Léelo antes de
  proponer** miniaturas, salida JSON, sacar `search` de `d.mu`, mover la
  descarga al demonio, volver a `godbus/prop` o reintroducir Matugen.

## `development/`

- [`probar-en-vivo.md`](development/probar-en-vivo.md) — sandbox XDG, tmux,
  matar procesos sin llevarse el `mpvpaper` del dueño, la DB en WAL.
- [`verificacion.md`](development/verificacion.md) — cómo se comprueba que un
  test prueba lo que dice: las seis trampas del arnés, qué no cazan `-race` ni
  `go vet`, y las dependencias externas de los tests.
- [`releases.md`](development/releases.md) — qué toca un bump, los tres
  canales de distribución, la unit de systemd y las trampas del instalador.

## `history/`

El roadmap de ingeniería completo (~126.000 caracteres), movido desde
`CLAUDE.md` sin cambiar una palabra y repartido por rango de versión. Índice y
tabla versión-por-versión: [`history/README.md`](history/README.md).

Es el razonamiento: qué se midió, qué se descartó y por qué, qué trampa costó
un ciclo. El resumen público, versión por versión, está en `CHANGELOG.md` (en
la raíz); son dos audiencias distintas y ninguno reemplaza al otro.
