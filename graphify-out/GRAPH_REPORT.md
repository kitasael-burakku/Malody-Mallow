# Graph Report - maly  (2026-08-20)

## Corpus Check
- 122 files · ~423,943 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1544 nodes · 4384 edges · 74 communities (66 shown, 8 thin omitted)
- Extraction: 87% EXTRACTED · 13% INFERRED · 0% AMBIGUOUS · INFERRED: 550 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `79ed5d8a`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- daemon_test.go
- DBPath
- Status
- Daemon
- Image and Lyrics Decoding
- picker
- Project Docs and CI
- Library Open and Benchmarks
- Player
- Interactive Installer Script
- Load
- buildTree
- update/update.go
- Search
- Model
- testing.T
- EnsureRuntimeDir
- tui/get_test.go
- Queue Shuffle and Repeat Tests
- D-Bus Properties Implementation
- console.go
- commands.go
- doctor.go
- Track
- github.com/charmbracelet/bubbletea.Cmd
- newStyles
- newConModel
- dbus.Error
- mpris_test.go
- github.com/charmbracelet/bubbletea.Model
- Command
- Model
- Album Art Cache
- newPicker
- styles.go
- Layout Computation
- clip
- daemon.New — orden de arranque
- T
- logoModel
- View Layout Tests
- Roadmap de releases
- Snapshot
- Dial
- Model
- fakeCtrl
- Visualizer Capture Tests
- raceSetup
- maly/get.go
- runPlaylist
- Model
- getItems
- Text Sanitizing Invariants
- info.go
- getPickModel
- picker.go
- metadataOf
- Progress Bar Tests
- Theme
- internal/config
- Packaged
- README and Changelog
- Filosofía: coordinar herramientas externas, no reimplementarlas
- Now Playing Column
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
- **Flujo de descarga vía yt-dlp (CLI y TUI)** — internal_getter, internal_getter_search, internal_getter_opts, internal_getter_diff, internal_getter_cleanup, cmd_maly_downloadOne, internal_tui_get, internal_tui_getpick [EXTRACTED 0.90]
- **Fronteras de saneado de texto ajeno** — internal_safetext, internal_library_scantrack, internal_media_parselrc, internal_getter_search, internal_ipc, claude_invariant_no_control_chars [EXTRACTED 0.90]
- **Orden de arranque del demonio (no negociable)** — internal_daemon_new, internal_daemon_acquirelock, internal_library, internal_player_start, internal_daemon_session, internal_mpris [EXTRACTED 0.95]
- **maly coordina herramientas externas en vez de reimplementarlas** — readme_malody_mallow, readme_credits_mpv, readme_credits_ytdlp, readme_credits_ffmpeg, readme_credits_pipewire [EXTRACTED 0.95]

## Communities (74 total, 8 thin omitted)

### Community 0 - "daemon_test.go"
Cohesion: 0.15
Nodes (37): SocketPath(), New(), Daemon, newRawDaemon(), newTestDaemon(), TestAdvanceAllBadStops(), TestAdvanceSkipsBadTrack(), TestCierraLiberaElLock() (+29 more)

### Community 1 - "DBPath"
Cohesion: 0.05
Nodes (77): cand(), completeArgs(), completeCommands(), completeControls(), completeJump(), completeMove(), completePlaylist(), completePlaylistNames() (+69 more)

### Community 2 - "Status"
Cohesion: 0.19
Nodes (13): github.com/godbus/dbus/v5/introspect.Node, Status, BusAvailable(), dbus.Conn, Service, loopOf(), playbackOf(), positionUS() (+5 more)

### Community 3 - "Daemon"
Cohesion: 0.07
Nodes (28): subscriber, bufio.Reader, bufio.Scanner, net.Conn, net.Listener, sync/atomic.Bool, sync/atomic.Int64, sync/atomic.Uint64 (+20 more)

### Community 4 - "Image and Lyrics Decoding"
Cohesion: 0.06
Nodes (38): gonum.org/v1/gonum/dsp/fourier.FFT, image.Image, image.RGBA, io.Reader, sync.Once, testing.F, time.Time, DecodeImage() (+30 more)

### Community 5 - "picker"
Cohesion: 0.36
Nodes (4): github.com/charmbracelet/bubbles/textinput.Model, picker, pickerItem, plListMsg

### Community 6 - "Project Docs and CI"
Cohesion: 0.08
Nodes (46): CI job: race, CI job: test, Captura: Command Palette de la TUI (ctrl+p), Captura: escritorio Hyprland con la TUI, fastfetch y Waybar, Captura: capa "Ahora suena" a pantalla completa (ctrl+t), Captura: selector de canciones difuso (ctrl+o), Captura: pantalla principal de la TUI (tres columnas), CHANGELOG.md (+38 more)

### Community 7 - "Library Open and Benchmarks"
Cohesion: 0.09
Nodes (43): testing.B, testing.TB, BenchmarkScan(), BenchmarkSearchLimit(), Open(), OpenIfExists(), fakeMusicDir(), Library (+35 more)

### Community 8 - "Player"
Cohesion: 0.09
Nodes (17): encoding/json.RawMessage, os/exec.Cmd, sync.Mutex, sync.WaitGroup, Player, mpvStartError(), reapStale(), safeCall() (+9 more)

### Community 9 - "Interactive Installer Script"
Cohesion: 0.12
Nodes (29): ask(), banner(), cleanup(), confirm(), dep_label(), die(), fetch(), fetch_show() (+21 more)

### Community 10 - "Load"
Cohesion: 0.06
Nodes (72): orUnset(), runConfig(), runInfo(), runLogo(), TestRunLogo(), embeddedStartErr(), runSelect(), runTUI() (+64 more)

### Community 11 - "buildTree"
Cohesion: 0.06
Nodes (52): Fold(), TestFold(), allNoise(), cleanTitle(), collapseSpaces(), cutAtSep(), dropArtistPrefix(), stripNoiseTags() (+44 more)

### Community 12 - "update/update.go"
Cohesion: 0.13
Nodes (22): session, DataDir(), Daemon, TestSessionFilePrivate(), saveSession(), sessionPath(), updateCheckCmd(), Cached() (+14 more)

### Community 13 - "Search"
Cohesion: 0.13
Nodes (29): searchEntry, context.Context, cmdErr(), decodeResults(), firstLine(), Result, Search(), emit() (+21 more)

### Community 14 - "Model"
Cohesion: 0.12
Nodes (18): context.CancelFunc, github.com/charmbracelet/bubbletea.KeyMsg, TestRechequeoDeUpdate(), Model, keyRepeats(), tickCmd(), updTickCmd(), vizTickCmd() (+10 more)

### Community 15 - "testing.T"
Cohesion: 0.13
Nodes (24): testing.T, TestPanicWithDeferredUnlockDoesNotLeakMutex(), TestParseAdjustRechazaNoFinitos(), TestRecoverResponseCatchesPanic(), TestRecoverResponsePassesThrough(), TestSetCode(), TestT(), TestTableIntegrity() (+16 more)

### Community 16 - "EnsureRuntimeDir"
Cohesion: 0.22
Nodes (10): TestCheckServiceNoDaemonNoLock(), os.File, EnsureRuntimeDir(), RuntimeDir(), TestEnsureRuntimeDir(), TestNoRoboElSocketDeUnDemonioArrancando(), TestReclamaSocketHuerfano(), acquireLock() (+2 more)

### Community 17 - "tui/get_test.go"
Cohesion: 0.22
Nodes (17): boxLinesSameWidth(), fakeTools(), Model, key(), newGetModel(), TestGetDescargaCierraYUsaStartGet(), TestGetEnterBusca(), TestGetEnterBuscaCuandoElTextoCambio() (+9 more)

### Community 18 - "Queue Shuffle and Repeat Tests"
Cohesion: 0.25
Nodes (23): New(), checkOrder(), mk(), TestMove(), TestMovePromiseFollowsTrack(), TestNextRepeatOff(), TestPeekInvalidatedByMutation(), TestPeekNextSequential() (+15 more)

### Community 19 - "D-Bus Properties Implementation"
Cohesion: 0.17
Nodes (15): github.com/godbus/dbus/v5/introspect.Property, exportProps(), dbus.Conn, dbus.Error, dbus.ObjectPath, dbus.Variant, dbus.Error, newTestProps() (+7 more)

### Community 20 - "console.go"
Cohesion: 0.16
Nodes (13): Red de paridad consola ↔ CLI, FmtTime(), OnOff(), formatHelpRow(), queueLines(), searchLines(), statusLines(), TestFormatHelpRowClipsPorColumna() (+5 more)

### Community 21 - "commands.go"
Cohesion: 0.12
Nodes (19): Release 1.16.2 — atajos compartidos y tres P3, helpKeys(), helpText(), lookup(), padRight(), runHelp(), runVersion(), serviceVersion() (+11 more)

### Community 22 - "doctor.go"
Cohesion: 0.18
Nodes (20): checkKeys(), checkLibrary(), checkMpv(), checkMusicDir(), checkOptionalTools(), checkService(), checkUpdate(), runDoctor() (+12 more)

### Community 23 - "Track"
Cohesion: 0.05
Nodes (25): database/sql.DB, Daemon, trackFromFile(), tracksFromDir(), TLf(), clampInt(), Library, Track (+17 more)

### Community 24 - "github.com/charmbracelet/bubbletea.Cmd"
Cohesion: 0.13
Nodes (16): github.com/charmbracelet/bubbletea.Cmd, github.com/charmbracelet/bubbletea.Msg, getSpinCmd(), Model, loadPlaylists(), notifyRefresh(), plAddCmd(), plCreateCmd() (+8 more)

### Community 25 - "newStyles"
Cohesion: 0.16
Nodes (18): TestActiveLyricStyleFrozenWhenNotPlaying(), TestHalfBlocksRender(), TestNpLyricsClamp(), TestNpLyricsLinesActiveFrozenWhenPaused(), TestNpLyricsLinesBlanksBeyondMaxDistance(), TestNpLyricsLinesNoBlankWithinNaturalWindow(), TestNpMetaTimeLineNoOverflow(), TestNpViewAvisaCaratulaOcultaPorAncho() (+10 more)

### Community 26 - "newConModel"
Cohesion: 0.19
Nodes (18): boxLinesFitWithin(), Model, newConModel(), TestConHelpNoOverflow(), TestConsoleCommandsSonReales(), TestConsoleControlsList(), TestConsoleHistorial(), TestConsolePlaylistRoundTrip() (+10 more)

### Community 27 - "dbus.Error"
Cohesion: 0.16
Nodes (4): dbus.Error, dbus.ObjectPath, player, root

### Community 28 - "mpris_test.go"
Cohesion: 0.21
Nodes (14): newFakeCtrl(), TestLoopOf(), TestPlaybackOf(), TestPlayerMethods(), TestPlayerSeek(), TestPlayerSetPosition(), TestPositionUS(), TestSetLoop() (+6 more)

### Community 29 - "github.com/charmbracelet/bubbletea.Model"
Cohesion: 0.28
Nodes (4): github.com/charmbracelet/bubbletea.Model, Request, Model, langLabel()

### Community 30 - "Command"
Cohesion: 0.24
Nodes (11): Opts, Command(), lookTool(), Spec(), TestCommand(), TestCommandCookies(), TestCommandPlaylist(), TestCommandPlaylistSubdir() (+3 more)

### Community 31 - "Model"
Cohesion: 0.25
Nodes (3): padTo(), Model, marker()

### Community 32 - "Album Art Cache"
Cohesion: 0.18
Nodes (12): ReadEmbedded(), newArtCache(), safeExt(), id3v2WithCover(), TestArtCache(), TestArtCacheEvicta(), TestSafeExt(), TestServiceMetadataArt() (+4 more)

### Community 33 - "newPicker"
Cohesion: 0.26
Nodes (13): TestPickModelFlujo(), TestPickerCtrlDU(), TestPickerDistingueBibliotecaVacia(), TestSongsViewMuestraFlash(), newPicker(), newPickerItem(), linesFitWithin(), pickerWith() (+5 more)

### Community 34 - "styles.go"
Cohesion: 0.21
Nodes (13): github.com/charmbracelet/lipgloss.Color, BlendHex(), ParseHex(), scaleHex(), blendHex(), parseHex(), pulseColor(), TestBlendHexGradation() (+5 more)

### Community 35 - "Layout Computation"
Cohesion: 0.21
Nodes (14): clampInt(), computeLayout(), Model, allOpts(), TestComputeLayoutAnchosDeColumna(), TestComputeLayoutBarraSiempre(), TestComputeLayoutDegenerado(), TestComputeLayoutInvariantes() (+6 more)

### Community 36 - "clip"
Cohesion: 0.18
Nodes (9): TestClipPadTo(), activeLyric(), center(), loadNowMeta(), lyricDistance(), npDetail(), TestActiveLyric(), wrapLyric() (+1 more)

### Community 37 - "daemon.New — orden de arranque"
Cohesion: 0.12
Nodes (18): Arquitectura demonio + clientes, Grafo de conocimiento graphify, Invariante: el demonio y sus hijos mueren juntos, Malody Mallow (maly), Convención de proyecto en español, cmd/maly (CLI), internal/daemon, advance(reason, chained) (+10 more)

### Community 38 - "T"
Cohesion: 0.09
Nodes (26): runDaemon(), downloadOne(), runGetPick(), envLangHint(), langName(), main(), openLibrary(), printTracks() (+18 more)

### Community 39 - "logoModel"
Cohesion: 0.13
Nodes (9): Model, logoRamp(), logoTickCmd(), newLogo(), splashCmd(), TestNewLogoArt(), logoModel, logoTickMsg (+1 more)

### Community 40 - "View Layout Tests"
Cohesion: 0.23
Nodes (15): Model, newLayoutTestModel(), TestBarraDeProgresoUnaSola(), TestFiltroNoRompeElBorde(), TestInvalidateArtTiraLosDosRenders(), TestNpColumnAprovechaElAncho(), TestNpColumnBloqueCentrado(), TestNpColumnCaratulaAmbosRenderers() (+7 more)

### Community 41 - "Roadmap de releases"
Cohesion: 0.15
Nodes (17): Hallazgo #4 (IO dentro de d.mu), refutado midiendo, Release 1.11.0 — auditoría de seguridad desde cero, Release 1.12.0 — auditoría de UX (P0+P1+P2), Release 1.15.0 — buscador de descargas (ctrl+g), Release 1.7.3 — SearchLimit y el completado del shell, Roadmap de releases, Modelo de amenaza: mismo UID = mismo nivel de confianza, completeTracks (+9 more)

### Community 42 - "Snapshot"
Cohesion: 0.24
Nodes (14): Cleanup(), NewAudio(), NewAudioAll(), Snapshot(), TestCleanupSoloBasuraNueva(), TestNewAudioDetectaLaDescarga(), TestNewAudioVarias(), TestNewSubdir() (+6 more)

### Community 43 - "Dial"
Cohesion: 0.37
Nodes (12): Dial(), serve(), TestDoAcceptsLargeLegitimateResponse(), TestDoBoundedRead(), TestDoInvalidResponse(), TestDoRecoversAfterTimeoutOnSameClient(), TestDoRejectsTruncatedResponse(), TestDoRoundTrip() (+4 more)

### Community 46 - "Visualizer Capture Tests"
Cohesion: 0.33
Nodes (11): fakeCaptureCandidate(), newTestViz(), TestArmRetryDoesNotDoubleArm(), TestBarsGravity(), TestCloseIsIdempotent(), TestCloseStopsRetryImmediately(), TestFFTBarsDominantBand(), TestReadLoopArmsRetryOnlyWithBackend() (+3 more)

### Community 47 - "raceSetup"
Cohesion: 0.47
Nodes (11): checkSync(), Daemon, raceSetup(), TestAdvanceGeneracionVigenteAvanza(), TestAdvanceObsoletoCorrompeDuracion(), TestAdvanceObsoletoSalteaPista(), TestAdvanceObsoletoTrasJump(), TestAdvanceObsoletoTrasMove() (+3 more)

### Community 48 - "maly/get.go"
Cohesion: 0.25
Nodes (9): Release 1.16.0 — marcado ✓, notLive y get pick, downloadOne, newDirEntry(), TestGetPlaylistAmbiguous(), internal/getter — coordinación de yt-dlp, getter.Cleanup, NewSubdir(), notLive (+1 more)

### Community 49 - "runPlaylist"
Cohesion: 0.15
Nodes (18): printQueue(), printScanLine(), printStatus(), resolveQueryPath(), runClient(), runKill(), TestRemoveCmdParsing(), TestResolveQueryPath() (+10 more)

### Community 50 - "Model"
Cohesion: 0.40
Nodes (5): github.com/charmbracelet/lipgloss.Style, blendColor(), Model, lyricsPulse(), TestLyricsPulseInRange()

### Community 51 - "getItems"
Cohesion: 0.23
Nodes (11): fmtViews(), getItems(), oneLine(), ownedKey(), ownedTitles(), TestFmtViews(), TestGetItemsMeta(), TestOwnedBibliotecaVacia() (+3 more)

### Community 52 - "Text Sanitizing Invariants"
Cohesion: 0.32
Nodes (8): Invariante: ningún carácter de control llega al terminal ni al bus D-Bus, Release 1.6.0 — segunda auditoría de seguridad, acquireLock (lock.go, flock), library.scanTrack, internal/media — extracción de lo embebido, media.ParseLRC, mpris/art.go — cache de carátulas, internal/safetext — Clean

### Community 53 - "info.go"
Cohesion: 0.21
Nodes (10): Invariante: maly nunca abre una conexión a internet por su cuenta, Release 1.13.0 — cinco fases de auditoría, libraryStats, internal/update, installerURL(ref), internal/version, version.Packaged / isPackagedPath, internal/viz — visualizador (+2 more)

### Community 54 - "getPickModel"
Cohesion: 0.39
Nodes (7): ownedLibraryTitles(), pickBody(), pickHint(), RunGetPick(), TestPickBodyYHint(), getPhase, getPickModel

### Community 55 - "picker.go"
Cohesion: 0.32
Nodes (5): TestPickerWidthMax(), pickerWidth(), pickerWidthMax(), TestPickerWidth(), pickerSource

### Community 56 - "metadataOf"
Cohesion: 0.38
Nodes (6): dbus.Variant, metadataOf(), pathTrackID(), TestMetadataOfLibraryTrack(), TestMetadataOfNoTrack(), TestMetadataOfPathTrack()

### Community 57 - "Progress Bar Tests"
Cohesion: 0.46
Nodes (7): TestProgressBarAnchoExacto(), TestProgressBarCasosLimite(), TestProgressBarEstelaYPista(), TestProgressBarMonotona(), TestProgressBarSubCelda(), TestProgressShadowDesplazada(), testTheme()

### Community 58 - "Theme"
Cohesion: 0.36
Nodes (9): Trampas conocidas al probar en vivo, Disciplina de verificación en ambas direcciones, Theme, internal/tui — Bubble Tea, progColor(), progHex(), progressBar(), progressShadow() (+1 more)

### Community 59 - "internal/config"
Cohesion: 0.33
Nodes (6): Release 1.14.0 — rediseño de la pantalla principal, internal/config, Theme.ResolveDerived, config.saveKey, computeLayout (layout.go), styles.panel()

### Community 60 - "Packaged"
Cohesion: 0.47
Nodes (4): isPackagedPath(), Packaged(), TestIsPackagedPath(), TestPackagedChannelOverride()

### Community 61 - "README and Changelog"
Cohesion: 0.50
Nodes (5): CHANGELOG.md — Registro público de releases, README.md (español), Arquitectura: demonio + clientes sobre socket Unix, README.en.md (inglés), Créditos: herramientas y librerías coordinadas

### Community 62 - "Filosofía: coordinar herramientas externas, no reimplementarlas"
Cohesion: 0.25
Nodes (9): Invariante: ningún valor no finito llega a mpv, Integración con Matugen, revertida entera (1.10.2), Filosofía: coordinar herramientas externas, no reimplementarlas, daemon scan (daemon_scan.go), d.seek, library.FillDurations, Library.Scan, internal/player — wrapper de mpv (+1 more)

## Ambiguous Edges - Review These
- `libraryStats` → `Release 1.16.2 — atajos compartidos y tres P3`  [AMBIGUOUS]
  CLAUDE.md · relation: references

## Knowledge Gaps
- **31 isolated node(s):** `maly.bash script`, `maly`, `Daemon`, `TrackInfo`, `conMsg` (+26 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **8 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **What is the exact relationship between `libraryStats` and `Release 1.16.2 — atajos compartidos y tres P3`?**
  _Edge tagged AMBIGUOUS (relation: references) - confidence is low._
- **Why does `T()` connect `T` to `DBPath`, `Daemon`, `Image and Lyrics Decoding`, `Library Open and Benchmarks`, `Player`, `Load`, `buildTree`, `update/update.go`, `Model`, `testing.T`, `console.go`, `commands.go`, `doctor.go`, `github.com/charmbracelet/bubbletea.Cmd`, `github.com/charmbracelet/bubbletea.Model`, `Model`, `clip`, `View Layout Tests`, `Model`, `maly/get.go`, `runPlaylist`, `getPickModel`, `Now Playing Column`?**
  _High betweenness centrality (0.113) - this node is a cross-community bridge._
- **Why does `Tf()` connect `T` to `daemon_test.go`, `DBPath`, `Status`, `Daemon`, `Library Open and Benchmarks`, `Player`, `Load`, `update/update.go`, `Search`, `Model`, `testing.T`, `EnsureRuntimeDir`, `console.go`, `commands.go`, `doctor.go`, `Track`, `github.com/charmbracelet/bubbletea.Cmd`, `newConModel`, `github.com/charmbracelet/bubbletea.Model`, `Command`, `Model`, `clip`, `Model`, `runPlaylist`, `getPickModel`?**
  _High betweenness centrality (0.101) - this node is a cross-community bridge._
- **Why does `Model` connect `Model` to `Status`, `Daemon`, `Image and Lyrics Decoding`, `picker`, `logoModel`, `Load`, `buildTree`, `Search`, `Model`, `console.go`, `getPickModel`, `github.com/charmbracelet/bubbletea.Cmd`?**
  _High betweenness centrality (0.044) - this node is a cross-community bridge._
- **Are the 23 inferred relationships involving `Open()` (e.g. with `BenchmarkScan()` and `BenchmarkSearchLimit()`) actually correct?**
  _`Open()` has 23 INFERRED edges - model-reasoned connections that need verification._
- **What connects `maly.bash script`, `maly`, `Daemon` to the rest of the system?**
  _31 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `DBPath` be split into smaller, more focused modules?**
  _Cohesion score 0.05459770114942529 - nodes in this community are weakly interconnected._