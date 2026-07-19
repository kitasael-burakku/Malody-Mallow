#!/bin/sh
# Mallow Install — el instalador de Malody Mallow (maly).
#
#   curl -fsSL https://raw.githubusercontent.com/kitasael-burakku/Malody-Mallow/main/mallow-install.sh | sh
#   ./mallow-install.sh [--install | --update | --uninstall] [--system] [--help]
#
# Interactivo por pantallas cuando hay terminal: wizard con el banner MALODY
# en degradado, menús y checklist navegables con ↑↓/jk + espacio (fallback
# numérico si el tty no da modo crudo) y spinner en los pasos largos. Flujo:
# acción (instalar/actualizar/desinstalar) → ámbito (usuario/sistema) →
# dependencias (checklist con mpv y git marcados; yt-dlp+ffmpeg y el
# visualizador son opcionales). Los flags pre-contestan su pantalla; sin
# terminal corre entero con los defaults, sin dibujar nada.
# Compila maly desde main e instala el binario y las completions. Multi-distro:
# detecta el gestor de paquetes y, si el Go de la distro no alcanza el mínimo
# de go.mod, baja el toolchain oficial de go.dev a ~/.cache/mallow — solo para
# compilar, sin tocar el sistema. POSIX sh puro.
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
# G0..G5: paleta Kitasan de maly interpolada (#7ab8b8 → #8098a8 → #b85c50),
# un color por línea del banner MALODY. Truecolor: los terminales sin él
# aproximan o ignoran, y sin terminal no se emite nada.
if [ -t 1 ]; then
	CY=$(printf '\033[36m') BD=$(printf '\033[1m') RD=$(printf '\033[31m')
	YL=$(printf '\033[33m') GN=$(printf '\033[32m') NC=$(printf '\033[0m')
	DM=$(printf '\033[2m')
	G0=$(printf '\033[38;2;122;184;184m') G1=$(printf '\033[38;2;124;171;178m')
	G2=$(printf '\033[38;2;127;158;171m') G3=$(printf '\033[38;2;139;140;150m')
	G4=$(printf '\033[38;2;162;116;115m') G5=$(printf '\033[38;2;184;92;80m')
else
	CY='' BD='' RD='' YL='' GN='' NC='' DM=''
	G0='' G1='' G2='' G3='' G4='' G5=''
fi

msg()  { printf '%smallow ▸%s %s\n' "$CY" "$NC" "$(tr2 "$1" "$2")"; }
warn() { printf '%smallow ⚠%s %s\n' "$YL" "$NC" "$(tr2 "$1" "$2")"; }
ok()   { printf '%smallow ✓%s %s\n' "$GN" "$NC" "$(tr2 "$1" "$2")"; }
die()  { hb_stop; printf '%smallow ✗%s %s\n' "$RD" "$NC" "$(tr2 "$1" "$2")" >&2; exit 1; }

# ---- spinner: indicador vivo para pasos largos (clone, build) ----
# En hardware lento esos pasos tardan minutos sin salida y parece que el
# instalador se colgó. Con terminal: spinner braille girando con el tiempo
# transcurrido, reescribiendo una línea; redirigido a un log cae al msg
# estático de siempre (nada intermedio). hb_stop limpia en silencio (para
# die y el trap: no dejar línea a medias ni proceso vivo); hb_done además
# deja el paso rematado con ✓ en el scrollback. El sleep fraccional no es
# POSIX estricto (GNU/busybox sí lo traen): se sondea una vez y sin él el
# spinner gira a 1 fps, que igual late.
HB_PID=''
hb_base=''
if sleep 0.1 2>/dev/null; then HBTICK=0.1 HBDIV=10; else HBTICK=1 HBDIV=1; fi
hb_start() {
	hb_base=$(tr2 "$1" "$2")
	if [ ! -t 1 ]; then
		msg "$1" "$2"
		return 0
	fi
	(
		hb_i=0
		while :; do
			for hb_f in ⣾ ⣽ ⣻ ⢿ ⡿ ⣟ ⣯ ⣷; do
				printf '\r\033[2K%smallow %s%s %s %s%ss%s' \
					"$CY" "$hb_f" "$NC" "$hb_base" "$DM" "$((hb_i / HBDIV))" "$NC"
				sleep "$HBTICK"
				hb_i=$((hb_i + 1))
			done
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
# hb_done remata un paso exitoso: limpia el spinner y deja constancia.
hb_done() {
	hb_stop
	[ ! -t 1 ] || printf '%smallow ✓%s %s\n' "$GN" "$NC" "$hb_base"
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

# term_cols: columnas del terminal (0 = desconocido); stty puede faltar.
term_cols() {
	set -- $(stty size < /dev/tty 2>/dev/null || :)
	if [ $# -eq 2 ]; then printf '%s' "$2"; else printf 0; fi
}

# banner: el MALODY de la TUI (figlet bloody) con el degradado Kitasan, una
# parada de color por línea. En terminales angostos (o sin stty) cae a la
# caja sobria de siempre, que se dibuja midiendo el contenido real.
banner() {
	cols=0
	if [ -t 1 ]; then cols=$(term_cols); fi
	if [ "$cols" -ge 58 ]; then
		printf '%s\n' \
			"${G0} ███▄ ▄███▓ ▄▄▄       ██▓     ▒█████  ▓█████▄▓██   ██▓${NC}" \
			"${G1}▓██▒▀█▀ ██▒▒████▄    ▓██▒    ▒██▒  ██▒▒██▀ ██▌▒██  ██▒${NC}" \
			"${G2}▓██    ▓██░▒██  ▀█▄  ▒██░    ▒██░  ██▒░██   █▌ ▒██ ██░${NC}" \
			"${G3}▒██    ▒██ ░██▄▄▄▄██ ▒██░    ▒██   ██░░▓█▄   ▌ ░ ▐██▓░${NC}" \
			"${G4}▒██▒   ░██▒ ▓█   ▓██▒░██████▒░ ████▓▒░░▒████▓  ░ ██▒▓░${NC}" \
			"${G5}░ ▒░   ░  ░ ▒▒   ▓▒█░░ ▒░▓  ░░ ▒░▒░▒░  ▒▒▓  ▒   ██▒▒▒${NC}"
		printf '\n      %s♪ %s%s\n' "$BD" "$(tr2 'Mallow Install — instalador de Malody Mallow' 'Mallow Install — the Malody Mallow installer')" "$NC"
		return 0
	fi
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

# ---- primitivas interactivas ----
# Todas leen de /dev/tty: con `curl | sh` stdin es el propio script. Sin
# terminal (TTY=0) cada pregunta devuelve su default sin imprimir nada: el
# script entero corre no interactivo con los defaults, como siempre.
# El sondeo va en subshell a propósito: `:` es un special builtin y POSIX
# manda que una redirección fallida en uno TERMINE el shell — sin subshell,
# el script entero moriría mudo justo aquí cuando no hay terminal.
if ( : < /dev/tty; ) 2>/dev/null; then TTY=1; else TTY=0; fi

# ask imprime un prompt y deja la línea leída en $REPLY (vacía → default $1).
ask() {
	REPLY=$1
	[ "$TTY" -eq 1 ] || return 0
	printf '%smallow ?%s %s ' "$CY" "$NC" "$(tr2 "$2" "$3")"
	IFS= read -r REPLY < /dev/tty || { printf '\n'; REPLY=$1; }
	[ -n "$REPLY" ] || REPLY=$1
}

# ---- teclado crudo (flechas/jk) ----
# RAWOK: hay stty y el tty acepta modo crudo; sin él, menús y checklist caen
# al modo numérico de siempre (número + Enter), que funciona en cualquier
# parte. raw_off es idempotente y lo llama también el trap: un Ctrl-C a
# mitad de menú no puede dejar el terminal mudo ni sin cursor.
RAWOK=0
if [ "$TTY" -eq 1 ] && command -v stty >/dev/null 2>&1 &&
	stty -g < /dev/tty > /dev/null 2>&1; then
	RAWOK=1
fi
SAVED_STTY=''
ESCCH=$(printf '\033')
raw_on() {
	SAVED_STTY=$(stty -g < /dev/tty)
	stty -icanon -echo min 1 time 0 < /dev/tty
	printf '\033[?25l' > /dev/tty
}
raw_off() {
	[ -n "$SAVED_STTY" ] || return 0
	stty "$SAVED_STTY" < /dev/tty 2>/dev/null || :
	SAVED_STTY=''
	printf '\033[?25h' > /dev/tty
}

# key_read imprime un token por tecla: up down space enter digit-N esc other.
# Las flechas llegan como ESC [ A/B: tras un ESC se leen hasta 2 bytes más
# con timeout corto (min 0 time 2) para distinguirlas de un ESC suelto.
key_read() {
	k=$(dd bs=1 count=1 2>/dev/null < /dev/tty)
	case $k in
	k) printf up; return 0 ;;
	j) printf down; return 0 ;;
	' ') printf space; return 0 ;;
	'') printf enter; return 0 ;; # \n y \r se pierden en $(): Enter
	[0-9]) printf 'digit-%s' "$k"; return 0 ;;
	"$ESCCH") ;;
	*) printf other; return 0 ;;
	esac
	stty min 0 time 2 < /dev/tty
	seq=$(dd bs=2 count=1 2>/dev/null < /dev/tty)
	stty min 1 time 0 < /dev/tty
	case $seq in
	'[A') printf up ;;
	'[B') printf down ;;
	*) printf esc ;;
	esac
}

# confirm pregunta sí/no; sin terminal no adivina: devuelve que no. Con modo
# crudo basta UNA tecla (s/y = sí; cualquier otra = no) y se deja el eco de
# la respuesta; sin él, la línea + Enter de siempre.
confirm() {
	[ "$TTY" -eq 1 ] || return 1
	printf '%smallow ?%s %s ' "$CY" "$NC" "$(tr2 "$1 [s/N]" "$2 [y/N]")"
	if [ "$RAWOK" -eq 1 ]; then
		raw_on
		ans=$(dd bs=1 count=1 2>/dev/null < /dev/tty)
		raw_off
	elif ! IFS= read -r ans < /dev/tty; then
		printf '\n'
		return 1
	fi
	case "$ans" in
	[sSyY]*)
		[ "$RAWOK" -eq 0 ] || printf '%s\n' "$(tr2 'sí' 'yes')"
		return 0 ;;
	*)
		[ "$RAWOK" -eq 0 ] || printf '%s\n' "no"
		return 1 ;;
	esac
}

# menu_num: el menú numérico de siempre (fallback sin modo crudo).
menu_num() {
	m_def=$1 m_n=$2
	shift 2
	m_i=1
	while [ "$m_i" -le "$m_n" ]; do
		m_mark=' '
		[ "$m_i" -eq "$m_def" ] && m_mark='*'
		printf '  %s%s%s %d) %s\n' "$CY" "$m_mark" "$NC" "$m_i" "$(tr2 "$1" "$2")"
		shift 2
		m_i=$((m_i + 1))
	done
	while :; do
		ask "$m_def" "elige [1-$m_n, Enter = $m_def]:" "choose [1-$m_n, Enter = $m_def]:"
		case $REPLY in
		[1-9]) if [ "$REPLY" -le "$m_n" ]; then return 0; fi ;;
		esac
	done
}

# menu deja en $REPLY la opción elegida (pares es/en tras el conteo). Con
# modo crudo: cursor ❯ que se mueve con ↑↓/jk, Enter elige, un dígito salta
# directo; el bloque se redibuja en el sitio (cursor arriba + repintado).
#   menu <default> <n> es1 en1 [es2 en2 …]
menu() {
	m_def=$1 m_n=$2
	shift 2
	if [ "$TTY" -eq 0 ]; then
		REPLY=$m_def
		return 0
	fi
	if [ "$RAWOK" -eq 0 ]; then
		menu_num "$m_def" "$m_n" "$@"
		return 0
	fi
	m_i=1
	while [ "$m_i" -le "$m_n" ]; do
		eval "M_$m_i=\$(tr2 \"\$1\" \"\$2\")"
		shift 2
		m_i=$((m_i + 1))
	done
	m_cur=$m_def
	m_drawn=0
	raw_on
	while :; do
		[ "$m_drawn" -eq 0 ] || printf '\033[%dA' "$((m_n + 1))" > /dev/tty
		m_i=1
		while [ "$m_i" -le "$m_n" ]; do
			eval "m_lbl=\$M_$m_i"
			if [ "$m_i" -eq "$m_cur" ]; then
				printf '\r\033[2K  %s❯ %s%s\n' "$CY$BD" "$m_lbl" "$NC" > /dev/tty
			else
				printf '\r\033[2K    %s%s%s\n' "$DM" "$m_lbl" "$NC" > /dev/tty
			fi
			m_i=$((m_i + 1))
		done
		printf '\r\033[2K  %s%s%s\n' "$DM" "$(tr2 '↑↓ mover · Enter elegir' '↑↓ move · Enter select')" "$NC" > /dev/tty
		m_drawn=1
		case $(key_read) in
		up) [ "$m_cur" -le 1 ] || m_cur=$((m_cur - 1)) ;;
		down) [ "$m_cur" -ge "$m_n" ] || m_cur=$((m_cur + 1)) ;;
		enter) break ;;
		digit-1) m_cur=1; break ;;
		digit-2) [ "$m_n" -ge 2 ] && { m_cur=2; break; } ;;
		digit-3) [ "$m_n" -ge 3 ] && { m_cur=3; break; } ;;
		digit-4) [ "$m_n" -ge 4 ] && { m_cur=4; break; } ;;
		esac
	done
	raw_off
	REPLY=$m_cur
}

# ---- wizard: pantallas limpias con banner fijo ----
# Cada pantalla de preguntas limpia el terminal y redibuja banner + barra de
# paso; la fase de ejecución (paquetes, clone, build) vuelve al log corrido
# para no perder la salida de pacman/go en el scrollback. Sin terminal no se
# dibuja nada, como siempre. STEP numera las pantallas que sí aparecen (las
# pre-contestadas por flag no cuentan).
STEP=0
scr() {
	[ "$TTY" -eq 1 ] || return 0
	STEP=$((STEP + 1))
	printf '\033[2J\033[H'
	banner
	printf '\n  %s── %d · %s ──%s\n\n' "$CY" "$STEP" "$(tr2 "$1" "$2")" "$NC"
}
# run_phase abre la fase de ejecución: pantalla limpia una vez y de ahí en
# adelante log normal.
run_phase() {
	[ "$TTY" -eq 1 ] || return 0
	printf '\033[2J\033[H'
	banner
	printf '\n'
}

# El trap va aquí, ANTES de la primera pantalla: raw_off debe correr aunque
# el usuario corte con Ctrl-C a mitad de un menú crudo (si no, el terminal
# queda sin eco ni cursor). TMP se crea después; vacío, rm -rf no toca nada.
TMP=''
cleanup() {
	hb_stop
	raw_off 2>/dev/null || :
	[ -z "$TMP" ] || rm -rf "$TMP"
}
trap 'st=$?; cleanup; exit $st' EXIT INT TERM

usage() {
	printf '%s\n' "$(tr2 'Mallow Install — instala Malody Mallow (maly) compilando desde main.

uso: mallow-install.sh [opciones]
  --install     instala (o reinstala) sin pasar por el menú
  --update      recompila y reinstala sobre una instalación existente
  --uninstall   quita binario y completions (pregunta por config/biblioteca)
  --system      instala en /usr/local para todos los usuarios (pide sudo)
  --ref=<tag>   compila ese tag/rama en vez de main (lo usa maly update)
  --help        esta ayuda

Sin opciones abre el flujo interactivo (o instala con los defaults si no
hay terminal). yt-dlp+ffmpeg (para `maly get`) y el visualizador son
opcionales y se eligen en la pantalla de dependencias.' 'Mallow Install — installs Malody Mallow (maly) building from main.

usage: mallow-install.sh [options]
  --install     install (or reinstall) skipping the menu
  --update      rebuild and reinstall over an existing install
  --uninstall   remove binary and completions (asks about config/library)
  --system      install to /usr/local for all users (asks for sudo)
  --ref=<tag>   build that tag/branch instead of main (used by maly update)
  --help        this help

With no options it opens the interactive flow (or installs with the
defaults when there is no terminal). yt-dlp+ffmpeg (for `maly get`) and
the visualizer are optional and picked on the dependencies screen.')"
}

# ---- argumentos ----
# ACTION vacío = decidir en el menú (o el default sin terminal); un flag de
# acción se salta esa pantalla. SYSTEM=-1 = preguntar el ámbito. REF vacío =
# compilar main; `maly update` pasa --ref=<tag> para instalar exactamente el
# release que anunció (main puede ir adelante del último tag).
ACTION='' SYSTEM=-1 REF=''
for a in "$@"; do
	case "$a" in
	--install | --update | --uninstall)
		[ -z "$ACTION" ] || die 'elige una sola acción (--install | --update | --uninstall)' \
			'pick a single action (--install | --update | --uninstall)'
		ACTION=${a#--} ;;
	--system) SYSTEM=1 ;;
	--ref=*) REF=${a#--ref=} ;;
	-h | --help) usage; exit 0 ;;
	*) usage >&2; die "opción desconocida: $a" "unknown option: $a" ;;
	esac
done

# ---- instalaciones existentes (deciden defaults del menú y del ámbito) ----
USR_BIN=$HOME/.local/bin/maly
SYS_BIN=/usr/local/bin/maly
USR_INST=0 SYS_INST=0
[ -x "$USR_BIN" ] && USR_INST=1
[ -x "$SYS_BIN" ] && SYS_INST=1

banner

# ---- pantalla 1: acción ----
if [ -z "$ACTION" ]; then
	a_def=1
	[ "$USR_INST" -eq 1 ] || [ "$SYS_INST" -eq 1 ] && a_def=2
	if [ "$TTY" -eq 1 ]; then
		scr 'acción' 'action'
		if [ "$USR_INST" -eq 1 ]; then msg "detecté maly en $USR_BIN" "found maly at $USR_BIN"; fi
		if [ "$SYS_INST" -eq 1 ]; then msg "detecté maly en $SYS_BIN" "found maly at $SYS_BIN"; fi
	fi
	menu "$a_def" 4 \
		'instalar' 'install' \
		'actualizar (recompilar y reinstalar)' 'update (rebuild and reinstall)' \
		'desinstalar' 'uninstall' \
		'salir' 'quit'
	case $REPLY in
	1) ACTION=install ;;
	2) ACTION=update ;;
	3) ACTION=uninstall ;;
	4) exit 0 ;;
	esac
fi

if [ "$ACTION" = update ] && [ "$USR_INST" -eq 0 ] && [ "$SYS_INST" -eq 0 ]; then
	# Nada que actualizar: interactivo se ofrece instalar; por flag se avisa.
	if confirm 'no encuentro maly instalado; ¿instalar desde cero?' \
		"couldn't find an installed maly; install from scratch?"; then
		ACTION=install
	else
		die 'no hay maly instalado que actualizar (usa --install)' \
			'no installed maly to update (use --install)'
	fi
fi

# ---- pantalla 2: ámbito (usuario/sistema) ----
if [ "$SYSTEM" -lt 0 ]; then
	s_def=1
	# Con maly solo en /usr/local, actualizar/desinstalar apuntan ahí solos.
	[ "$SYS_INST" -eq 1 ] && [ "$USR_INST" -eq 0 ] && s_def=2
	if [ "$TTY" -eq 1 ] && { [ "$ACTION" = install ] || [ "$s_def" -eq 2 ] || [ "$SYS_INST" -eq 1 ]; }; then
		scr 'ámbito' 'scope'
		menu "$s_def" 2 \
			'usuario (~/.local/bin, sin sudo)' 'user (~/.local/bin, no sudo)' \
			'sistema (/usr/local, para todos; pide sudo)' 'system (/usr/local, for everyone; asks for sudo)'
		SYSTEM=$((REPLY - 1))
	else
		SYSTEM=$((s_def - 1))
	fi
fi

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
		die 'para el ámbito de sistema necesitas sudo (o corre como root)' \
			'for the system scope you need sudo (or run as root)'
	SUDO=sudo
fi

# ---- desinstalar ----
if [ "$ACTION" = uninstall ]; then
	run_phase
	found=0
	for f in "$BIN/maly" "$BASHC/maly" "$FISHC/maly.fish" "$ZSHC/_maly"; do
		if [ -e "$f" ]; then
			$SUDO rm -f "$f"
			found=1
			msg "quitado: $f" "removed: $f"
		fi
	done
	[ "$found" -eq 1 ] || warn 'no encontré nada que quitar en esas rutas' 'nothing to remove at those paths'

	# Config y biblioteca son del usuario y por defecto se respetan; borrar
	# es opción explícita (y sin terminal, jamás).
	CFG_DIR=${XDG_CONFIG_HOME:-$HOME/.config}/maly
	DATA_DIR=${XDG_DATA_HOME:-$HOME/.local/share}/maly
	if [ -d "$CFG_DIR" ] || [ -d "$DATA_DIR" ]; then
		if confirm "¿borrar también tu config y biblioteca? ($CFG_DIR, $DATA_DIR)" \
			"also delete your config and library? ($CFG_DIR, $DATA_DIR)"; then
			rm -rf "$CFG_DIR" "$DATA_DIR"
			msg 'config y biblioteca borradas' 'config and library deleted'
		else
			msg 'tu config, biblioteca y playlists quedan intactas' 'your config, library and playlists are untouched'
		fi
	fi
	exit 0
fi

TMP=$(mktemp -d "${TMPDIR:-/tmp}/mallow.XXXXXX")

# fetch baja una URL a un archivo con curl o wget, lo que haya. Timeouts y
# reintentos acotados: el default de wget son 20 intentos EN SILENCIO — con
# la red mal parecía que el instalador se colgaba y luego moría mudo.
# En curl, --connect-timeout solo cubre el connect TCP: una conexión que
# entra y luego se atasca esperaría PARA SIEMPRE (visto en una VM de Mint,
# colgado tras el 100 % de la barra); --speed-limit/--speed-time abortan si
# baja de 1 B/s por 30 s, el equivalente del -T 30 -t 3 que ya lleva wget.
# OJO: quien llame a fetch debe manejar el fallo (`|| die …`); bajo set -eu
# un fetch suelto que falla termina el script sin mensaje.
fetch() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL --connect-timeout 30 --speed-limit 1 --speed-time 30 -o "$2" "$1"
	elif command -v wget >/dev/null 2>&1; then wget -q -T 30 -t 3 -O "$2" "$1"
	else die 'necesito curl o wget para descargar' 'curl or wget needed to download'
	fi
}

# fetch_small es fetch con tope TOTAL (60 s), solo para archivos de bytes
# (VERSION, .sha256): cierra hasta los modos de atasco que el detector de
# velocidad no alcanza a ver. Jamás usarlo para descargas grandes — un tope
# total mataría un tarball legítimo en una conexión lenta.
fetch_small() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL --connect-timeout 30 --speed-limit 1 --speed-time 30 --max-time 60 -o "$2" "$1"
	elif command -v wget >/dev/null 2>&1; then wget -q -T 20 -t 2 -O "$2" "$1"
	else die 'necesito curl o wget para descargar' 'curl or wget needed to download'
	fi
}

# fetch_show es fetch con barra de progreso, para descargas grandes (el
# tarball de Go tarda minutos y sin salida parece un cuelgue). Redirigido a
# un log, o con un wget viejo sin --show-progress, cae al fetch silencioso.
fetch_show() {
	[ -t 1 ] || { fetch "$1" "$2"; return; }
	if command -v curl >/dev/null 2>&1; then
		curl -fSL --connect-timeout 30 --speed-limit 1 --speed-time 30 --progress-bar -o "$2" "$1"
	elif command -v wget >/dev/null 2>&1 && wget --help 2>/dev/null | grep -q -- --show-progress; then
		wget -q --show-progress -T 30 -t 3 -O "$2" "$1"
	else
		fetch "$1" "$2"
	fi
}

# ---- fuente: el checkout donde corre el script, o un clon temporal ----
SRC=''
script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" 2>/dev/null && pwd) || script_dir=''
if [ -n "$script_dir" ] && [ -f "$script_dir/go.mod" ] && [ -d "$script_dir/cmd/maly" ]; then
	SRC=$script_dir
fi

# ---- gestor de paquetes ----
INSTALL_CMD=''
if command -v pacman >/dev/null 2>&1; then INSTALL_CMD='pacman -S --needed --noconfirm'
elif command -v apt-get >/dev/null 2>&1; then INSTALL_CMD='apt-get install -y'
elif command -v dnf >/dev/null 2>&1; then INSTALL_CMD='dnf install -y'
elif command -v zypper >/dev/null 2>&1; then INSTALL_CMD='zypper --non-interactive install'
elif command -v xbps-install >/dev/null 2>&1; then INSTALL_CMD='xbps-install -y'
fi
PM=${INSTALL_CMD%% *}

# ytdlp_stale dice si el yt-dlp del PATH tiene más de YTDLP_STALE_DAYS
# días. yt-dlp versiona por fecha (YYYY.MM.DD) y releasea casi semanal;
# YouTube rompe las versiones viejas (los repos de apt van meses atrás),
# así que un umbral de versión fija envejecería en semanas — la
# antigüedad no. Solo usa `date +%…` POSIX: -d/+%s no son portables
# (musl/busybox/BSD divergen). La aritmética es aproximada (meses de 30
# días, sin bisiestos): con umbral de meses el margen sobra. Formato
# irreconocible (fork, sufijo raro) = no determinable = NO obsoleto, para
# no molestar builds legítimos. Deja versión y edad en YTDLP_VER/_AGE.
YTDLP_STALE_DAYS=90
YTDLP_VER='' YTDLP_AGE=''
ytdlp_stale() {
	YTDLP_VER=$(yt-dlp --version 2>/dev/null | sed -n 1p) || YTDLP_VER=''
	case $YTDLP_VER in
	[0-9][0-9][0-9][0-9].[0-9][0-9].[0-9][0-9] | [0-9][0-9][0-9][0-9].[0-9][0-9].[0-9][0-9].*) ;;
	*) return 1 ;;
	esac
	yv_y=${YTDLP_VER%%.*}
	yv_r=${YTDLP_VER#*.}
	yv_m=${yv_r%%.*}
	yv_r=${yv_r#*.}
	yv_d=${yv_r%%.*}
	yv_t=$(date +%Y.%m.%d)
	yt_y=${yv_t%%.*}
	yv_r=${yv_t#*.}
	yt_m=${yv_r%%.*}
	yt_d=${yv_r#*.}
	# Pelar el cero líder: la aritmética POSIX leería 08/09 como octal roto.
	yv_m=${yv_m#0} yv_d=${yv_d#0} yt_m=${yt_m#0} yt_d=${yt_d#0}
	YTDLP_AGE=$(( (yt_y*365 + (yt_m-1)*30 + yt_d) - (yv_y*365 + (yv_m-1)*30 + yv_d) ))
	[ "$YTDLP_AGE" -gt "$YTDLP_STALE_DAYS" ]
}

# ---- pantalla 3: dependencias ----
# Solo aparecen las que faltan; mpv y git van marcados (sin ellos maly no
# suena / no hay qué compilar), yt-dlp+ffmpeg y el visualizador son
# opcionales y arrancan desmarcados. `maly get` es un comando opcional: sus
# herramientas no deben colarse en una instalación por defecto. Un yt-dlp
# presente pero viejo cuenta como faltante: existir no le sirve de nada a
# `maly get` si YouTube ya no le responde.
DEPS=''
SEL_mpv=1 SEL_git=1 SEL_get=0 SEL_viz=0
YTDLP_STALE=0
if command -v yt-dlp >/dev/null 2>&1; then
	ytdlp_stale && YTDLP_STALE=1
	YTDLP_MISSING=0
else
	YTDLP_MISSING=1
fi
command -v mpv >/dev/null 2>&1 || DEPS="$DEPS mpv"
if [ -z "$SRC" ] && ! command -v git >/dev/null 2>&1; then DEPS="$DEPS git"; fi
if [ "$YTDLP_MISSING" -eq 1 ] || [ "$YTDLP_STALE" -eq 1 ] || ! command -v ffmpeg >/dev/null 2>&1; then
	DEPS="$DEPS get"
fi
if ! command -v pw-record >/dev/null 2>&1 && ! command -v parec >/dev/null 2>&1; then
	DEPS="$DEPS viz"
fi

dep_label() {
	case $1 in
	mpv) tr2 'mpv — motor de audio (sin él maly no suena)' 'mpv — audio engine (maly cannot play without it)' ;;
	git) tr2 'git — para clonar el repositorio' 'git — to clone the repository' ;;
	get) tr2 'yt-dlp + ffmpeg — para `maly get`, descargar música (opcional)' 'yt-dlp + ffmpeg — for `maly get`, music download (optional)' ;;
	viz) tr2 'pulseaudio-utils (parec) — visualizador con audio real (opcional)' 'pulseaudio-utils (parec) — real-audio visualizer (optional)' ;;
	esac
}

# dep_at devuelve la clave i-ésima (1-based) de $DEPS.
dep_at() {
	da_i=1
	for da_k in $DEPS; do
		if [ "$da_i" -eq "$1" ]; then
			printf '%s' "$da_k"
			return 0
		fi
		da_i=$((da_i + 1))
	done
}

# La actualización no re-ofrece opcionales: solo asegura lo imprescindible.
if [ "$TTY" -eq 1 ] && [ "$ACTION" != update ] && [ -n "$DEPS" ]; then
	scr 'dependencias' 'dependencies'
	if [ "$YTDLP_STALE" -eq 1 ]; then
		warn "tu yt-dlp es de $YTDLP_VER (~$YTDLP_AGE días): YouTube cambia seguido y esa versión suele fallar — marcarlo instala uno reciente sin tocar el del sistema" \
			"your yt-dlp is from $YTDLP_VER (~$YTDLP_AGE days old): YouTube changes often and that version tends to fail — selecting it installs a recent one without touching the system's"
	fi
	msg 'esto falta en tu sistema; marca qué instalar:' 'these are missing on your system; pick what to install:'
	d_n=0
	for d_k in $DEPS; do d_n=$((d_n + 1)); done
	if [ "$RAWOK" -eq 1 ]; then
		# Checklist con cursor: ↑↓/jk mueve, espacio (o el dígito) marca,
		# Enter continúa; se redibuja en el sitio como el menú.
		d_cur=1
		d_drawn=0
		raw_on
		while :; do
			[ "$d_drawn" -eq 0 ] || printf '\033[%dA' "$((d_n + 1))" > /dev/tty
			d_i=1
			for d_k in $DEPS; do
				eval "d_on=\$SEL_$d_k"
				d_box='[ ]'
				[ "$d_on" -eq 0 ] || d_box="[${GN}x${NC}]"
				if [ "$d_i" -eq "$d_cur" ]; then
					printf '\r\033[2K  %s❯%s %s %s%s%s\n' "$CY$BD" "$NC" "$d_box" "$BD" "$(dep_label "$d_k")" "$NC" > /dev/tty
				else
					printf '\r\033[2K    %s %s%s%s\n' "$d_box" "$DM" "$(dep_label "$d_k")" "$NC" > /dev/tty
				fi
				d_i=$((d_i + 1))
			done
			printf '\r\033[2K  %s%s%s\n' "$DM" "$(tr2 '↑↓ mover · espacio (des)marcar · Enter continuar' '↑↓ move · space toggle · Enter continue')" "$NC" > /dev/tty
			d_drawn=1
			d_tok=$(key_read)
			case $d_tok in
			up) [ "$d_cur" -le 1 ] || d_cur=$((d_cur - 1)) ;;
			down) [ "$d_cur" -ge "$d_n" ] || d_cur=$((d_cur + 1)) ;;
			space)
				d_k=$(dep_at "$d_cur")
				eval "SEL_$d_k=\$((1 - SEL_$d_k))" ;;
			enter) break ;;
			digit-*)
				d_d=${d_tok#digit-}
				if [ "$d_d" -ge 1 ] && [ "$d_d" -le "$d_n" ]; then
					d_k=$(dep_at "$d_d")
					eval "SEL_$d_k=\$((1 - SEL_$d_k))"
				fi ;;
			esac
		done
		raw_off
	else
		while :; do
			d_i=1
			for d_k in $DEPS; do
				eval "d_on=\$SEL_$d_k"
				d_box='[ ]'
				[ "$d_on" -eq 1 ] && d_box='[x]'
				printf '  %s%d%s %s %s\n' "$CY" "$d_i" "$NC" "$d_box" "$(dep_label "$d_k")"
				d_i=$((d_i + 1))
			done
			ask '' 'número para (des)marcar, Enter para continuar:' 'number to toggle, Enter to continue:'
			[ -n "$REPLY" ] || break
			d_i=1
			for d_k in $DEPS; do
				if [ "$REPLY" = "$d_i" ]; then eval "SEL_$d_k=\$((1 - SEL_$d_k))"; fi
				d_i=$((d_i + 1))
			done
		done
	fi
fi

# git desmarcado con clon pendiente no tiene arreglo aguas abajo: cortar ya.
case " $DEPS " in *' git '*)
	[ "$SEL_git" -eq 1 ] ||
		die 'sin git no puedo clonar; márcalo, instálalo tú, o corre el script desde un checkout' \
			'without git I cannot clone; select it, install it yourself, or run the script from a checkout'
	;;
esac

# De aquí en adelante se ejecuta: pantalla limpia y log corrido con
# scrollback (la salida de pacman/apt/go debe quedar visible).
run_phase

# ---- paquetes a instalar según la selección ----
PKGS='' PIPX_YTDLP=0
for d_k in $DEPS; do
	eval "d_on=\$SEL_$d_k"
	[ "$d_on" -eq 1 ] || continue
	case $d_k in
	mpv) PKGS="$PKGS mpv" ;;
	git) PKGS="$PKGS git" ;;
	get)
		command -v ffmpeg >/dev/null 2>&1 || PKGS="$PKGS ffmpeg"
		# Faltante u obsoleto da igual: ambos necesitan uno fresco.
		if [ "$YTDLP_MISSING" -eq 1 ] || [ "$YTDLP_STALE" -eq 1 ]; then
			if [ "$PM" = apt-get ]; then
				# Debian/Ubuntu empaquetan un yt-dlp viejo que ya no baja
				# de YouTube: va vía pipx, que instala el actual en
				# ~/.local/bin sin tocar el del sistema.
				PIPX_YTDLP=1
				command -v pipx >/dev/null 2>&1 || PKGS="$PKGS pipx"
			else
				# En repos rodantes `install` del gestor también actualiza
				# uno viejo (pacman --needed lo deja; ahí actualiza el
				# sistema, no este script).
				PKGS="$PKGS yt-dlp"
			fi
		fi ;;
	viz)
		# parec en vez de pw-record: existe en PulseAudio puro y en PipeWire
		# (vía pipewire-pulse), así el paquete es el mismo casi en todos lados.
		if [ "$PM" = pacman ]; then PKGS="$PKGS libpulse"; else PKGS="$PKGS pulseaudio-utils"; fi ;;
	esac
done

if [ -n "$PKGS" ]; then
	PKG_SUDO=''
	[ "$(id -u)" -ne 0 ] && PKG_SUDO='sudo '
	if [ -z "$INSTALL_CMD" ]; then
		die "no reconozco tu gestor de paquetes; instala a mano:$PKGS" \
			"couldn't detect your package manager; install manually:$PKGS"
	fi
	msg "a instalar:$PKGS" "to install:$PKGS"
	if confirm "¿instalar con \`$PKG_SUDO$INSTALL_CMD$PKGS\`?" \
		"install with \`$PKG_SUDO$INSTALL_CMD$PKGS\`?"; then
		[ -z "$PKG_SUDO" ] || command -v sudo >/dev/null 2>&1 ||
			die 'no hay sudo; instala las dependencias como root y reintenta' \
				'no sudo available; install the dependencies as root and retry'
		# shellcheck disable=SC2086
		$PKG_SUDO$INSTALL_CMD$PKGS ||
			die 'falló la instalación de dependencias' 'dependency install failed'
	else
		die "instálalo tú y reintenta:  $PKG_SUDO$INSTALL_CMD$PKGS" \
			"install it yourself and retry:  $PKG_SUDO$INSTALL_CMD$PKGS"
	fi
fi

if [ "$PIPX_YTDLP" -eq 1 ]; then
	msg 'instalando yt-dlp actual vía pipx (el de los repos apt va meses atrás y YouTube lo rompe)' \
		'installing current yt-dlp via pipx (the apt repo one lags months behind and YouTube breaks it)'
	# --force: si el viejo era un pipx previo, reinstala en vez de fallar
	# con "already installed".
	pipx install --force yt-dlp ||
		die 'falló `pipx install yt-dlp`' '`pipx install yt-dlp` failed'
	msg 'yt-dlp quedó en ~/.local/bin (pipx); `pipx upgrade yt-dlp` lo actualiza' \
		'yt-dlp lives in ~/.local/bin (pipx); `pipx upgrade yt-dlp` updates it'
fi

if [ -z "$SRC" ]; then
	if [ -n "$REF" ]; then
		hb_start "clonando Malody Mallow ($REF)…" "cloning Malody Mallow ($REF)…"
	else
		hb_start 'clonando Malody Mallow…' 'cloning Malody Mallow…'
	fi
	# ${REF:+…}: --branch acepta tags; sin --ref se compila main, como siempre.
	# advice.detachedHead: clonar un tag deja checkout suelto y git imprime su
	# consejo de "detached HEAD" AUNQUE lleve --quiet — puro susto en el wizard.
	git -c advice.detachedHead=false clone --quiet --depth=1 ${REF:+--branch "$REF"} "$REPO_URL" "$TMP/src" ||
		{ hb_stop; die 'falló el clonado' 'clone failed'; }
	hb_done
	SRC=$TMP/src
else
	[ -z "$REF" ] || warn "corriendo desde un checkout: --ref=$REF se ignora, se compila lo que hay aquí" \
		"running from a checkout: --ref=$REF is ignored, building what is here"
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
	fetch_small 'https://go.dev/VERSION?m=text' "$TMP/gover" ||
		die 'no pude consultar la versión de Go en go.dev (¿red, proxy?)' \
			"couldn't query the Go version from go.dev (network, proxy?)"
	GOV=''
	IFS= read -r GOV < "$TMP/gover" || : # sin newline final read devuelve ≠0 pero SÍ llena la variable
	case "$GOV" in go1.*) ;; *) die "respuesta rara de go.dev: $GOV" "odd reply from go.dev: $GOV" ;; esac
	msg "bajando $GOV linux/$GOARCH…" "downloading $GOV linux/$GOARCH…"
	fetch_show "https://go.dev/dl/$GOV.linux-$GOARCH.tar.gz" "$TMP/go.tgz" ||
		die 'falló la descarga de Go; revisa tu conexión y reintenta' \
			'the Go download failed; check your connection and retry'
	# Verificar el SHA-256 publicado junto al tarball: TLS ya protege el
	# transporte, esto cubre un mirror/caché comprometido. Sin sha256sum en
	# el sistema (rarísimo: coreutils/busybox lo traen) se avisa y sigue.
	# Con heartbeat: hashear 80 MB en un disco lento son minutos — sin
	# latido parece un cuelgue (visto en la VM de Mint; el ping salía limpio
	# porque este tramo ni toca la red). die ya hace hb_stop.
	if command -v sha256sum >/dev/null 2>&1; then
		hb_start 'verificando la descarga…' 'verifying the download…'
		# dl.google.com sirve el .sha256 plano; go.dev/dl devolvería HTML.
		SUMURL="https://dl.google.com/go/$GOV.linux-$GOARCH.tar.gz.sha256"
		fetch_small "$SUMURL" "$TMP/go.tgz.sha256" ||
			die "no pude bajar el checksum de Go ($SUMURL)" \
				"couldn't download the Go checksum ($SUMURL)"
		# El .sha256 real viene SIN newline final, y ahí read devuelve ≠0
		# aunque SÍ llenó la variable: nunca resetearla en el ||.
		want=''
		IFS= read -r want < "$TMP/go.tgz.sha256" || :
		want=${want%% *} # formatos viejos traen "hash  archivo"
		[ -n "$want" ] ||
			die "el checksum descargado vino vacío o ilegible ($SUMURL)" \
				"the downloaded checksum came back empty or unreadable ($SUMURL)"
		got=$(sha256sum "$TMP/go.tgz")
		got=${got%% *}
		[ "$got" = "$want" ] ||
			die "el checksum del Go bajado no coincide: descarga corrupta (esperaba $want, obtuve $got)" \
				"downloaded Go checksum mismatch: corrupt download (expected $want, got $got)"
		hb_done
	else
		warn 'sin sha256sum no puedo verificar la descarga de Go' \
			'without sha256sum the Go download cannot be verified'
	fi
	# El heartbeat arranca ANTES del rm: borrar un Go previo (~12 mil
	# archivos) en un disco lento es otro tramo mudo de minutos, y extraer
	# ~240 MB también.
	hb_start "extrayendo Go en $CACHE/go…" "extracting Go into $CACHE/go…"
	rm -rf "$CACHE/go"
	mkdir -p "$CACHE"
	tar -C "$CACHE" -xzf "$TMP/go.tgz" ||
		{ hb_stop; die 'falló la extracción de Go (¿descarga incompleta, disco lleno?)' \
			'extracting Go failed (incomplete download, disk full?)'; }
	hb_done
	GO=$CACHE/go/bin/go
fi

# ---- compilar ----
hb_start 'compilando maly… (la primera vez baja dependencias de Go)' \
	'building maly… (first run downloads Go dependencies)'
(cd "$SRC" && "$GO" build -trimpath -ldflags '-s -w' -o "$TMP/maly" ./cmd/maly) ||
	{ hb_stop; die 'falló la compilación' 'build failed'; }
hb_done

# ---- instalar binario y completions ----
# La versión previa se lee antes de pisar el binario, para el delta final.
OLDVER=$("$BIN/maly" version 2>/dev/null | sed -n 's/.*\(v[0-9][0-9.]*\).*/\1/p' | sed -n 1p) || OLDVER=''
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
if ! command -v mpv >/dev/null 2>&1; then
	sep
	warn 'sin mpv maly no puede reproducir nada; instálalo cuando puedas' \
		'without mpv maly cannot play anything; install it when you can'
fi
if ! command -v pw-record >/dev/null 2>&1 && ! command -v parec >/dev/null 2>&1; then
	sep
	warn 'sin pw-record/parec el visualizador queda en modo animación (opcional: pipewire o pulseaudio-utils)' \
		'without pw-record/parec the visualizer stays in animation mode (optional: pipewire or pulseaudio-utils)'
fi
if [ "$PIPX_YTDLP" -eq 0 ] &&
	{ ! command -v yt-dlp >/dev/null 2>&1 || ! command -v ffmpeg >/dev/null 2>&1; }; then
	sep
	warn 'sin yt-dlp y ffmpeg no funciona `maly get` (descargar música; opcional)' \
		'without yt-dlp and ffmpeg `maly get` will not work (music download; optional)'
fi

printf '\n'
ver=$("$TMP/maly" version | sed -n 1p)
ok "listo: ${BD}${ver}${NC}" "done: ${BD}${ver}${NC}"
newver=$(printf '%s' "$ver" | sed -n 's/.*\(v[0-9][0-9.]*\).*/\1/p')
if [ -n "$OLDVER" ] && [ -n "$newver" ] && [ "$OLDVER" != "$newver" ]; then
	msg "actualizado: $OLDVER → $newver" "updated: $OLDVER → $newver"
fi
# Un servicio maly vivo sigue corriendo el binario ANTERIOR hasta reiniciarlo
# (la TUI lo avisa, pero desde CLI pura nadie se entera). Mismas rutas de
# socket que config.RuntimeDir: $XDG_RUNTIME_DIR/maly o el fallback en tmp.
if [ -n "${XDG_RUNTIME_DIR:-}" ]; then
	MSOCK=$XDG_RUNTIME_DIR/maly/maly.sock
else
	MSOCK=${TMPDIR:-/tmp}/maly-$(id -u)/maly.sock
fi
if [ -S "$MSOCK" ]; then
	warn 'el servicio maly sigue corriendo el binario anterior: reinícialo con  maly kill  (la música se pausa; la sesión se conserva)' \
		'the maly service is still running the previous binary: restart it with  maly kill  (playback pauses; the session is kept)'
fi
msg 'primer paso:  maly scan   (indexa ~/Music; acepta otra ruta) · luego:  maly' \
	'first step:  maly scan   (indexes ~/Music; takes another path) · then:  maly'
