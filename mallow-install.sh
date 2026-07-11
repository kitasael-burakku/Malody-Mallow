#!/bin/sh
# Mallow Install — el instalador de Malody Mallow (maly).
#
#   curl -fsSL https://raw.githubusercontent.com/kitasael-burakku/Malody-Mallow/main/mallow-install.sh | sh
#   ./mallow-install.sh [--system] [--uninstall] [--help]
#
# Compila maly desde main e instala el binario y las completions. Multi-distro:
# detecta el gestor de paquetes para las dependencias (git, mpv) y, si el Go de
# la distro no alcanza el mínimo de go.mod, baja el toolchain oficial de go.dev
# a ~/.cache/mallow — solo para compilar, sin tocar el sistema. POSIX sh puro.
set -eu

REPO_URL="https://github.com/kitasael-burakku/Malody-Mallow.git"

# ---- idioma: mensajes en es/en según el locale ----
# sys_lang encadena entorno → archivos del sistema → localectl y devuelve el
# primer locale no vacío (vacío = inglés). El entorno puede venir pelado
# (curl | sudo sh, chroots) aunque el sistema sí tenga idioma configurado.
sys_lang() {
	for v in "${LC_ALL:-}" "${LC_MESSAGES:-}" "${LANG:-}"; do
		if [ -n "$v" ]; then printf '%s' "$v"; return 0; fi
	done
	# /etc/locale.conf (systemd: Arch, Fedora) y /etc/default/locale (Debian):
	# línea LANG=… con o sin comillas; el ancla deja fuera las comentadas.
	for f in /etc/locale.conf /etc/default/locale; do
		[ -r "$f" ] || continue
		v=$(sed -n 's/^[[:space:]]*LANG=//p' "$f" 2>/dev/null | sed -n 1p | tr -d '\042\047')
		if [ -n "$v" ]; then printf '%s' "$v"; return 0; fi
	done
	# Último eslabón, solo con timeout a mano: localectl habla con dbus y en
	# contenedores/chroots puede quedarse colgado en vez de fallar.
	if command -v localectl >/dev/null 2>&1 && command -v timeout >/dev/null 2>&1; then
		v=$(timeout 2 localectl status 2>/dev/null | sed -n 's/.*System Locale: LANG=//p' | sed -n 1p)
		if [ -n "$v" ]; then printf '%s' "$v"; return 0; fi
	fi
	return 0
}
case "$(sys_lang)" in es*) ES=1 ;; *) ES=0 ;; esac
tr2() { if [ "$ES" -eq 1 ]; then printf '%s' "$1"; else printf '%s' "$2"; fi; }

# ---- colores solo si stdout es un terminal ----
if [ -t 1 ]; then
	CY=$(printf '\033[36m') BD=$(printf '\033[1m') RD=$(printf '\033[31m')
	YL=$(printf '\033[33m') GN=$(printf '\033[32m') NC=$(printf '\033[0m')
else
	CY='' BD='' RD='' YL='' GN='' NC=''
fi

msg()  { printf '%smallow ▸%s %s\n' "$CY" "$NC" "$(tr2 "$1" "$2")"; }
warn() { printf '%smallow ⚠%s %s\n' "$YL" "$NC" "$(tr2 "$1" "$2")"; }
ok()   { printf '%smallow ✓%s %s\n' "$GN" "$NC" "$(tr2 "$1" "$2")"; }
die()  { hb_stop; printf '%smallow ✗%s %s\n' "$RD" "$NC" "$(tr2 "$1" "$2")" >&2; exit 1; }

# ---- heartbeat: latido para pasos largos (clone, build) ----
# Cosmético: en hardware lento esos pasos tardan minutos sin salida y parece
# que el instalador se colgó. Si stdout es un terminal, reescribe una línea
# con el tiempo transcurrido cada 3 s; redirigido a un log no imprime nada
# intermedio. hb_stop es idempotente; die y el trap lo llaman para no dejar
# la línea a medias ni el proceso vivo.
HB_PID=''
hb_start() {
	msg "$1" "$2"
	[ -t 1 ] || return 0
	hb_base=$(tr2 "$1" "$2")
	(
		hb_s=0
		while :; do
			sleep 3
			hb_s=$((hb_s + 3))
			printf '\r\033[2K%smallow ▸%s %s %ss' "$CY" "$NC" "$hb_base" "$hb_s"
		done
	) &
	HB_PID=$!
}
hb_stop() {
	[ -n "$HB_PID" ] || return 0
	kill "$HB_PID" 2>/dev/null || :
	wait "$HB_PID" 2>/dev/null || :
	HB_PID=''
	if [ -t 1 ]; then printf '\r\033[2K'; fi
}

# rep repite el carácter $1 $2 veces (printf '%*s' con ancho dinámico no es
# POSIX); chars cuenta caracteres — no bytes — con wc -m, porque ${#var} y
# expr length no son consistentes entre shells con multibyte.
#
# wc -m solo cuenta multibyte bajo un locale UTF-8 *generado*; en una VM o
# contenedor recién instalado el entorno puede no tenerlo y wc caería a contar
# bytes (caja desalineada). C.UTF-8 existe sin generar en glibc moderno y en
# musl todo es UTF-8; se sondea, y si tampoco sirve se usa el del entorno.
if [ "$(printf '─' | LC_ALL=C.UTF-8 wc -m 2>/dev/null)" -eq 1 ] 2>/dev/null; then
	WCLOC=C.UTF-8
else
	WCLOC=${LC_ALL:-}
fi
rep() {
	out=''
	i=0
	while [ "$i" -lt "$2" ]; do out="$out$1"; i=$((i + 1)); done
	printf '%s' "$out"
}
chars() { printf '%s' "$1" | LC_ALL=$WCLOC wc -m; }

# banner dibuja la caja midiendo el contenido real: editar los textos no la
# desalinea. Margen fijo de 3 espacios a cada lado.
banner() {
	t1='♪  Malody Mallow · maly'
	t2=$(tr2 'Mallow Install — instalador' 'Mallow Install — installer')
	w1=$(( $(chars "$t1") ))
	w2=$(( $(chars "$t2") ))
	w=$w1
	if [ "$w2" -gt "$w" ]; then w=$w2; fi
	bar=$(rep '─' $((w + 6)))
	printf '%s\n' "${CY}  ╭${bar}╮${NC}"
	printf '%s\n' "${CY}  │${NC}   ${BD}${t1}${NC}$(rep ' ' $((w - w1)))   ${CY}│${NC}"
	printf '%s\n' "${CY}  │${NC}   ${t2}$(rep ' ' $((w - w2)))   ${CY}│${NC}"
	printf '%s\n' "${CY}  ╰${bar}╯${NC}"
}

# confirm pregunta por /dev/tty (stdin puede ser el propio script vía curl).
# Sin terminal no adivina: devuelve que no.
confirm() {
	printf '%smallow ?%s %s ' "$CY" "$NC" "$(tr2 "$1 [s/N]" "$2 [y/N]")"
	if IFS= read -r ans 2>/dev/null < /dev/tty; then
		:
	else
		printf '\n'
		return 1
	fi
	case "$ans" in [sSyY]*) return 0 ;; *) return 1 ;; esac
}

usage() {
	printf '%s\n' "$(tr2 'Mallow Install — instala Malody Mallow (maly) compilando desde main.

uso: mallow-install.sh [opciones]
  --system      instala en /usr/local para todos los usuarios (pide sudo)
  --uninstall   quita binario y completions (config y biblioteca quedan)
  --help        esta ayuda

Sin opciones instala en ~/.local/bin. Re-ejecutarlo actualiza.' 'Mallow Install — installs Malody Mallow (maly) building from main.

usage: mallow-install.sh [options]
  --system      install to /usr/local for all users (asks for sudo)
  --uninstall   remove binary and completions (config and library stay)
  --help        this help

With no options it installs to ~/.local/bin. Re-running updates.')"
}

# ---- argumentos ----
SYSTEM=0 UNINSTALL=0
for a in "$@"; do
	case "$a" in
	--system) SYSTEM=1 ;;
	--uninstall) UNINSTALL=1 ;;
	-h|--help) usage; exit 0 ;;
	*) usage >&2; die "opción desconocida: $a" "unknown option: $a" ;;
	esac
done

# ---- rutas de instalación ----
if [ "$SYSTEM" -eq 1 ]; then
	BIN=/usr/local/bin
	BASHC=/usr/local/share/bash-completion/completions
	FISHC=/usr/local/share/fish/vendor_completions.d
	ZSHC=/usr/local/share/zsh/site-functions
else
	BIN=$HOME/.local/bin
	DATA=${XDG_DATA_HOME:-$HOME/.local/share}
	BASHC=$DATA/bash-completion/completions
	FISHC=${XDG_CONFIG_HOME:-$HOME/.config}/fish/completions
	ZSHC=$DATA/zsh/site-functions
fi

# ¿$BIN ya existía antes de esta corrida? (para el aviso de fish: su PATH por
# defecto puede incluirlo, pero si lo creamos ahora conviene avisar igual).
# Se comprueba antes de instalar (install -D lo crea).
BIN_EXISTED=1
[ "$SYSTEM" -eq 0 ] && { [ -d "$BIN" ] || BIN_EXISTED=0; }

SUDO=''
if [ "$SYSTEM" -eq 1 ] && [ "$(id -u)" -ne 0 ]; then
	command -v sudo >/dev/null 2>&1 ||
		die 'para --system necesitas sudo (o corre como root)' 'for --system you need sudo (or run as root)'
	SUDO=sudo
fi

banner

# ---- desinstalar ----
if [ "$UNINSTALL" -eq 1 ]; then
	found=0
	for f in "$BIN/maly" "$BASHC/maly" "$FISHC/maly.fish" "$ZSHC/_maly"; do
		if [ -e "$f" ]; then
			$SUDO rm -f "$f"
			found=1
			msg "quitado: $f" "removed: $f"
		fi
	done
	[ "$found" -eq 1 ] || warn 'no encontré nada que quitar en esas rutas' 'nothing to remove at those paths'
	msg 'tu config, biblioteca y playlists quedan intactas' 'your config, library and playlists are untouched'
	exit 0
fi

TMP=$(mktemp -d "${TMPDIR:-/tmp}/mallow.XXXXXX")
trap 'st=$?; hb_stop; rm -rf "$TMP"; exit $st' EXIT INT TERM

# fetch baja una URL a un archivo con curl o wget, lo que haya.
fetch() {
	if command -v curl >/dev/null 2>&1; then curl -fsSLo "$2" "$1"
	elif command -v wget >/dev/null 2>&1; then wget -qO "$2" "$1"
	else die 'necesito curl o wget para descargar' 'curl or wget needed to download'
	fi
}

# ---- fuente: el checkout donde corre el script, o un clon temporal ----
SRC=''
script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" 2>/dev/null && pwd) || script_dir=''
if [ -n "$script_dir" ] && [ -f "$script_dir/go.mod" ] && [ -d "$script_dir/cmd/maly" ]; then
	SRC=$script_dir
fi

# ---- dependencias del sistema: git (si hay que clonar) y mpv ----
INSTALL_CMD=''
if command -v pacman >/dev/null 2>&1; then INSTALL_CMD='pacman -S --needed --noconfirm'
elif command -v apt-get >/dev/null 2>&1; then INSTALL_CMD='apt-get install -y'
elif command -v dnf >/dev/null 2>&1; then INSTALL_CMD='dnf install -y'
elif command -v zypper >/dev/null 2>&1; then INSTALL_CMD='zypper --non-interactive install'
elif command -v xbps-install >/dev/null 2>&1; then INSTALL_CMD='xbps-install -y'
fi

NEED=''
if [ -z "$SRC" ] && ! command -v git >/dev/null 2>&1; then NEED="$NEED git"; fi
command -v mpv >/dev/null 2>&1 || NEED="$NEED mpv"

if [ -n "$NEED" ]; then
	PKG_SUDO=''
	[ "$(id -u)" -ne 0 ] && PKG_SUDO='sudo '
	if [ -z "$INSTALL_CMD" ]; then
		die "no reconozco tu gestor de paquetes; instala a mano:$NEED" \
			"couldn't detect your package manager; install manually:$NEED"
	fi
	msg "falta:$NEED" "missing:$NEED"
	if confirm "¿instalar con \`$PKG_SUDO$INSTALL_CMD$NEED\`?" \
		"install with \`$PKG_SUDO$INSTALL_CMD$NEED\`?"; then
		[ -z "$PKG_SUDO" ] || command -v sudo >/dev/null 2>&1 ||
			die 'no hay sudo; instala las dependencias como root y reintenta' \
				'no sudo available; install the dependencies as root and retry'
		# shellcheck disable=SC2086
		$PKG_SUDO$INSTALL_CMD$NEED ||
			die 'falló la instalación de dependencias' 'dependency install failed'
	else
		die "instálalo tú y reintenta:  $PKG_SUDO$INSTALL_CMD$NEED" \
			"install it yourself and retry:  $PKG_SUDO$INSTALL_CMD$NEED"
	fi
fi

if [ -z "$SRC" ]; then
	hb_start 'clonando Malody Mallow…' 'cloning Malody Mallow…'
	git clone --quiet --depth=1 "$REPO_URL" "$TMP/src" ||
		{ hb_stop; die 'falló el clonado' 'clone failed'; }
	hb_stop
	SRC=$TMP/src
else
	msg "compilando desde el checkout: $SRC" "building from the checkout: $SRC"
fi

# ---- Go: el de la distro si alcanza el mínimo de go.mod; si no, el oficial ----
GOMIN=$(awk '$1 == "go" { print $2; exit }' "$SRC/go.mod")
[ -n "$GOMIN" ] || die 'no pude leer el mínimo de Go de go.mod' "couldn't read the Go minimum from go.mod"

# gover_ok comprueba que `$1 version` ≥ $2 (mayor.menor; ignora el parche).
gover_ok() {
	v=$("$1" version 2>/dev/null) || return 1
	v=${v#go version go}
	maj=${v%%.*}
	rest=${v#*.}
	min=${rest%%.*}
	case "$maj$min" in '' | *[!0-9]*) return 1 ;; esac
	wmaj=${2%%.*}
	wrest=${2#*.}
	wmin=${wrest%%.*}
	[ "$maj" -gt "$wmaj" ] || { [ "$maj" -eq "$wmaj" ] && [ "$min" -ge "$wmin" ]; }
}

GO=''
CACHE=${XDG_CACHE_HOME:-$HOME/.cache}/mallow
if command -v go >/dev/null 2>&1 && gover_ok go "$GOMIN"; then
	GO=go
elif gover_ok "$CACHE/go/bin/go" "$GOMIN"; then
	GO=$CACHE/go/bin/go
	msg "usando el Go cacheado en $CACHE/go" "using the cached Go at $CACHE/go"
else
	# Ni el go de la distro ni el cacheado alcanzan (en Debian/Fedora el paquete
	# además desactiva la descarga automática de toolchains, así que GOTOOLCHAIN
	# no es salida). Ofrecer el toolchain oficial, contenido en el cache.
	msg "maly necesita Go ≥ $GOMIN y tu sistema no lo tiene" \
		"maly needs Go ≥ $GOMIN and your system doesn't have it"
	if ! confirm "¿bajar el Go oficial de go.dev a $CACHE/go? (~80 MB, solo para compilar)" \
		"download official Go from go.dev to $CACHE/go? (~80 MB, only used to build)"; then
		die 'instala Go desde https://go.dev/dl/ y reintenta' \
			'install Go from https://go.dev/dl/ and retry'
	fi
	case "$(uname -m)" in
	x86_64) GOARCH=amd64 ;;
	aarch64 | arm64) GOARCH=arm64 ;;
	armv7l | armv6l) GOARCH=armv6l ;;
	i686 | i386) GOARCH=386 ;;
	riscv64) GOARCH=riscv64 ;;
	*) die "arquitectura sin binario oficial de Go: $(uname -m)" \
		"no official Go binary for this architecture: $(uname -m)" ;;
	esac
	fetch 'https://go.dev/VERSION?m=text' "$TMP/gover"
	IFS= read -r GOV < "$TMP/gover"
	case "$GOV" in go1.*) ;; *) die "respuesta rara de go.dev: $GOV" "odd reply from go.dev: $GOV" ;; esac
	msg "bajando $GOV linux/$GOARCH…" "downloading $GOV linux/$GOARCH…"
	fetch "https://go.dev/dl/$GOV.linux-$GOARCH.tar.gz" "$TMP/go.tgz"
	rm -rf "$CACHE/go"
	mkdir -p "$CACHE"
	tar -C "$CACHE" -xzf "$TMP/go.tgz"
	GO=$CACHE/go/bin/go
fi

# ---- compilar ----
hb_start 'compilando maly… (la primera vez baja dependencias de Go)' \
	'building maly… (first run downloads Go dependencies)'
(cd "$SRC" && "$GO" build -trimpath -ldflags '-s -w' -o "$TMP/maly" ./cmd/maly) ||
	{ hb_stop; die 'falló la compilación' 'build failed'; }
hb_stop

# ---- instalar binario y completions ----
$SUDO install -Dm755 "$TMP/maly" "$BIN/maly"
msg "instalado: $BIN/maly" "installed: $BIN/maly"

# inst_comp genera la completion con el binario recién compilado y la instala.
inst_comp() {
	"$TMP/maly" completions "$1" > "$TMP/comp.$1" 2>/dev/null || return 0
	$SUDO install -Dm644 "$TMP/comp.$1" "$2"
	msg "completions $1: $2" "completions $1: $2"
}
# --system instala las tres (como un paquete); en modo usuario, solo las de
# los shells presentes para no sembrar directorios de shells que no usas.
if [ "$SYSTEM" -eq 1 ] || command -v bash >/dev/null 2>&1; then inst_comp bash "$BASHC/maly"; fi
if [ "$SYSTEM" -eq 1 ] || command -v fish >/dev/null 2>&1; then inst_comp fish "$FISHC/maly.fish"; fi
if [ "$SYSTEM" -eq 1 ] || command -v zsh >/dev/null 2>&1; then
	inst_comp zsh "$ZSHC/_maly"
	if [ "$SYSTEM" -eq 0 ]; then
		warn "zsh: agrega a ~/.zshrc si no lo tienes:  fpath=($ZSHC \$fpath); autoload -U compinit && compinit" \
			"zsh: add to ~/.zshrc if you haven't:  fpath=($ZSHC \$fpath); autoload -U compinit && compinit"
	fi
fi

# ---- avisos finales ----
# sep imprime una línea en blanco antes del primer aviso del bloque, para
# separarlo del log de instalación; si no hay avisos, no deja hueco doble.
SEP=0
sep() { if [ "$SEP" -eq 0 ]; then printf '\n'; SEP=1; fi; }

if [ "$SYSTEM" -eq 0 ]; then
	# Consejo según el shell de login: en fish lo idiomático (y persistente) es
	# fish_add_path, no el export de POSIX.
	sh_name=${SHELL:-}; sh_name=${sh_name##*/}
	if [ "$sh_name" = fish ]; then
		on_path=0
		case ":$PATH:" in *":$BIN:"*) on_path=1 ;; esac
		if [ "$on_path" -eq 0 ]; then
			sep
			warn "$BIN no está en tu PATH; agrégalo:  fish_add_path $BIN" \
				"$BIN is not in your PATH; add it:  fish_add_path $BIN"
		elif [ "$BIN_EXISTED" -eq 0 ]; then
			sep
			warn "$BIN se creó en esta instalación; para que futuras sesiones lo vean, agrégalo:  fish_add_path $BIN" \
				"$BIN was created by this install; for future sessions to see it, add it:  fish_add_path $BIN"
		fi
	else
		# bash/zsh: no vale mirar el $PATH de esta corrida (puede venir heredado
		# de la sesión); lo que cuenta es que quede escrito en el rc que las
		# terminales nuevas sí leen. ~/.profile no sirve: solo lo leen los login
		# shells, y una terminal nueva del escritorio no lo es.
		case "$sh_name" in
		zsh) RC=${ZDOTDIR:-$HOME}/.zshrc ;;
		*) RC=$HOME/.bashrc ;;
		esac
		path_line="export PATH=\"$BIN:\$PATH\""
		if ! grep -qF "$BIN" "$RC" 2>/dev/null; then
			sep
			if confirm "$RC no menciona $BIN; ¿agregar  $path_line ?" \
				"$RC doesn't mention $BIN; add  $path_line ?"; then
				printf '\n# added by mallow-install.sh\n%s\n' "$path_line" >> "$RC"
				msg "escrito en $RC; con abrir una terminal nueva basta" \
					"written to $RC; opening a new terminal is enough"
			else
				warn "agrega  $path_line  a $RC (una terminal nueva basta); sin eso, cada terminal nueva necesitará  source ~/.profile  o ese export a mano" \
					"add  $path_line  to $RC (a new terminal is enough); without it, every new terminal will need  source ~/.profile  or that export by hand"
			fi
		fi
	fi
fi
if ! command -v pw-record >/dev/null 2>&1 && ! command -v parec >/dev/null 2>&1; then
	sep
	warn 'sin pw-record/parec el visualizador queda en modo animación (opcional: pipewire o pulseaudio-utils)' \
		'without pw-record/parec the visualizer stays in animation mode (optional: pipewire or pulseaudio-utils)'
fi
if ! command -v yt-dlp >/dev/null 2>&1 || ! command -v ffmpeg >/dev/null 2>&1; then
	sep
	warn 'sin yt-dlp y ffmpeg no funciona `maly get` (descargar música; opcional)' \
		'without yt-dlp and ffmpeg `maly get` will not work (music download; optional)'
fi

printf '\n'
ver=$("$TMP/maly" version | sed -n 1p)
ok "listo: ${BD}${ver}${NC}" "done: ${BD}${ver}${NC}"
msg 'primer paso:  maly scan   (indexa ~/Music; acepta otra ruta) · luego:  maly' \
	'first step:  maly scan   (indexes ~/Music; takes another path) · then:  maly'
