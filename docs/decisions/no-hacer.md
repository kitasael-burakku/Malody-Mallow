# Decisiones cerradas — lo que NO se hace, y por qué

Fuente única: esta ficha. `CLAUDE.md` lleva la lista corta (la regla sin el
desarrollo) y remite aquí.

Todo lo de abajo se **evaluó y se descartó**, casi siempre con la medición o
el experimento delante. No son huecos por hacer: son decisiones. Si una idea
nueva cae en una de estas casillas, lo que hay que releer es la razón, no
reabrir el debate desde cero.

Cada entrada dice dónde está el desarrollo completo.

## Frontera con lo online

**maly nunca abre una conexión a internet por su cuenta.** Es el invariante
que encabeza `CLAUDE.md`, y de él cuelgan tres descartes concretos:

- **Miniaturas en el buscador de descargas** (1.16.1). Sería la primera
  petición de red propia de la historia del proyecto. En cuanto maly baje UNA
  imagen deja de ser un reproductor que coordina herramientas y pasa a ser un
  cliente de YouTube, con su gestión de timeouts, reintentos, caché, TLS y
  user-agent, y una superficie de red propia que auditar. Descartadas, **no
  aplazadas**.
- **La pantalla completa del buscador** (1.16.1), que caía con ellas: solo
  tenía sentido para dar sitio a una imagen.
- **HTTP propio en `internal/update`** — pudiendo usar la API de GitHub, usa
  `git ls-remote`.

Cualquier idea que necesite traerse un recurso de la red se resuelve
pidiéndoselo a yt-dlp o no se hace.
→ `docs/history/roadmap-1.14-1.16.md` (1.16.1)

## Parsear la salida de yt-dlp

**No se parsea la salida HUMANA** (progreso, avisos, errores): es frágil y
cambia sin aviso. El proyecto la esquivó tres veces a propósito (diff de
directorio, `%(playlist_index)02d` en el nombre, `%(playlist_title)s` como
subdirectorio).

La ÚNICA excepción es `--dump-json` (`getter.Search`), que es su interfaz de
MÁQUINA — lo mismo que se hace con ffprobe. Se decodifican 6 campos de los
~50: toda la superficie de acoplamiento.

Y **no** se usa `--print` con separador, que parece más barato y está roto de
entrada: un resultado real trae el separador dentro del propio título.
→ `docs/history/roadmap-1.14-1.16.md` (1.15.0)

## El demonio

- **No se saca `search` ni `playlist_play` de `d.mu`.** Se MIDIÓ y la
  medición refutó la hipótesis del informe: con 40.000 pistas un `search` de
  la biblioteca entera retiene `d.mu` 96 ms, y con un scan reescribiendo esas
  mismas filas a la vez el peor caso fue 112 ms — la contención por la única
  conexión SQLite suma 16 ms, no segundos. Además la consulta vacía no es
  alcanzable por el demonio (verificado otra vez en A-20: el único cliente que
  manda `search` por IPC es la consola, y sus argumentos salen de
  `strings.Fields`).
  → `docs/history/candidatos-post-1.0.md`
- **La descarga NO pertenece al demonio.** Meterla ahí exigiría inventar
  trabajos asíncronos, progreso por push, cancelación y errores parciales — un
  subsistema entero en un demonio cuyo `dispatch` es a propósito un switch
  plano bajo un solo mutex. Hoy `getter` arma el comando y el CLIENTE lo corre.
  → `docs/history/roadmap-1.14-1.16.md` (1.15.0)
- **`dispatch` no se convierte en tabla de funciones.** Sigue siendo un switch
  plano de ~20 comandos: dividirlo complicaría las tres excepciones de «antes
  de `d.mu`», que hoy son ifs explícitos y auditables.
  → `docs/history/roadmap-1.8-1.11.md` (1.10.0)
- **Sin auth ni `SO_PEERCRED` en el socket.** El dir 0700 + el chequeo de
  dueño ya es la frontera real; peer creds entre procesos del mismo UID serían
  teatro. El `chmod 0600` del socket es defensa en profundidad, no el arreglo
  que cierra el vector.
  → `docs/history/roadmap-1.8-1.11.md` (1.11.0)
- **Sin rollback de las mutaciones de `dispatch` ante fallo del player**, y sin
  tope de conexiones concurrentes ni cota de `req.Paths` más allá de los ya
  puestos: el atacante de esos vectores es del mismo UID, o sea que ya tiene la
  cuenta.
  → `docs/history/candidatos-post-1.0.md`
- **No se usa `Pdeathsig`.** Go documenta que la señal se envía al morir el
  HILO creador, no el proceso (go.dev/issue/27505): sin una goroutine
  permanente con `LockOSThread` mataría mpv a mitad de canción. Para lo que
  ninguna señal cubre (SIGKILL, OOM, pánico), `player.Start` reapea por IPC al
  mpv que siguiera en el socket.

## Dependencias

- **`godbus/prop` no vuelve.** Tiene una data race con propiedades mapa y
  nunca borra claves; `internal/mpris/props.go` es la implementación propia que
  lo reemplaza. `TestPropsConcurrent` es justo el test que la habría cazado.
- **`charmbracelet/x/ansi.Strip` se rechazó** para `internal/safetext`: no se
  delega una propiedad de SEGURIDAD en una librería externa.
- **`gonum` se queda.** Se investigó sacarlo y se midió: el módulo pesa 19 MB
  en disco, pero el linker mete ~65 KB en el binario final, y el module graph
  pruning de Go descarta sus dependencias pesadas sin tocarlas. La nota «19 MB»
  de la auditoría medía el eje equivocado; reemplazar una FFT real auditada por
  una propia para ahorrar 65 KB es mal cambio.
  → `docs/history/roadmap-1.8-1.11.md` (1.11.1)
- **SQLite sigue siendo modernc (sin CGo)**, coherente con el PKGBUILD.

## CLI y consola

- **Sin parser de flags.** `-v`/`-l`/`-h` son COMANDOS. Todo lo que en otro
  proyecto sería un flag es un subcomando (`get playlist`, `get pick`).
- **Sin salida JSON.** El scripting de reproducción ya lo cubre MPRIS, y el
  API de máquina de `doctor` es su código de salida.
  → `docs/history/roadmap-1.0-1.7.md` (1.7.0)
- **Sin `maly report`** (info + doctor con marco para pegar): la salida de
  `info` ya es pegable porque lipgloss se apaga fuera del tty, y un comando
  cuyo propósito es producir un volcado público roza el motivo por el que
  config/sesión/DB son 0600.
- **Sin `status --verbose`**: `-v` ya es alias de `version`, y esos datos son
  estado de la instalación, no de la reproducción.
- **Sin escape para la guarda de scan vacío** (ni `scan force` ni clave de
  config). Un subcomando costaría tabla de comandos + completions + ayuda +
  espejo de la consola + i18n; una clave se pone una vez y se olvida, que es
  lo contrario de una confirmación puntual. Quien quiera vaciar la biblioteca
  borra `library.db`, y el mensaje se lo dice.
  → `docs/history/roadmap-1.14-1.16.md` (1.16.4, A-01)
- **Sin `playlist move`**: asimetría consciente con la cola (C25).
- **La consola ctrl+p nunca fue espejo estricto de la CLI.** Tiene `cls`,
  `viz`, `logo` y `quit`, y le faltan `select` y `completions`. Lo que sí es
  obligatorio es la **paridad de los comandos que existen en ambos lados**, que
  la cuida `ConsoleCommands` + sus dos tests — el gap que dejó pasar A-04.
- **`ctrl+g` y las demás PANTALLAS no entran en `ConsoleCommands`**: son
  pantallas, no comandos, igual que ctrl+o y ctrl+t.

## TUI

- **El ratón sigue descartado.**
- **La marquesina para las letras largas se descartó**: leer texto que se
  desplaza es peor que leerlo en dos filas, y contradice la autocancelación de
  animaciones de la 1.7.2. Ensanchar la columna tampoco (a 40 celdas seguiría
  cortando el 23 % de las líneas reales del dueño, robándoselo a la cola).
- **El «carrusel» de letras se implementó, se probó en vivo y se revirtió** sin
  llegar a commitear: el dueño prefirió el enfoque simple.
  → `docs/history/roadmap-1.12-1.13.md` (1.13.1)
- **El reparto de la pantalla no vuelve dentro de `View()`.** `computeLayout`
  es PURA e innegociable: la aritmética de tres columnas + viz + banner es
  justo la clase de código que se rompe una celda a la vez y solo en ciertos
  tamaños.
- **La barra de reproducción no vuelve a la columna.** En 26 celdas el
  progreso no se lee (decisión del dueño, tras probarlo).
- **No se colorean trozos de un `pickerItem.label`**: es una sola cadena que el
  picker pinta con un solo estilo. Por eso `channel_is_verified` se descartó.

## Temas y colores

- **Matugen no vuelve.** `maly theme sync` (1.9.0) y la recarga por `SIGUSR1`
  (1.10.1) se revirtieron ENTERAS en la 1.10.2, código y sistema real, tras
  probarlas dadas de alta de verdad: «son demasiadas cosas orientadas a un
  desktop cuando maly debería sentirse global o para uno mismo». Depender de la
  paleta del wallpaper, con plantilla + `post_hook` + señal + archivo estático
  + script de restauración, es theming de escritorio viviendo dentro de un
  reproductor.
  → `docs/history/roadmap-1.8-1.11.md` (1.10.2)
- **Sin `#rrggbbaa`** en ningún color: con `transparent = true` no hay fondo
  conocido contra el que componer.
- **El arte ASCII del banner no es clave TOML** (`logo.txt` aparte): un string
  multilínea rompería el parser por líneas de `saveKey` y el escapado del `\`
  de figlet.

## Distribución

- **Sin firma de releases** (minisign/GPG estudiados): la clave viajaría del
  mismo GitHub que ya se confía. → `docs/development/releases.md`
- **El PKGBUILD no vive en este repo.** → `docs/development/releases.md`
- **`--uninstall` no toca `/usr/bin`**: es territorio del gestor de paquetes.
  Lo señala y remite al gestor — informa, no borra.

## Documentación y proceso

- **El CHANGELOG no lleva categorías Added/Changed/Fixed**: la mayoría de
  releases son una o dos piezas acotadas.
- **Los tests no honran `-short`** y CI no lo pasa: sería un no-op.
- **`internal/daemon` no está en el job `race` de CI.**
  → `docs/development/verificacion.md`
