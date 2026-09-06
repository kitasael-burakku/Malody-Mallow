# Roadmap de ingeniería — índice

El roadmap completo vivía dentro de `CLAUDE.md` y pesaba ~126.000 caracteres.
Está movido aquí **sin cambiar una palabra**, repartido en cuatro archivos por
rango de versión. Esta tabla existe para que se pueda ir directo al archivo
correcto sin abrir los cuatro.

Qué es cada cosa:

- **Este roadmap** — el razonamiento de ingeniería: qué se midió, qué se
  descartó y por qué, qué trampa costó un ciclo. Para quien va a tocar código.
- **`CHANGELOG.md`** (raíz) — el resumen público, versión por versión, para
  quien solo quiere saber qué cambió. No lo reemplaza: son dos audiencias.
- **`CLAUDE.md`** (raíz) — el contrato operativo: filosofía, invariantes y
  arquitectura de alto nivel. Es lo único que se carga siempre.

## Archivos

| Archivo | Rango | Qué contiene |
| --- | --- | --- |
| [`roadmap-1.0-1.7.md`](roadmap-1.0-1.7.md) | v1.0.0 – v1.7.3 | Primeras releases, auditorías 1 y 2, diagnóstico, rendimiento |
| [`roadmap-1.8-1.11.md`](roadmap-1.8-1.11.md) | v1.8.0 – v1.11.1 | Auditoría técnica exhaustiva, Matugen revertido, AUR, canal de paquete |
| [`roadmap-1.12-1.13.md`](roadmap-1.12-1.13.md) | v1.12.0 – v1.13.1 | Auditoría de UX (76 hallazgos), segunda auditoría técnica integral |
| [`roadmap-1.14-1.16.md`](roadmap-1.14-1.16.md) | v1.14.0 – v1.16.4 | Rediseño de la pantalla, buscador de descargas, promesa obsoleta, Phase 0 |
| [`candidatos-post-1.0.md`](candidatos-post-1.0.md) | — | Lista de candidatos (cerrada) y las mediciones que refutaron hipótesis |

## Versión por versión

Una línea por entrada, con el archivo donde está desarrollada. Las entradas
«Sobre X, sin bump de versión» son cambios que no tocaron el binario
(instalador, PKGBUILD, unit de systemd) y viven junto a la versión que las
precede.

### v1.0.0 – v1.7.3 → [`roadmap-1.0-1.7.md`](roadmap-1.0-1.7.md)

| Versión | Qué trajo |
| --- | --- |
| 1.0.0–1.0.2 | Primer tag, revisión de seguridad, instalador interactivo por pantallas |
| 1.1.0 | Capa «Ahora suena» (ctrl+t), `maly kill`, colores del logo configurables |
| 1.1.5 | **Auditoría completa 1**: resolución de pistas fuera de `d.mu`, `--ref=` en update, `Search` sin `LIMIT 500` |
| 1.2.0 | Carátula real en kitty (protocolo gráfico), rediseño visual del instalador |
| 1.2.1 | `[ytdlp] cookies_from_browser` (passthrough sin validar) |
| 1.3.0 | `maly move`, progreso de scan, ancho dinámico de la ayuda `?` |
| 1.3.1 | Op IPC `refresh`: las mutaciones de playlists se reflejan en vivo |
| 1.4.0 | Shuffle por **permutación** (`order`/`pos`/`staged`), se va `history` |
| 1.5.0 | `internal/probe` + `FillDurations` (fase 2 del scan); B11 cerrado (seek fuera de `d.mu`) |
| 1.5.1 | El panel ctrl+l abierto se entera de mutaciones ajenas; árbol sin saltos |
| 1.6.0 | **Auditoría de seguridad 2**: `internal/safetext`, NaN/Inf, flock, SIGHUP, caché de carátulas acotado |
| 1.6.1 | Cierre de la auditoría: hallazgo #4 **medido y refutado**, `Model.progressBar` |
| 1.6.2 | El chequeo de update se repite cada hora, no solo en `Init` |
| 1.7.0 | `maly info` y `maly doctor` (contratos de salida distintos); instalador compila el último tag |
| 1.7.1 | Análisis de rendimiento: solo una pieza (sondas de `FillDurations` en paralelo) |
| 1.7.2 | Relojes de animación autocancelados (3,5 % → 0,7 % de un núcleo en reposo) |
| 1.7.3 | `library.SearchLimit`: el TAB pasa de 92 ms/39 MB a 14 ms/16 MB con 40k pistas |

### v1.8.0 – v1.11.1 → [`roadmap-1.8-1.11.md`](roadmap-1.8-1.11.md)

| Versión | Qué trajo |
| --- | --- |
| 1.8.0 | Prioridad alta de la auditoría exhaustiva: espejo del gapless, `folded` del árbol, **CI** |
| 1.9.0 | Prioridad media: `maly config`, tests de doctor/info, systemd empaquetada, Matugen (revertido después) |
| 1.10.0 | Prioridad baja: índices SQLite muertos, Makefile+CHANGELOG, `[visualizer] backend`, `mpris.Controller` |
| 1.10.1 | Recarga en caliente por `SIGUSR1` (revertida en la 1.10.2) |
| 1.10.2 | **Matugen sacado entero**, código y sistema real; paquete `maly` publicado en el AUR |
| 1.11.0 | **Auditoría de seguridad desde cero**: sin vulnerabilidades reales; 4 endurecimientos + `maly get playlist` |
| 1.11.1 | `version.Channel`/`Packaged()`: `maly update` remite al gestor de paquetes; unit a `default.target` |

### v1.12.0 – v1.13.1 → [`roadmap-1.12-1.13.md`](roadmap-1.12-1.13.md)

| Versión | Qué trajo |
| --- | --- |
| 1.12.0 | **Auditoría de UX** (76 hallazgos): P0+P1+P2 cerrados; 4 no tocados con su razón |
| 1.12.1 | Los modales rompían el borde: `panel()` no trunca por ancho, cada llamador es responsable |
| — | Auditoría técnica del 2026-08-08 (commit `5f10d3f`): 23 hallazgos en dos tandas |
| 1.13.0 | Cinco fases de la segunda auditoría integral: `recoverResponse`, `maxSubscribers`, hardening de systemd |
| — | `RuntimeDirectory=`/`ConfigurationDirectory=` (el `ReadWritePaths` rompía en boot limpio) |
| 1.13.1 | Lyrics View: pulso de brillo y corte de visibilidad (`maxLyricDistance`) |

### v1.14.0 – v1.16.4 → [`roadmap-1.14-1.16.md`](roadmap-1.14-1.16.md)

| Versión | Qué trajo |
| --- | --- |
| 1.14.0 | **Rediseño de la pantalla principal**: `computeLayout` puro, barra de progreso, títulos limpios |
| 1.14.1 | La columna «Ahora suena» quedaba con filas muertas en terminales altas |
| 1.15.0 | **Buscador de descargas ctrl+g**: `getter.Search` con `--dump-json`; tres premisas refutadas |
| — | El éxito de una descarga se mide por diff de directorio, no por el exit code (con la **corrección** de la medición anterior) |
| 1.16.0 | Marcador `✓` de lo ya descargado, filtro de lives, `maly get pick` |
| 1.16.1 | `view_count` en los resultados; `pickerWidthMax`; miniaturas **descartadas** |
| 1.16.2 | `maly -h` saca los atajos de `tui.HelpRows`; C24, G7 y C26 |
| 1.16.3 | **Carrera de la promesa obsoleta** en `advance()`: `loadGen` viaja hasta el consumidor |
| 1.16.4 | **Phase 0** de la auditoría del 2026-09: A-01 (scan vacío no purga), A-02, A-03 |
