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
case "${LC_ALL:-${LC_MESSAGES:-${LANG:-}}}" in es*) ES=1 ;; *) ES=0 ;; esac
tr2() { if [ "$ES" -eq 1 ]; then printf '%s' "$1"; else printf '%s' "$2"; fi; }

# ---- colores solo si stdout es un terminal ----
if [ -t 1 ]; then
	CY=$(printf '\033[36m') BD=$(printf '\033[1m') RD=$(printf '\033[31m')
	YL=$(printf '\033[33m') NC=$(printf '\033[0m')
else
	CY='' BD='' RD='' YL='' NC=''
fi

msg()  { printf '%smallow ▸%s %s\n' "$CY" "$NC" "$(tr2 "$1" "$2")"; }
warn() { printf '%smallow ⚠%s %s\n' "$YL" "$NC" "$(tr2 "$1" "$2")"; }
die()  { printf '%smallow ✗%s %s\n' "$RD" "$NC" "$(tr2 "$1" "$2")" >&2; exit 1; }

banner() {
	printf '%s\n' "${CY}  ╭─────────────────────────────────╮${NC}"
	printf '%s\n' "${CY}  │${NC}   ${BD}♪  Malody Mallow · maly${NC}       ${CY}│${NC}"
	printf '%s\n' "${CY}  │${NC}   $(tr2 'Mallow Install — instalador ' 'Mallow Install — installer  ')  ${CY}│${NC}"
	printf '%s\n' "${CY}  ╰─────────────────────────────────╯${NC}"
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
trap 'st=$?; rm -rf "$TMP"; exit $st' EXIT INT TERM

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
	msg 'clonando Malody Mallow…' 'cloning Malody Mallow…'
	git clone --quiet --depth=1 "$REPO_URL" "$TMP/src"
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
msg 'compilando maly… (la primera vez baja dependencias de Go)' \
	'building maly… (first run downloads Go dependencies)'
(cd "$SRC" && "$GO" build -trimpath -ldflags '-s -w' -o "$TMP/maly" ./cmd/maly) ||
	die 'falló la compilación' 'build failed'

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
case ":$PATH:" in
*":$BIN:"*) ;;
*) warn "$BIN no está en tu PATH: abre una sesión nueva o agrégalo a tu shell" \
	"$BIN is not in your PATH: open a new session or add it to your shell" ;;
esac
if ! command -v pw-record >/dev/null 2>&1 && ! command -v parec >/dev/null 2>&1; then
	warn 'sin pw-record/parec el visualizador queda en modo animación (opcional: pipewire o pulseaudio-utils)' \
		'without pw-record/parec the visualizer stays in animation mode (optional: pipewire or pulseaudio-utils)'
fi

ver=$("$TMP/maly" version | sed -n 1p)
msg "listo: ${BD}${ver}${NC}" "done: ${BD}${ver}${NC}"
msg 'primer paso:  maly scan   (indexa ~/Music; acepta otra ruta) · luego:  maly' \
	'first step:  maly scan   (indexes ~/Music; takes another path) · then:  maly'
