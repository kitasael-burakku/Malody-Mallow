# Graph Report - maly  (2026-08-19)

## Corpus Check
- cluster-only mode — file stats not available

## Summary
- 1574 nodes · 4396 edges · 80 communities (70 shown, 10 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 546 edges (avg confidence: 0.85)
- Token cost: 86,127 input · 1,221 output

## Graph Freshness
- Built from commit: `58481e2b`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Daemon Core Dispatch
- TUI Model Update Loop
- Config Loading and TUI Entry
- MPRIS D-Bus Integration
- SQLite Library Core
- CLI Command Table and Help
- Cover Art and Lyrics Parsing
- Queue Shuffle and Repeat Tests
- Project Docs and CI
- mpv Player Process Wrapper
- Graphify Knowledge Graph Tooling
- yt-dlp Download CLI Tests
- Client and i18n Tests
- Interactive Shell Installer
- Remote Search via yt-dlp JSON
- TUI Console Command Handlers
- Daemon Integration Tests
- Fuzzy Picker Component
- Playback Queue
- Playlist Database Operations
- D-Bus Properties Implementation
- TUI View Rendering
- CLI Client and Diagnostics
- Download Directory Diffing
- Now Playing Layer
- Download Search Screen Tests
- Track Title Display Cleanup
- Daemon Architecture Notes
- CLI Entry Point
- Console Panel Tests
- Doctor Verdicts and i18n
- Control Character Sanitization
- Pure Layout Computation
- TUI Playlist Panel
- Console Diagnostics Tests
- MPRIS Album Art Cache
- TUI Architecture Notes
- TUI Layout Regression Tests
- Daemon Track Resolution
- Now Playing Rendering Tests
- CLI Diagnostics and Getter Docs
- yt-dlp Command Building
- Theme Color Derivation
- Download Search Screen Logic
- Project Design Decisions
- CLI Playlist Commands
- TUI Styles and Contrast
- Session Persistence
- Song Picker and Select Mode
- Lyric and Gradient Styling
- Console Rendering Helpers
- Search and Library Helper Docs
- Doctor Checks and XDG Paths
- ffprobe Duration Probing
- Audio Capture and FFT Visualizer
- Keybinding Control Presets
- Progress Bar Tests
- Text Safety and Media Docs
- Progress Bar Rendering
- Release Update Checking
- IPC Socket Client Tests
- Library and Scan Docs
- Search Result Item Formatting
- Now Playing Column
- Bash Completion Script
- Model Progress Bar Methods
- Track Info Formatting
- i18n Translation Table
- IPC Client
- maly Root
- ASCII Logo Banner Model
- TUI Message Types and Ticks
- Visualizer Capture Tests
- get pick Screen
- Logo Splash Animation

## God Nodes (most connected - your core abstractions)
1. `T()` - 106 edges
2. `Tf()` - 82 edges
3. `Open()` - 55 edges
4. `Load()` - 54 edges
5. `Daemon` - 41 edges
6. `Model` - 41 edges
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
- **Flujo de descarga: elegir, bajar, verificar y reindexar** — claude_tui_get, claude_getter_search, claude_getter_opts, claude_getter_diff, claude_getter_cleanup, claude_get_go, claude_daemon_scan [EXTRACTED 0.85]
- **Cadena del gapless: promesa de cola, ventana de mpv y avance** — claude_queue_peeknext, claude_player_setnext, claude_daemon_handle, claude_daemon_advance, claude_daemon_learnduration [EXTRACTED 0.85]
- **Fronteras de saneado de texto ajeno con safetext.Clean** — claude_internal_safetext, claude_library_scantrack, claude_media_parselrc, claude_getter_search, claude_internal_ipc, claude_invariante_control_chars [EXTRACTED 0.90]
- **Pipeline de extracción de graphify (detect → AST + semántico → merge → build)** — claude_skills_graphify_step2_detect, claude_skills_graphify_part_a_ast, claude_skills_graphify_part_b_semantic, claude_skills_graphify_part_c_merge, claude_skills_graphify_step4_build, claude_skills_graphify_step5_labels, claude_skills_graphify_step9_manifest [EXTRACTED 0.95]
- **maly coordina herramientas externas en vez de reimplementarlas** — readme_malody_mallow, readme_credits_mpv, readme_credits_ytdlp, readme_credits_ffmpeg, readme_credits_pipewire [EXTRACTED 0.95]
- **Guardas de integridad del grafo entre builds incrementales** — claude_skills_graphify_shrink_guard, claude_skills_graphify_step45_health, claude_skills_graphify_node_id_format, claude_skills_graphify_source_file_rule, claude_skills_graphify_build_merge [INFERRED 0.85]

## Communities (80 total, 10 thin omitted)

### Community 0 - "Daemon Core Dispatch"
Cohesion: 0.06
Nodes (28): subscriber, bufio.Reader, bufio.Scanner, net.Conn, net.Listener, sync/atomic.Bool, sync/atomic.Int64, sync/atomic.Uint64 (+20 more)

### Community 1 - "TUI Model Update Loop"
Cohesion: 0.21
Nodes (5): context.CancelFunc, github.com/charmbracelet/bubbletea.KeyMsg, Model, vizTickCmd(), vizTickMsg

### Community 2 - "Config Loading and TUI Entry"
Cohesion: 0.06
Nodes (63): orUnset(), runConfig(), embeddedStartErr(), runSelect(), runTUI(), errWrap(), TestEmbeddedStartErrTraduceAlreadyRunning(), KeyConflict (+55 more)

### Community 3 - "MPRIS D-Bus Integration"
Cohesion: 0.05
Nodes (39): github.com/godbus/dbus/v5/introspect.Node, Status, BusAvailable(), dbus.Conn, dbus.Error, dbus.ObjectPath, dbus.Variant, Service (+31 more)

### Community 4 - "SQLite Library Core"
Cohesion: 0.14
Nodes (16): database/sql.DB, clampInt(), Fold(), Library, Track, ReadTags(), scanTrack(), TestFold() (+8 more)

### Community 5 - "CLI Command Table and Help"
Cohesion: 0.06
Nodes (55): client(), helpKeys(), helpText(), lookup(), padRight(), runHelp(), runVersion(), serviceVersion() (+47 more)

### Community 6 - "Cover Art and Lyrics Parsing"
Cohesion: 0.10
Nodes (26): image.Image, image.RGBA, testing.F, DecodeImage(), dimsOK(), ScaleBox(), LyricLine, LyricsFor() (+18 more)

### Community 7 - "Queue Shuffle and Repeat Tests"
Cohesion: 0.25
Nodes (23): New(), checkOrder(), mk(), TestMove(), TestMovePromiseFollowsTrack(), TestNextRepeatOff(), TestPeekInvalidatedByMutation(), TestPeekNextSequential() (+15 more)

### Community 8 - "Project Docs and CI"
Cohesion: 0.08
Nodes (45): CI job: race, CI job: test, Captura: Command Palette de la TUI (ctrl+p), Captura: escritorio Hyprland con la TUI, fastfetch y Waybar, Captura: capa "Ahora suena" a pantalla completa (ctrl+t), Captura: selector de canciones difuso (ctrl+o), Captura: pantalla principal de la TUI (tres columnas), CHANGELOG.md (+37 more)

### Community 9 - "mpv Player Process Wrapper"
Cohesion: 0.10
Nodes (16): encoding/json.RawMessage, os/exec.Cmd, sync.Mutex, sync.WaitGroup, Player, mpvStartError(), reapStale(), safeCall() (+8 more)

### Community 10 - "Graphify Knowledge Graph Tooling"
Cohesion: 0.09
Nodes (37): Integración graphify en CLAUDE.md, Modos de traversal BFS y DFS, build_merge (merge incremental), --cluster-only, Update solo de código (sin LLM), Rúbrica de confidence_score, Prohibición de aristas calls entre lenguajes, Fast path: grafo existente (+29 more)

### Community 11 - "yt-dlp Download CLI Tests"
Cohesion: 0.06
Nodes (69): runGet(), runGetPlaylist(), captureStderr(), failingYtdlp(), fakeArgs(), getPlaylistFailingSandbox(), getPlaylistSandbox(), getSandbox() (+61 more)

### Community 12 - "Client and i18n Tests"
Cohesion: 0.13
Nodes (23): TestRemoveCmdParsing(), TestResolveQueryPath(), testing.T, TestSetCode(), TestT(), TestTableIntegrity(), TestTf(), TestTL() (+15 more)

### Community 13 - "Interactive Shell Installer"
Cohesion: 0.12
Nodes (29): ask(), banner(), cleanup(), confirm(), dep_label(), die(), fetch(), fetch_show() (+21 more)

### Community 14 - "Remote Search via yt-dlp JSON"
Cohesion: 0.20
Nodes (24): searchEntry, context.Context, cmdErr(), decodeResults(), firstLine(), Result, Search(), emit() (+16 more)

### Community 15 - "TUI Console Command Handlers"
Cohesion: 0.21
Nodes (6): github.com/charmbracelet/bubbletea.Cmd, github.com/charmbracelet/bubbletea.Model, Model, Model, getSpinCmd(), langLabel()

### Community 16 - "Daemon Integration Tests"
Cohesion: 0.15
Nodes (32): SocketPath(), newTestDaemon(), next(), TestAdvanceAllBadStops(), TestAdvanceSkipsBadTrack(), TestConcurrentClientsStress(), TestGaplessChain(), TestGaplessRepeatOne() (+24 more)

### Community 17 - "Fuzzy Picker Component"
Cohesion: 0.10
Nodes (23): github.com/charmbracelet/bubbles/textinput.Model, TestPickModelFlujo(), TestPickerCtrlDU(), TestPickerDistingueBibliotecaVacia(), TestSongsViewMuestraFlash(), newPicker(), newPickerItem(), linesFitWithin() (+15 more)

### Community 19 - "Playlist Database Operations"
Cohesion: 0.23
Nodes (4): Library, qualCols(), Playlist, PlaylistNotFound

### Community 20 - "D-Bus Properties Implementation"
Cohesion: 0.17
Nodes (15): github.com/godbus/dbus/v5/introspect.Property, exportProps(), dbus.Conn, dbus.Error, dbus.ObjectPath, dbus.Variant, dbus.Error, newTestProps() (+7 more)

### Community 21 - "TUI View Rendering"
Cohesion: 0.20
Nodes (5): TestClipPadTo(), clip(), padTo(), Model, marker()

### Community 22 - "CLI Client and Diagnostics"
Cohesion: 0.12
Nodes (19): printQueue(), printScanLine(), printStatus(), resolveQueryPath(), runClient(), runDaemon(), runKill(), libraryStats() (+11 more)

### Community 23 - "Download Directory Diffing"
Cohesion: 0.18
Nodes (18): newDirEntry(), TestGetPlaylistAmbiguous(), Cleanup(), NewAudio(), NewAudioAll(), NewSubdir(), Snapshot(), TestCleanupSoloBasuraNueva() (+10 more)

### Community 24 - "Now Playing Layer"
Cohesion: 0.17
Nodes (11): ReadEmbedded(), activeLyric(), center(), Model, loadNowMeta(), lyricDistance(), npDetail(), TestActiveLyric() (+3 more)

### Community 25 - "Download Search Screen Tests"
Cohesion: 0.19
Nodes (19): boxLinesSameWidth(), fakeTools(), Model, key(), newGetModel(), TestGetDescargaCierraYUsaStartGet(), TestGetEnterBusca(), TestGetEnterBuscaCuandoElTextoCambio() (+11 more)

### Community 26 - "Track Title Display Cleanup"
Cohesion: 0.06
Nodes (50): allNoise(), cleanTitle(), collapseSpaces(), cutAtSep(), dropArtistPrefix(), stripNoiseTags(), TestArbolYColaMuestranElTituloLimpio(), TestCleanTitleCasosReales() (+42 more)

### Community 27 - "Daemon Architecture Notes"
Cohesion: 0.10
Nodes (23): CI en GitHub Actions (build/vet/test + race), acquireLock — identidad del demonio por flock, advance — política de avance y salto de pistas, dispatch — switch plano de comandos bajo d.mu, handle — reflejo a MPRIS y suscriptores, learnDuration — aprende duración desde mpv, serve — bucle de conexiones con deadlines, session.go — sesión persistida en JSON atómico (+15 more)

### Community 28 - "CLI Entry Point"
Cohesion: 0.19
Nodes (14): envLangHint(), langName(), main(), openLibrary(), printTracks(), runLang(), runLogo(), runScan() (+6 more)

### Community 29 - "Console Panel Tests"
Cohesion: 0.19
Nodes (18): boxLinesFitWithin(), Model, newConModel(), TestConHelpNoOverflow(), TestConsoleCommandsSonReales(), TestConsoleControlsList(), TestConsoleHistorial(), TestConsolePlaylistRoundTrip() (+10 more)

### Community 30 - "Doctor Verdicts and i18n"
Cohesion: 0.27
Nodes (14): checkKeys(), checkLibrary(), checkMpv(), checkMusicDir(), checkOptionalTools(), checkService(), checkUpdate(), runDoctor() (+6 more)

### Community 31 - "Control Character Sanitization"
Cohesion: 0.29
Nodes (4): Library, Clean(), TestClean(), TestCleanBarridoDeControles()

### Community 32 - "Pure Layout Computation"
Cohesion: 0.21
Nodes (14): clampInt(), computeLayout(), Model, allOpts(), TestComputeLayoutAnchosDeColumna(), TestComputeLayoutBarraSiempre(), TestComputeLayoutDegenerado(), TestComputeLayoutInvariantes() (+6 more)

### Community 33 - "TUI Playlist Panel"
Cohesion: 0.16
Nodes (16): github.com/charmbracelet/bubbletea.Msg, DBPath(), newDirEntry(), TestCtrlXPidenConfirmacion(), Model, loadPlaylists(), notifyRefresh(), plAddCmd() (+8 more)

### Community 34 - "Console Diagnostics Tests"
Cohesion: 0.29
Nodes (10): libStatsTUI(), openLibIfExists(), TestConConfig(), TestConInfo(), TestDiagServiceVersionNoDaemonNoLock(), TestLibStatsTUIConDB(), TestLibStatsTUINoDB(), TestOpenLibIfExistsDoesNotCreate() (+2 more)

### Community 35 - "MPRIS Album Art Cache"
Cohesion: 0.23
Nodes (10): newArtCache(), safeExt(), id3v2WithCover(), TestArtCache(), TestArtCacheEvicta(), TestSafeExt(), TestServiceMetadataArt(), writeTrack() (+2 more)

### Community 36 - "TUI Architecture Notes"
Cohesion: 0.21
Nodes (14): commands.go — tabla única de comandos, saveKey — edita claves dentro de secciones TOML, internal/tui — interfaz Bubble Tea, Contrato de panel(): el llamador garantiza el ancho, Red de paridad consola↔CLI (ConsoleCommands), artkitty.go — carátula por protocolo gráfico de kitty, computeLayout — reparto puro de la pantalla, console.go — consola ctrl+p (+6 more)

### Community 37 - "TUI Layout Regression Tests"
Cohesion: 0.25
Nodes (14): Model, newLayoutTestModel(), TestBarraDeProgresoUnaSola(), TestFiltroNoRompeElBorde(), TestInvalidateArtTiraLosDosRenders(), TestNpColumnAprovechaElAncho(), TestNpColumnBloqueCentrado(), TestNpColumnCaratulaAmbosRenderers() (+6 more)

### Community 38 - "Daemon Track Resolution"
Cohesion: 0.43
Nodes (4): Daemon, trackFromFile(), tracksFromDir(), TLf()

### Community 39 - "Now Playing Rendering Tests"
Cohesion: 0.25
Nodes (13): TestActiveLyricStyleFrozenWhenNotPlaying(), TestHalfBlocksRender(), TestNpLyricsClamp(), TestNpLyricsLinesActiveFrozenWhenPaused(), TestNpLyricsLinesBlanksBeyondMaxDistance(), TestNpLyricsLinesNoBlankWithinNaturalWindow(), TestNpMetaTimeLineNoOverflow(), TestNpViewAvisaCaratulaOcultaPorAncho() (+5 more)

### Community 40 - "CLI Diagnostics and Getter Docs"
Cohesion: 0.22
Nodes (15): cmd/maly — CLI y punto de entrada, maly config — configuración efectiva, Miniaturas y pantalla completa del buscador, descartadas, doctor.go — diagnóstico de VEREDICTOS, cmd/maly/get.go — wrapper CLI de yt-dlp, getter.Cleanup — borra intermedios huérfanos, getter/diff.go — verificación por diff de directorio, getter.Opts — contrato de --no-playlist / --yes-playlist (+7 more)

### Community 41 - "yt-dlp Command Building"
Cohesion: 0.20
Nodes (13): downloadOne(), runGetPick(), Opts, Command(), lookTool(), Spec(), TestCommand(), TestCommandCookies() (+5 more)

### Community 42 - "Theme Color Derivation"
Cohesion: 0.26
Nodes (9): BlendHex(), ParseHex(), scaleHex(), blendHex(), parseHex(), pulseColor(), TestBlendHexGradation(), TestPulseColorLevelOneLightensTowardWhite() (+1 more)

### Community 44 - "Project Design Decisions"
Cohesion: 0.17
Nodes (16): CLAUDE.md — Documento de ingeniería del proyecto, Theme.ResolveDerived — colores derivados de accent, Firma de releases (minisign/GPG), fuera de alcance, Ratón en la TUI, descartado, internal/config — carga y merge de configuración, internal/viz — captura de audio y FFT, mallow-install.sh — instalador interactivo, Malody Mallow (maly) — reproductor de música local en terminal (+8 more)

### Community 45 - "CLI Playlist Commands"
Cohesion: 0.33
Nodes (9): confirmOverwrite(), confirmYesNo(), isTTY(), notifyRefresh(), runPlaylist(), shouldDeletePlaylist(), TestPlaylistAddNoExisteSugiereCreate(), TestPlaylistExportNoClobber() (+1 more)

### Community 46 - "TUI Styles and Contrast"
Cohesion: 0.27
Nodes (9): github.com/charmbracelet/lipgloss.Color, contrastFg(), selectedFg(), TestBordeSinFocoUsaAccentDim(), TestErrorColorConfigurable(), TestSelectedFgConservaElTextoDelTema(), TestSelectedFgContrast(), TestSelectedHasExplicitBackground() (+1 more)

### Community 47 - "Session Persistence"
Cohesion: 0.48
Nodes (3): session, Daemon, saveSession()

### Community 48 - "Song Picker and Select Mode"
Cohesion: 0.20
Nodes (3): pickerWidth(), Model, selectModel

### Community 49 - "Lyric and Gradient Styling"
Cohesion: 0.36
Nodes (5): github.com/charmbracelet/lipgloss.Style, logoRamp(), blendColor(), lyricsPulse(), TestLyricsPulseInRange()

### Community 50 - "Console Rendering Helpers"
Cohesion: 0.24
Nodes (9): formatHelpRow(), queueLines(), searchLines(), TestFormatHelpRowClipsPorColumna(), conMsg, getDoneMsg, getPlaylistDoneMsg, styles (+1 more)

### Community 51 - "Search and Library Helper Docs"
Cohesion: 0.33
Nodes (7): CHANGELOG.md — Registro público de releases, notLive — descarta directos y estrenos, getter.Search — búsqueda remota vía --dump-json, ViewCount — visitas en los resultados, library.OpenIfExists — abrir sin crear la base, libraryStats — estadísticas de biblioteca compartidas, get_pick.go — pantalla de maly get pick

### Community 52 - "Doctor Checks and XDG Paths"
Cohesion: 0.13
Nodes (26): TestCheckKeys(), TestCheckLibraryNoDB(), TestCheckServiceNoDaemonNoLock(), TestRunLogo(), Daemon, os.File, DataDir(), Default() (+18 more)

### Community 53 - "ffprobe Duration Probing"
Cohesion: 0.42
Nodes (7): Available(), Duration(), fakeFfprobe(), TestAvailable(), TestDuration(), TestDurationFailingFile(), TestDurationNotAvailable()

### Community 54 - "Audio Capture and FFT Visualizer"
Cohesion: 0.15
Nodes (11): gonum.org/v1/gonum/dsp/fourier.FFT, io.Reader, sync.Once, time.Time, CaptureBackend(), filterCandidates(), Viz, New() (+3 more)

### Community 55 - "Keybinding Control Presets"
Cohesion: 0.40
Nodes (5): completeControls(), runControls(), PresetNames(), SaveControls(), ValidPreset()

### Community 56 - "Progress Bar Tests"
Cohesion: 0.46
Nodes (7): TestProgressBarAnchoExacto(), TestProgressBarCasosLimite(), TestProgressBarEstelaYPista(), TestProgressBarMonotona(), TestProgressBarSubCelda(), TestProgressShadowDesplazada(), testTheme()

### Community 57 - "Text Safety and Media Docs"
Cohesion: 0.33
Nodes (7): internal/media — carátula y letras embebidas, internal/safetext — Clean de caracteres de control, Invariante: ningún carácter de control llega al terminal ni a D-Bus, scanTrack — punto único de salida de la biblioteca, dimsOK — guarda anti-bomba de descompresión, ParseLRC — parser de letras sincronizadas, art.go — caché de carátulas acotado a 32 MB

### Community 58 - "Progress Bar Rendering"
Cohesion: 0.62
Nodes (6): Theme, progColor(), progHex(), progressBar(), progressShadow(), progStep()

### Community 59 - "Release Update Checking"
Cohesion: 0.22
Nodes (16): updateCheckCmd(), Cached(), cachePath(), installerURL(), Latest(), latestTag(), Newer(), parse() (+8 more)

### Community 60 - "IPC Socket Client Tests"
Cohesion: 0.35
Nodes (13): Dial(), Ping(), serve(), TestDoAcceptsLargeLegitimateResponse(), TestDoBoundedRead(), TestDoInvalidResponse(), TestDoRecoversAfterTimeoutOnSameClient(), TestDoRejectsTruncatedResponse() (+5 more)

### Community 61 - "Library and Scan Docs"
Cohesion: 0.38
Nodes (7): scan del demonio — dos fases fuera de d.mu, internal/library — SQLite modernc sin CGo, Disciplina: medir antes de tocar, y medir el efecto observable, FillDurations — segunda fase del scan con ffprobe, Fold — plegado Unicode para búsqueda, library.SearchLimit — búsqueda con tope en SQL, tree.go — árbol de la biblioteca

### Community 62 - "Search Result Item Formatting"
Cohesion: 0.21
Nodes (11): fmtViews(), getItems(), oneLine(), ownedKey(), ownedTitles(), TestFmtViews(), TestGetItemsMeta(), TestOwnedBibliotecaVacia() (+3 more)

### Community 76 - "TUI Message Types and Ticks"
Cohesion: 0.18
Nodes (12): TestRechequeoDeUpdate(), keyRepeats(), tickCmd(), updTickCmd(), actionMsg, panelID, statusMsg, subDeadMsg (+4 more)

### Community 77 - "Visualizer Capture Tests"
Cohesion: 0.33
Nodes (11): fakeCaptureCandidate(), newTestViz(), TestArmRetryDoesNotDoubleArm(), TestBarsGravity(), TestCloseIsIdempotent(), TestCloseStopsRetryImmediately(), TestFFTBarsDominantBand(), TestReadLoopArmsRetryOnlyWithBackend() (+3 more)

### Community 78 - "get pick Screen"
Cohesion: 0.29
Nodes (9): ownedLibraryTitles(), pickBody(), pickedResult(), pickHint(), RunGetPick(), TestPickBodyYHint(), TestPickedResultSigueAlCursor(), getPhase (+1 more)

### Community 79 - "Logo Splash Animation"
Cohesion: 0.25
Nodes (7): logoTickCmd(), newLogo(), splashCmd(), TestNewLogoArt(), Run(), logoTickMsg, splashDoneMsg

## Knowledge Gaps
- **35 isolated node(s):** `splashDoneMsg`, `subDeadMsg`, `updMsg`, `Model`, `selAddMsg` (+30 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **10 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `T()` connect `Doctor Verdicts and i18n` to `Daemon Core Dispatch`, `TUI Model Update Loop`, `Config Loading and TUI Entry`, `CLI Command Table and Help`, `mpv Player Process Wrapper`, `yt-dlp Download CLI Tests`, `Client and i18n Tests`, `TUI Console Command Handlers`, `Fuzzy Picker Component`, `TUI View Rendering`, `CLI Client and Diagnostics`, `Download Directory Diffing`, `Now Playing Layer`, `Track Title Display Cleanup`, `CLI Entry Point`, `TUI Playlist Panel`, `TUI Layout Regression Tests`, `yt-dlp Command Building`, `Download Search Screen Logic`, `CLI Playlist Commands`, `Song Picker and Select Mode`, `Console Rendering Helpers`, `Keybinding Control Presets`, `Release Update Checking`, `Now Playing Column`, `get pick Screen`, `Logo Splash Animation`?**
  _High betweenness centrality (0.081) - this node is a cross-community bridge._
- **Why does `Tf()` connect `CLI Client and Diagnostics` to `Daemon Core Dispatch`, `TUI Model Update Loop`, `Config Loading and TUI Entry`, `MPRIS D-Bus Integration`, `SQLite Library Core`, `CLI Command Table and Help`, `mpv Player Process Wrapper`, `yt-dlp Download CLI Tests`, `Client and i18n Tests`, `Remote Search via yt-dlp JSON`, `TUI Console Command Handlers`, `Playlist Database Operations`, `TUI View Rendering`, `Now Playing Layer`, `CLI Entry Point`, `Console Panel Tests`, `Doctor Verdicts and i18n`, `Control Character Sanitization`, `TUI Playlist Panel`, `yt-dlp Command Building`, `Download Search Screen Logic`, `CLI Playlist Commands`, `Song Picker and Select Mode`, `Doctor Checks and XDG Paths`, `ffprobe Duration Probing`, `Keybinding Control Presets`, `Release Update Checking`, `get pick Screen`?**
  _High betweenness centrality (0.062) - this node is a cross-community bridge._
- **Why does `Open()` connect `yt-dlp Download CLI Tests` to `TUI Playlist Panel`, `Console Diagnostics Tests`, `SQLite Library Core`, `CLI Command Table and Help`, `CLI Playlist Commands`, `get pick Screen`, `Fuzzy Picker Component`, `Doctor Checks and XDG Paths`, `CLI Client and Diagnostics`, `CLI Entry Point`, `Doctor Verdicts and i18n`?**
  _High betweenness centrality (0.047) - this node is a cross-community bridge._
- **Are the 23 inferred relationships involving `Open()` (e.g. with `BenchmarkScan()` and `BenchmarkSearchLimit()`) actually correct?**
  _`Open()` has 23 INFERRED edges - model-reasoned connections that need verification._
- **What connects `splashDoneMsg`, `subDeadMsg`, `updMsg` to the rest of the system?**
  _35 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Daemon Core Dispatch` be split into smaller, more focused modules?**
  _Cohesion score 0.06436487638533675 - nodes in this community are weakly interconnected._
- **Should `Config Loading and TUI Entry` be split into smaller, more focused modules?**
  _Cohesion score 0.06442058496853018 - nodes in this community are weakly interconnected._