# Graph Report - maly  (2026-08-20)

## Corpus Check
- cluster-only mode — file stats not available

## Summary
- 1531 nodes · 4338 edges · 78 communities (67 shown, 11 thin omitted)
- Extraction: 87% EXTRACTED · 13% INFERRED · 0% AMBIGUOUS · INFERRED: 546 edges (avg confidence: 0.85)
- Token cost: 85,605 input · 1,098 output

## Graph Freshness
- Built from commit: `ec0af17b`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Playlist CLI Commands
- Shell Completion Engine
- MPRIS D-Bus Service
- Daemon Core Dispatch
- Image and Lyrics Decoding
- Track Title Cleanup
- Project Docs and CI
- Library Open and Benchmarks
- mpv Player Wrapper
- Interactive Installer Script
- Config Paths and Values
- TUI Model Tests
- Update Check and Install
- yt-dlp Remote Search
- TUI Model and Tickers
- i18n and Player Test Fakes
- Config Loading Tests
- Download Search Screen Tests
- Queue Shuffle and Repeat Tests
- D-Bus Properties Implementation
- Design Decisions and Invariants
- CLI Command Table and Help
- Doctor Diagnostics Checks
- Playback Queue
- TUI Playlist Commands
- Now Playing View Tests
- Console Palette Tests
- Info and Config Output
- SQLite Library Core
- Console Command Handlers
- Library Tree Navigation
- TUI View Rendering
- Album Art Cache
- Fuzzy Picker Tests
- Theme Color Blending
- Layout Computation
- Now Playing Lyrics Panel
- Daemon Client Architecture
- CLI Entry Point
- Banner Logo Rendering
- View Layout Tests
- Release Roadmap and Scanning
- Download Directory Diffing
- Playlist Storage
- Download Search Screen
- TUI Key Handling
- Visualizer Capture Tests
- Control Char Sanitizing and M3U
- Getter Options and Pick
- IPC Client Commands
- Lyric and Viz Styling
- Security Audits and IPC
- Text Sanitizing Invariants
- Packaging and Update Channel
- Selector Model
- Library Scanning
- ffprobe Duration Probe
- Progress Bar Tests
- Progress Bar Rendering
- TUI Error Wrapping
- Track Resolution
- README and Changelog
- External Tool Philosophy
- Logo Tick Commands
- Key Conflict Detection
- Language Selector
- Now Playing Column
- Song Picker Screen
- Bash Completion Script
- Progress Bar Model Methods
- Track Info Display
- maly Binary
- English README Status

## God Nodes (most connected - your core abstractions)
1. `T()` - 106 edges
2. `Tf()` - 82 edges
3. `Open()` - 55 edges
4. `Load()` - 54 edges
5. `Model` - 41 edges
6. `Daemon` - 41 edges
7. `DBPath()` - 40 edges
8. `Dial()` - 39 edges
9. `Track` - 37 edges
10. `newStyles()` - 34 edges

## Surprising Connections (you probably didn't know these)
- `Captura: selector de canciones difuso (ctrl+o)` --semantically_similar_to--> `Autocompletado dinámico de shell`  [INFERRED] [semantically similar]
  pictures/song-picker.jpg → README.md
- `CI job: test` --semantically_similar_to--> `Makefile`  [INFERRED] [semantically similar]
  .github/workflows/ci.yml → README.md
- `Captura: escritorio Hyprland con la TUI, fastfetch y Waybar` --references--> `Integración MPRIS (playerctl, Waybar)`  [INFERRED]
  pictures/desktop-hyprland.png → README.md
- `Captura: pantalla principal de la TUI (tres columnas)` --references--> `Playlists de maly`  [INFERRED]
  pictures/tui-main.jpg → README.md
- `Captura: escritorio Hyprland con la TUI, fastfetch y Waybar` --conceptually_related_to--> `Unit systemd --user maly.service`  [INFERRED]
  pictures/desktop-hyprland.png → README.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Flujo de descarga vía yt-dlp (CLI y TUI)** — internal_getter, internal_getter_search, internal_getter_opts, internal_getter_diff, internal_getter_cleanup, cmd_maly_downloadOne, internal_tui_get, internal_tui_getpick [EXTRACTED 0.90]
- **Fronteras de saneado de texto ajeno** — internal_safetext, internal_library_scantrack, internal_media_parselrc, internal_getter_search, internal_ipc, claude_invariant_no_control_chars [EXTRACTED 0.90]
- **Orden de arranque del demonio (no negociable)** — internal_daemon_new, internal_daemon_acquirelock, internal_library, internal_player_start, internal_daemon_session, internal_mpris [EXTRACTED 0.95]
- **maly coordina herramientas externas en vez de reimplementarlas** — readme_malody_mallow, readme_credits_mpv, readme_credits_ytdlp, readme_credits_ffmpeg, readme_credits_pipewire [EXTRACTED 0.95]

## Communities (78 total, 11 thin omitted)

### Community 0 - "Playlist CLI Commands"
Cohesion: 0.05
Nodes (83): runDaemon(), confirmOverwrite(), confirmYesNo(), isTTY(), notifyRefresh(), runPlaylist(), shouldDeletePlaylist(), TestPlaylistAddNoExisteSugiereCreate() (+75 more)

### Community 1 - "Shell Completion Engine"
Cohesion: 0.05
Nodes (79): cand(), completeArgs(), completeCommands(), completeControls(), completeJump(), completeMove(), completePlaylist(), completePlaylistNames() (+71 more)

### Community 2 - "MPRIS D-Bus Service"
Cohesion: 0.05
Nodes (38): github.com/godbus/dbus/v5/introspect.Node, BusAvailable(), dbus.Conn, dbus.Error, dbus.ObjectPath, dbus.Variant, Service, loopOf() (+30 more)

### Community 3 - "Daemon Core Dispatch"
Cohesion: 0.06
Nodes (32): subscriber, bufio.Reader, bufio.Scanner, net.Conn, net.Listener, sync/atomic.Bool, sync/atomic.Int64, sync/atomic.Uint64 (+24 more)

### Community 4 - "Image and Lyrics Decoding"
Cohesion: 0.06
Nodes (38): gonum.org/v1/gonum/dsp/fourier.FFT, image.Image, image.RGBA, io.Reader, sync.Once, testing.F, time.Time, DecodeImage() (+30 more)

### Community 5 - "Track Title Cleanup"
Cohesion: 0.07
Nodes (35): github.com/charmbracelet/bubbles/textinput.Model, Fold(), TestFold(), allNoise(), cleanTitle(), collapseSpaces(), cutAtSep(), dropArtistPrefix() (+27 more)

### Community 6 - "Project Docs and CI"
Cohesion: 0.08
Nodes (46): CI job: race, CI job: test, Captura: Command Palette de la TUI (ctrl+p), Captura: escritorio Hyprland con la TUI, fastfetch y Waybar, Captura: capa "Ahora suena" a pantalla completa (ctrl+t), Captura: selector de canciones difuso (ctrl+o), Captura: pantalla principal de la TUI (tres columnas), CHANGELOG.md (+38 more)

### Community 7 - "Library Open and Benchmarks"
Cohesion: 0.09
Nodes (43): testing.B, testing.TB, BenchmarkScan(), BenchmarkSearchLimit(), Open(), OpenIfExists(), fakeMusicDir(), Library (+35 more)

### Community 8 - "mpv Player Wrapper"
Cohesion: 0.11
Nodes (13): encoding/json.RawMessage, os/exec.Cmd, sync.Mutex, sync.WaitGroup, Player, mpvStartError(), reapStale(), safeCall() (+5 more)

### Community 9 - "Interactive Installer Script"
Cohesion: 0.12
Nodes (29): ask(), banner(), cleanup(), confirm(), dep_label(), die(), fetch(), fetch_show() (+21 more)

### Community 10 - "Config Paths and Values"
Cohesion: 0.10
Nodes (31): runGetPick(), runControls(), runLogo(), Visualizer, Ytdlp, clampHex(), collapseTilde(), ConfigDir() (+23 more)

### Community 11 - "TUI Model Tests"
Cohesion: 0.11
Nodes (31): DefaultKeys(), keyMsgFor(), TestBibliotecaNoMienteVaciaMientrasCarga(), TestCerrarAyudaRedespachaLaTecla(), TestClipPadTo(), TestConsolaDiagnostico(), TestConsoleViewMuestraProgresoDeScan(), TestErrorDeBibliotecaPersiste() (+23 more)

### Community 12 - "Update Check and Install"
Cohesion: 0.12
Nodes (27): runUpdate(), captureStdout(), fakeGitNewerTag(), TestRunUpdateCurrentVersion(), TestRunUpdateNotPackagedNeedsCurl(), TestRunUpdatePackaged(), updateCheckCmd(), Cached() (+19 more)

### Community 13 - "yt-dlp Remote Search"
Cohesion: 0.18
Nodes (25): searchEntry, context.Context, cmdErr(), decodeResults(), firstLine(), Result, Search(), emit() (+17 more)

### Community 14 - "TUI Model and Tickers"
Cohesion: 0.14
Nodes (16): context.CancelFunc, TestRechequeoDeUpdate(), Model, keyRepeats(), tickCmd(), updTickCmd(), vizTickCmd(), actionMsg (+8 more)

### Community 15 - "i18n and Player Test Fakes"
Cohesion: 0.14
Nodes (23): testing.T, TestSetCode(), TestT(), TestTableIntegrity(), TestTf(), TestTL(), TestTLf(), verbs() (+15 more)

### Community 16 - "Config Loading Tests"
Cohesion: 0.21
Nodes (25): Load(), env(), TestConfigPrivate(), TestConfigViejoSigueCargando(), TestDerivadoExplicitoGana(), TestDerivadoInvalidoSeRepuebla(), TestDerivadosSiguenAlAccentDelUsuario(), TestEnsureRuntimeDir() (+17 more)

### Community 17 - "Download Search Screen Tests"
Cohesion: 0.14
Nodes (25): fmtViews(), getItems(), boxLinesSameWidth(), fakeTools(), Model, key(), newGetModel(), TestConGetUso() (+17 more)

### Community 18 - "Queue Shuffle and Repeat Tests"
Cohesion: 0.25
Nodes (23): New(), checkOrder(), mk(), TestMove(), TestMovePromiseFollowsTrack(), TestNextRepeatOff(), TestPeekInvalidatedByMutation(), TestPeekNextSequential() (+15 more)

### Community 19 - "D-Bus Properties Implementation"
Cohesion: 0.17
Nodes (15): github.com/godbus/dbus/v5/introspect.Property, exportProps(), dbus.Conn, dbus.Error, dbus.ObjectPath, dbus.Variant, dbus.Error, newTestProps() (+7 more)

### Community 20 - "Design Decisions and Invariants"
Cohesion: 0.10
Nodes (20): Red de paridad consola ↔ CLI, Release 1.14.0 — rediseño de la pantalla principal, Trampas conocidas al probar en vivo, Disciplina de verificación en ambas direcciones, internal/config, Theme.ResolveDerived, config.saveKey, internal/tui — Bubble Tea (+12 more)

### Community 21 - "CLI Command Table and Help"
Cohesion: 0.13
Nodes (17): client(), helpKeys(), helpText(), lookup(), padRight(), runHelp(), TestConsoleParityConCLI(), TestHelpAtajosDesdeHelpRows() (+9 more)

### Community 22 - "Doctor Diagnostics Checks"
Cohesion: 0.26
Nodes (16): runVersion(), serviceVersion(), checkKeys(), checkLibrary(), checkMpv(), checkMusicDir(), checkOptionalTools(), checkService() (+8 more)

### Community 24 - "TUI Playlist Commands"
Cohesion: 0.17
Nodes (11): github.com/charmbracelet/bubbletea.Cmd, Model, getSpinCmd(), Model, notifyRefresh(), plAddCmd(), plCreateCmd(), plDeleteCmd() (+3 more)

### Community 25 - "Now Playing View Tests"
Cohesion: 0.14
Nodes (19): TestLangEscChoosesActiveLanguage(), TestActiveLyric(), TestActiveLyricStyleFrozenWhenNotPlaying(), TestHalfBlocksRender(), TestNpLyricsClamp(), TestNpLyricsLinesBlanksBeyondMaxDistance(), TestNpLyricsLinesNoBlankWithinNaturalWindow(), TestNpMetaTimeLineNoOverflow() (+11 more)

### Community 26 - "Console Palette Tests"
Cohesion: 0.17
Nodes (20): boxLinesFitWithin(), Model, newConModel(), TestConHelpNoOverflow(), TestConsoleCommandsSonReales(), TestConsoleControlsList(), TestConsoleHistorial(), TestConsolePlaylistRoundTrip() (+12 more)

### Community 27 - "Info and Config Output"
Cohesion: 0.14
Nodes (14): orUnset(), runConfig(), libraryStats(), runInfo(), libraryStats, OnOff(), internal/probe — ffprobe, Available() (+6 more)

### Community 28 - "SQLite Library Core"
Cohesion: 0.18
Nodes (9): database/sql.DB, clampInt(), Library, Track, scanTrack(), tighten(), pending, libraryMsg (+1 more)

### Community 29 - "Console Command Handlers"
Cohesion: 0.28
Nodes (3): github.com/charmbracelet/bubbletea.Model, Model, langLabel()

### Community 30 - "Library Tree Navigation"
Cohesion: 0.24
Nodes (8): containsAll(), keyOf(), nodeKey(), TestContainsAll(), libTree, node, nodeKind, treeState

### Community 31 - "TUI View Rendering"
Cohesion: 0.22
Nodes (4): FmtTime(), padTo(), Model, marker()

### Community 32 - "Album Art Cache"
Cohesion: 0.18
Nodes (12): ReadEmbedded(), newArtCache(), safeExt(), id3v2WithCover(), TestArtCache(), TestArtCacheEvicta(), TestSafeExt(), TestServiceMetadataArt() (+4 more)

### Community 33 - "Fuzzy Picker Tests"
Cohesion: 0.20
Nodes (17): TestPickModelFlujo(), TestCtrlXPidenConfirmacion(), TestLibraryMsgRefreshesPlaylists(), TestPickerCtrlDU(), TestPickerDistingueBibliotecaVacia(), TestSongsViewMuestraFlash(), newPicker(), newPickerItem() (+9 more)

### Community 34 - "Theme Color Blending"
Cohesion: 0.20
Nodes (13): github.com/charmbracelet/lipgloss.Color, BlendHex(), ParseHex(), scaleHex(), blendHex(), parseHex(), pulseColor(), TestBlendHexGradation() (+5 more)

### Community 35 - "Layout Computation"
Cohesion: 0.21
Nodes (14): clampInt(), computeLayout(), Model, allOpts(), TestComputeLayoutAnchosDeColumna(), TestComputeLayoutBarraSiempre(), TestComputeLayoutDegenerado(), TestComputeLayoutInvariantes() (+6 more)

### Community 36 - "Now Playing Lyrics Panel"
Cohesion: 0.25
Nodes (9): activeLyric(), center(), Model, loadNowMeta(), lyricDistance(), npDetail(), TestNpLyricsLinesActiveFrozenWhenPaused(), wrapLyric() (+1 more)

### Community 37 - "Daemon Client Architecture"
Cohesion: 0.14
Nodes (16): Arquitectura demonio + clientes, Grafo de conocimiento graphify, Invariante: el demonio y sus hijos mueren juntos, Malody Mallow (maly), Convención de proyecto en español, cmd/maly (CLI), internal/daemon, advance(reason, chained) (+8 more)

### Community 38 - "CLI Entry Point"
Cohesion: 0.20
Nodes (14): downloadOne(), envLangHint(), langName(), main(), openLibrary(), printTracks(), runLang(), runScan() (+6 more)

### Community 39 - "Banner Logo Rendering"
Cohesion: 0.17
Nodes (4): Model, newLogo(), TestNewLogoArt(), logoModel

### Community 40 - "View Layout Tests"
Cohesion: 0.23
Nodes (15): Model, newLayoutTestModel(), TestBarraDeProgresoUnaSola(), TestFiltroNoRompeElBorde(), TestInvalidateArtTiraLosDosRenders(), TestNpColumnAprovechaElAncho(), TestNpColumnBloqueCentrado(), TestNpColumnCaratulaAmbosRenderers() (+7 more)

### Community 41 - "Release Roadmap and Scanning"
Cohesion: 0.15
Nodes (15): Hallazgo #4 (IO dentro de d.mu), refutado midiendo, Release 1.12.0 — auditoría de UX (P0+P1+P2), Release 1.15.0 — buscador de descargas (ctrl+g), Release 1.16.2 — atajos compartidos y tres P3, Release 1.7.3 — SearchLimit y el completado del shell, Roadmap de releases, completeTracks, daemon.dispatch (+7 more)

### Community 42 - "Download Directory Diffing"
Cohesion: 0.29
Nodes (13): Cleanup(), NewAudio(), NewAudioAll(), NewSubdir(), Snapshot(), TestCleanupSoloBasuraNueva(), TestNewAudioDetectaLaDescarga(), TestNewAudioVarias() (+5 more)

### Community 43 - "Playlist Storage"
Cohesion: 0.23
Nodes (4): Library, qualCols(), Playlist, PlaylistNotFound

### Community 46 - "Visualizer Capture Tests"
Cohesion: 0.33
Nodes (11): fakeCaptureCandidate(), newTestViz(), TestArmRetryDoesNotDoubleArm(), TestBarsGravity(), TestCloseIsIdempotent(), TestCloseStopsRetryImmediately(), TestFFTBarsDominantBand(), TestReadLoopArmsRetryOnlyWithBackend() (+3 more)

### Community 47 - "Control Char Sanitizing and M3U"
Cohesion: 0.29
Nodes (4): Library, Clean(), TestClean(), TestCleanBarridoDeControles()

### Community 48 - "Getter Options and Pick"
Cohesion: 0.24
Nodes (9): Release 1.16.0 — marcado ✓, notLive y get pick, downloadOne, newDirEntry(), TestGetPlaylistAmbiguous(), internal/getter — coordinación de yt-dlp, getter.Cleanup, notLive, getter.Opts (+1 more)

### Community 49 - "IPC Client Commands"
Cohesion: 0.31
Nodes (8): printQueue(), printScanLine(), printStatus(), resolveQueryPath(), runClient(), runKill(), TestRemoveCmdParsing(), TestResolveQueryPath()

### Community 50 - "Lyric and Viz Styling"
Cohesion: 0.36
Nodes (5): github.com/charmbracelet/lipgloss.Style, logoRamp(), blendColor(), lyricsPulse(), TestLyricsPulseInRange()

### Community 51 - "Security Audits and IPC"
Cohesion: 0.25
Nodes (9): Release 1.11.0 — auditoría de seguridad desde cero, Release 1.13.0 — cinco fases de auditoría, Modelo de amenaza: mismo UID = mismo nivel de confianza, daemon.handle, recoverResponse, daemon.serve, internal/ipc — protocolo y cliente, ipc.Client (+1 more)

### Community 52 - "Text Sanitizing Invariants"
Cohesion: 0.32
Nodes (8): Invariante: ningún carácter de control llega al terminal ni al bus D-Bus, Release 1.6.0 — segunda auditoría de seguridad, acquireLock (lock.go, flock), library.scanTrack, internal/media — extracción de lo embebido, media.ParseLRC, mpris/art.go — cache de carátulas, internal/safetext — Clean

### Community 53 - "Packaging and Update Channel"
Cohesion: 0.29
Nodes (7): Invariante: maly nunca abre una conexión a internet por su cuenta, internal/update, installerURL(ref), internal/version, version.Packaged / isPackagedPath, PKGBUILD del AUR (paquete maly), Unit de systemd --user (maly.service)

### Community 54 - "Selector Model"
Cohesion: 0.29
Nodes (4): github.com/charmbracelet/bubbletea.Msg, loadPlaylists(), loadLibrary(), selectModel

### Community 55 - "Library Scanning"
Cohesion: 0.30
Nodes (5): ReadTags(), TestReadTags(), TestReadTagsSaneaControles(), underRoot(), ScanResult

### Community 56 - "ffprobe Duration Probe"
Cohesion: 0.46
Nodes (6): Duration(), fakeFfprobe(), TestAvailable(), TestDuration(), TestDurationFailingFile(), TestDurationNotAvailable()

### Community 57 - "Progress Bar Tests"
Cohesion: 0.46
Nodes (7): TestProgressBarAnchoExacto(), TestProgressBarCasosLimite(), TestProgressBarEstelaYPista(), TestProgressBarMonotona(), TestProgressBarSubCelda(), TestProgressShadowDesplazada(), testTheme()

### Community 58 - "Progress Bar Rendering"
Cohesion: 0.62
Nodes (6): Theme, progColor(), progHex(), progressBar(), progressShadow(), progStep()

### Community 59 - "TUI Error Wrapping"
Cohesion: 0.40
Nodes (3): errWrap(), TestEmbeddedStartErrTraduceAlreadyRunning(), wrapErr

### Community 60 - "Track Resolution"
Cohesion: 0.47
Nodes (4): Daemon, trackFromFile(), tracksFromDir(), IsAudio()

### Community 61 - "README and Changelog"
Cohesion: 0.50
Nodes (5): CHANGELOG.md — Registro público de releases, README.md (español), Arquitectura: demonio + clientes sobre socket Unix, README.en.md (inglés), Créditos: herramientas y librerías coordinadas

### Community 62 - "External Tool Philosophy"
Cohesion: 0.50
Nodes (5): Invariante: ningún valor no finito llega a mpv, Integración con Matugen, revertida entera (1.10.2), Filosofía: coordinar herramientas externas, no reimplementarlas, d.seek, internal/player — wrapper de mpv

### Community 63 - "Logo Tick Commands"
Cohesion: 0.50
Nodes (4): logoTickCmd(), splashCmd(), logoTickMsg, splashDoneMsg

### Community 64 - "Key Conflict Detection"
Cohesion: 0.50
Nodes (4): KeyConflict, KeyConflicts(), TestKeyConflictsAgrupaYOrdena(), TestKeyConflictsSinColision()

## Ambiguous Edges - Review These
- `libraryStats` → `Release 1.16.2 — atajos compartidos y tres P3`  [AMBIGUOUS]
  CLAUDE.md · relation: references

## Knowledge Gaps
- **31 isolated node(s):** `selAddMsg`, `selDoneMsg`, `subDeadMsg`, `updMsg`, `conMsg` (+26 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **11 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **What is the exact relationship between `libraryStats` and `Release 1.16.2 — atajos compartidos y tres P3`?**
  _Edge tagged AMBIGUOUS (relation: references) - confidence is low._
- **Why does `T()` connect `Doctor Diagnostics Checks` to `Playlist CLI Commands`, `Shell Completion Engine`, `Daemon Core Dispatch`, `Image and Lyrics Decoding`, `Track Title Cleanup`, `Library Open and Benchmarks`, `mpv Player Wrapper`, `Config Paths and Values`, `TUI Model Tests`, `Update Check and Install`, `TUI Model and Tickers`, `i18n and Player Test Fakes`, `Config Loading Tests`, `Design Decisions and Invariants`, `CLI Command Table and Help`, `TUI Playlist Commands`, `Info and Config Output`, `Console Command Handlers`, `TUI View Rendering`, `Now Playing Lyrics Panel`, `CLI Entry Point`, `View Layout Tests`, `Download Directory Diffing`, `Download Search Screen`, `TUI Key Handling`, `Getter Options and Pick`, `IPC Client Commands`, `Selector Model`, `Language Selector`, `Now Playing Column`, `Song Picker Screen`?**
  _High betweenness centrality (0.111) - this node is a cross-community bridge._
- **Why does `Tf()` connect `Doctor Diagnostics Checks` to `Playlist CLI Commands`, `Shell Completion Engine`, `MPRIS D-Bus Service`, `Daemon Core Dispatch`, `Track Title Cleanup`, `Library Open and Benchmarks`, `mpv Player Wrapper`, `Config Paths and Values`, `Update Check and Install`, `yt-dlp Remote Search`, `TUI Model and Tickers`, `i18n and Player Test Fakes`, `Config Loading Tests`, `TUI Playlist Commands`, `Console Palette Tests`, `Info and Config Output`, `SQLite Library Core`, `Console Command Handlers`, `TUI View Rendering`, `Now Playing Lyrics Panel`, `CLI Entry Point`, `Playlist Storage`, `Download Search Screen`, `Control Char Sanitizing and M3U`, `IPC Client Commands`, `Library Scanning`, `ffprobe Duration Probe`, `Song Picker Screen`?**
  _High betweenness centrality (0.095) - this node is a cross-community bridge._
- **Why does `installerURL(ref)` connect `Packaging and Update Channel` to `Interactive Installer Script`?**
  _High betweenness centrality (0.044) - this node is a cross-community bridge._
- **Are the 23 inferred relationships involving `Open()` (e.g. with `BenchmarkScan()` and `BenchmarkSearchLimit()`) actually correct?**
  _`Open()` has 23 INFERRED edges - model-reasoned connections that need verification._
- **What connects `selAddMsg`, `selDoneMsg`, `subDeadMsg` to the rest of the system?**
  _31 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Playlist CLI Commands` be split into smaller, more focused modules?**
  _Cohesion score 0.05375139977603583 - nodes in this community are weakly interconnected._