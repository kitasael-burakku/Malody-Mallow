# Releases y distribución

Fuente única: esta ficha. `CLAUDE.md` solo dice que una release es «bump +
tag + entrada de CHANGELOG + entrada de roadmap» y remite aquí.

## Qué toca un bump

1. `internal/version/version.go` — la const `Version` (sin la «v»).
2. `CHANGELOG.md` — un párrafo público, para quien solo quiere saber qué
   cambió. Sin categorías Added/Changed/Fixed: la mayoría de releases son una
   o dos piezas bien acotadas.
3. `docs/history/roadmap-*.md` — la entrada de ingeniería: qué se midió, qué
   se descartó, qué trampa costó un ciclo. Más la fila en
   `docs/history/README.md`.
4. Un **git tag anotado**. Los tags empiezan en `v1.0.0`.

**El badge de versión de ambos README NO se toca**: es dinámico
(`img.shields.io/github/v/tag/…?sort=semver`), lo actualiza el tag. (`CLAUDE.md`
afirmó durante mucho tiempo que se actualizaba a mano junto con la const;
era falso y quedó corregido al reorganizar la documentación.)

Cambios que **no** tocan el binario (instalador, PKGBUILD, unit de systemd,
solo documentación) **no** llevan bump: van al roadmap como «Sobre 1.x.y, sin
bump de versión…», que es la convención ya establecida.

Se han saltado números a propósito cuando la tanda era grande y guiada a
seguridad, para que el número lo diga: 1.1.5 (se saltaron 1.1.1–1.1.4) y
1.6.0 (se saltó 1.5.2). También se brincó la 0.7.0 antes del 1.0.0.

## Canales de distribución

Son **tres**, y conviven:

- **`mallow-install.sh`** (raíz del repo) — el instalador interactivo por
  pantallas. Por defecto compila **el último tag estable**; `--main` pide la
  rama de desarrollo y `--ref=<x>` tiene máxima prioridad e ignora ambos.
  Modo usuario a `~/.local/bin`, `--system` a `/usr/local`. **Nunca** instala
  en `/usr/bin`.
- **AUR** — paquete `maly` (`yay -S maly`), compila desde el tag estable más
  reciente. El PKGBUILD vive en un **repo aparte**
  (`~/Projects/PKGBUILDS/maly`, clon de `ssh://aur@aur.archlinux.org/maly.git`),
  NO dentro de este repo: el AUR publica empujando a ese remote directo, y
  mezclar esa historia exigiría un submódulo sin beneficio real. Fija
  `-ldflags "-X maly/internal/version.Channel=pacman"`, `CGO_ENABLED=0`, y
  empaqueta su propia unit de systemd (`ExecStart=/usr/bin/maly daemon`,
  distinta de la que genera el instalador). Instalada pero **sin `enable`**: un
  paquete no debe arrancar servicios solo.
- **`maly update`** — descarga el instalador con curl y corre `sh <tmp>
  --update`, pinneado al **tag anunciado** (`installerURL(ref)` +
  `--ref=`). Con un binario empaquetado (`version.Packaged()`) **no** toca el
  instalador: imprime `up.found_packaged` y remite al gestor.

El canal de paquete existe justamente porque el dueño vivió el problema: con
el paquete de AUR y el instalador manual conviviendo terminó con dos
binarios, dos units de systemd y una corriendo el binario que no está en el
PATH (ver la 1.11.1 en `docs/history/roadmap-1.8-1.11.md`).

## Unit de systemd `--user`

Vive en **cuatro** lugares que hay que tocar juntos: el generador dentro de
`mallow-install.sh`, el `maly.service` del PKGBUILD (con su propio `pkgrel`
bump y `updpkgsums`, porque tiene checksum propio en `source=()`), el ejemplo
documentado de **ambos** README, y la unit real del dueño.

Dos decisiones que no se revierten sin releer su motivo:

- **`default.target`, no `graphical-session.target`.** En Hyprland/sway nadie
  activa `graphical-session.target` solo, y maly no necesita nada gráfico
  (mpv corre con `--no-video`, MPRIS es D-Bus, el viz capta audio).
- **`RuntimeDirectory=maly` + `ConfigurationDirectory=maly`, no
  `ReadWritePaths=` para esas dos.** `ReadWritePaths=` exige que la ruta YA
  EXISTA al armar el namespace de montaje, que ocurre ANTES del proceso: en un
  boot limpio `/run/user/$UID` es tmpfs recién montado y el arranque moría con
  `status=226/NAMESPACE`. `$XDG_DATA_HOME` sí sigue yendo por
  `ReadWritePaths=` **con `-` delante** (ignorar si no existe), porque no tiene
  directiva dedicada. El hardening (`NoNewPrivileges`, `ProtectSystem=strict`,
  `PrivateTmp`, `RestrictAddressFamilies=AF_UNIX`, …) va **sin `ProtectHome`**,
  que bloquearía un `music_dir` en un disco externo.

## Firma de releases: descartada

Se estudió minisign vs GPG. La conclusión: solo aportaría de verdad ya con el
binario instalado verificando en `maly update` con una clave embebida en
compilación; firmar el bootstrap `curl | sh` no cierra nada, porque la clave
viajaría del mismo GitHub que ya se está confiando. Exige un proceso de
release nuevo y queda como iniciativa aparte.

## Trampas del instalador

- Sondea `/dev/tty` **en subshell**: `:` es un special builtin y POSIX manda
  que su redirección fallida termine el shell entero — sin subshell, el modo
  no interactivo moría mudo.
- `raw_off` va en el trap (un Ctrl-C en pleno modo crudo dejaría el terminal
  sin eco ni cursor); el trap se arma ANTES de las pantallas con `TMP=''`.
- `latest_tag()` compara versiones con `awk`, no en sh puro: ahí «0» es un
  valor legítimo (`v1.0.0`) y el pelado de ceros líderes para `$(( ))` se
  rompería.
- Clonar un tag anotado con `--depth=1` hace que git imprima un aviso
  inofensivo («… is not a commit!»); el stderr del clonado va a un temporal y
  solo se muestra si el clonado falla de verdad.
- Probarlo bajo tmux con HOME alterno: pasar `GOMODCACHE`/`GOCACHE` reales al
  `go build` o deja un mod-cache de solo lectura en el sandbox (`chmod -R u+w`
  antes de borrar).
