package getter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeYtdlp instala un yt-dlp falso cuyo cuerpo es body y registra los
// argumentos recibidos (uno por línea) en el archivo que devuelve. Mismo
// patrón que el yt-dlp falso de cmd/maly/get_test.go y el mpv falso de
// player_test.go: los tests de búsqueda no tocan la red jamás.
//
// El PATH real queda DETRÁS del directorio falso, no fuera: algunos cuerpos
// necesitan utilidades externas (cat para el heredoc, sleep para el timeout)
// y el PATH aislado de get_test.go no las tiene — poner el falso PRIMERO ya
// garantiza que gane sobre un yt-dlp de verdad instalado en la máquina.
func fakeYtdlp(t *testing.T, body string) (argsFile string) {
	t.Helper()
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	argsFile = filepath.Join(tmp, "args.txt")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + strconv.Quote(argsFile) + "\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(bin, "yt-dlp"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsFile
}

// emit arma un cuerpo de yt-dlp falso que escribe lines a stdout. Heredoc con
// delimitador entre comillas: el shell no expande NADA del contenido, así que
// el JSON viaja byte a byte como lo escribió el test.
func emit(lines ...string) string {
	return "cat <<'MALYEOF'\n" + strings.Join(lines, "\n") + "\nMALYEOF"
}

func fakeArgsOf(t *testing.T, argsFile string) []string {
	t.Helper()
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

// TestSearchArgs fija la invocación: --flat-playlist (no resolver cada video)
// y --dump-json (interfaz de máquina), el spec ytsearchN: al final tras "--"
// como en Command, y NINGÚN --cookies-from-browser: buscar no lo necesita y
// con Chromium ese flag puede pedir desbloquear el keyring en cada búsqueda.
func TestSearchArgs(t *testing.T) {
	argsFile := fakeYtdlp(t, emit())

	if _, err := Search(context.Background(), "  aurora runaway  ", 5); err != nil {
		t.Fatal(err)
	}

	args := fakeArgsOf(t, argsFile)
	if got := args[len(args)-1]; got != "ytsearch5:aurora runaway" {
		t.Errorf("el spec debía ser ytsearch5:aurora runaway, fue %q", got)
	}
	if got := args[len(args)-2]; got != "--" {
		t.Errorf("el spec debe ir tras --, había %q", got)
	}
	joined := strings.Join(args, " ")
	for _, flag := range []string{"--flat-playlist", "--dump-json"} {
		if !strings.Contains(joined, flag) {
			t.Errorf("falta %q en la invocación: %v", flag, args)
		}
	}
	if strings.Contains(joined, "--cookies-from-browser") {
		t.Errorf("la búsqueda no debe mandar cookies: %v", args)
	}
}

// TestSearchN: n <= 0 cae en el default y por encima del tope se recorta.
func TestSearchN(t *testing.T) {
	for _, tc := range []struct{ in, want int }{{0, defaultResults}, {-3, defaultResults}, {999, maxResults}, {7, 7}} {
		argsFile := fakeYtdlp(t, emit())
		if _, err := Search(context.Background(), "x", tc.in); err != nil {
			t.Fatal(err)
		}
		args := fakeArgsOf(t, argsFile)
		want := "ytsearch" + strconv.Itoa(tc.want) + ":x"
		if got := args[len(args)-1]; got != want {
			t.Errorf("Search(n=%d) pidió %q, quería %q", tc.in, got, want)
		}
	}
}

func TestSearchParsesResults(t *testing.T) {
	fakeYtdlp(t, emit(
		`{"title":"AURORA - Runaway","uploader":"AURORA","duration":250,"url":"https://www.youtube.com/watch?v=aaa"}`,
		`{"title":"Runaway (Live)","channel":"AURORA VEVO","duration":null,"url":"https://www.youtube.com/watch?v=bbb"}`,
		`{"title":"Runaway cover","uploader":"Alguien","duration":-5,"url":"https://www.youtube.com/watch?v=ccc"}`,
	))

	res, err := Search(context.Background(), "runaway", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 3 {
		t.Fatalf("quería 3 resultados, hubo %d: %+v", len(res), res)
	}
	if res[0].Title != "AURORA - Runaway" || res[0].Uploader != "AURORA" || res[0].Duration != 250 {
		t.Errorf("primer resultado mal decodificado: %+v", res[0])
	}
	if res[0].URL != "https://www.youtube.com/watch?v=aaa" {
		t.Errorf("la URL debe ser la que reportó yt-dlp, fue %q", res[0].URL)
	}
	// Sin uploader se cae a channel (--flat-playlist no siempre trae los dos).
	if res[1].Uploader != "AURORA VEVO" {
		t.Errorf("sin uploader debía usarse channel, hubo %q", res[1].Uploader)
	}
	// duration null y duration negativa quedan en 0 = desconocida.
	if res[1].Duration != 0 || res[2].Duration != 0 {
		t.Errorf("una duración null o negativa debe quedar en 0: %v / %v", res[1].Duration, res[2].Duration)
	}
}

// TestSearchTitleConSeparadores es el test que justifica --dump-json en vez de
// --print con un separador: los títulos REALES de YouTube traen el separador
// dentro (este salió tal cual de buscar "aurora runaway") y además comillas.
// Partir por delimitador dejaría el título cortado a la mitad.
func TestSearchTitleConSeparadores(t *testing.T) {
	const want = `AURORA - Runaway | Sub "Español" - Lyrics + (VIDEO OFICIAL) HD`
	fakeYtdlp(t, emit(
		`{"title":"AURORA - Runaway | Sub \"Español\" - Lyrics + (VIDEO OFICIAL) HD","uploader":"ANDREW BUTRON","duration":250,"url":"https://x/1"}`,
	))

	res, err := Search(context.Background(), "runaway", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Title != want {
		t.Fatalf("el título llegó cortado o mal: %+v", res)
	}
}

// TestSearchSanea: los títulos y canales son texto AJENO que llega solo con
// buscar, y el recorte de la TUI conserva los escapes ANSI. Sin
// safetext.Clean, este título le cambia el título a la ventana del usuario.
func TestSearchSanea(t *testing.T) {
	fakeYtdlp(t, emit(
		`{"title":"Runaway\u001b]0;pwned\u0007","uploader":"AUR\u009bORA","duration":1,"url":"https://x/1"}`,
	))

	res, err := Search(context.Background(), "runaway", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("quería 1 resultado, hubo %d", len(res))
	}
	if strings.ContainsRune(res[0].Title, 0x1b) || strings.ContainsRune(res[0].Title, 0x07) {
		t.Errorf("el título conserva caracteres de control: %q", res[0].Title)
	}
	if res[0].Title != "Runaway]0;pwned" {
		t.Errorf("título saneado inesperado: %q", res[0].Title)
	}
	// El CSI de 8 bits (U+009B) es la misma orden en un solo carácter: no
	// basta con filtrar ESC.
	if res[0].Uploader != "AURORA" {
		t.Errorf("uploader saneado inesperado: %q", res[0].Uploader)
	}
}

// TestSearchDescartaInvalidos: lo que no se puede nombrar (sin título) o no se
// puede descargar (sin URL usable) no es elegible y no llega a la lista.
func TestSearchDescartaInvalidos(t *testing.T) {
	fakeYtdlp(t, emit(
		`{"title":"sin url","uploader":"a","duration":1}`,
		`{"title":"esquema plano","uploader":"a","duration":1,"url":"http://x/1"}`,
		`{"title":"","uploader":"a","duration":1,"url":"https://x/2"}`,
		`{"title":"   ","uploader":"a","duration":1,"url":"https://x/3"}`,
		`{"title":"guion inicial","uploader":"a","duration":1,"url":"-oh/no"}`,
		`{"title":"buena","uploader":"a","duration":1,"url":"https://x/4"}`,
	))

	res, err := Search(context.Background(), "x", 6)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Title != "buena" {
		t.Fatalf("solo la entrada válida debía sobrevivir, hubo %+v", res)
	}
}

// TestSearchSinResultados: cero coincidencias es un estado legítimo, no un
// fallo — quien lo consume debe poder distinguirlo de un error.
func TestSearchSinResultados(t *testing.T) {
	fakeYtdlp(t, emit())

	res, err := Search(context.Background(), "asdkjhasdkjh", 5)
	if err != nil {
		t.Fatalf("sin resultados no debe ser error: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("quería lista vacía, hubo %+v", res)
	}
}

// TestSearchFallo: el stderr de yt-dlp dice qué pasó; "exit status 1" no.
func TestSearchFallo(t *testing.T) {
	fakeYtdlp(t, "echo 'ERROR: unable to download webpage' >&2\nexit 1")

	_, err := Search(context.Background(), "x", 5)
	if err == nil {
		t.Fatal("un yt-dlp que falla debe dar error")
	}
	if !strings.Contains(err.Error(), "unable to download webpage") {
		t.Errorf("el error debe llevar el stderr de yt-dlp, fue %q", err)
	}
}

// TestSearchTimeoutPropio ejercita el vencimiento de searchTimeout: el
// mensaje es de maly y menciona el tiempo. Encogido para no esperar 20 s.
func TestSearchTimeoutPropio(t *testing.T) {
	old := searchTimeout
	searchTimeout = 300 * time.Millisecond
	defer func() { searchTimeout = old }()
	fakeYtdlp(t, "sleep 5")

	start := time.Now()
	_, err := Search(context.Background(), "x", 5)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("una búsqueda que se pasa del tiempo debe dar error")
	}
	// Sin exec.CommandContext el proceso corre sus 5 s enteros.
	if elapsed > 3*time.Second {
		t.Errorf("la búsqueda no se cortó: tardó %v", elapsed)
	}
	if !strings.Contains(err.Error(), "300ms") {
		t.Errorf("el error debía mencionar el tiempo agotado, fue %q", err)
	}
}

// TestSearchCancelacionDelLlamador: si quien llama cancela (cerrar la
// pantalla) el error que vuelve es el SUYO, no el mensaje de timeout de maly
// — que ahí sería mentira.
func TestSearchCancelacionDelLlamador(t *testing.T) {
	fakeYtdlp(t, "sleep 5")

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := Search(ctx, "x", 5)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("quería el error del llamador, hubo %v", err)
	}
	if elapsed > 3*time.Second {
		t.Errorf("la cancelación no cortó el proceso: tardó %v", elapsed)
	}
}

// TestSearchConsultaVacia: sin consulta no se invoca a yt-dlp siquiera.
func TestSearchConsultaVacia(t *testing.T) {
	argsFile := fakeYtdlp(t, emit(`{"title":"no debería salir","url":"https://x/1"}`))

	res, err := Search(context.Background(), "   ", 5)
	if err != nil || res != nil {
		t.Fatalf("consulta vacía debe dar (nil, nil), dio (%+v, %v)", res, err)
	}
	if _, err := os.Stat(argsFile); err == nil {
		t.Error("no debía invocarse yt-dlp con la consulta vacía")
	}
}

// TestSearchSinYtdlp: buscar solo necesita yt-dlp (no ffmpeg), y su ausencia
// da el mismo mensaje con instrucciones que el resto de `maly get`.
func TestSearchSinYtdlp(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := Search(context.Background(), "x", 5)
	if err == nil || !strings.Contains(err.Error(), "yt-dlp") {
		t.Errorf("sin yt-dlp en el PATH debe fallar mencionándolo; err = %v", err)
	}
}

// TestSearchDescartaLives: solo se tiran los dos estados que no son
// descargables. La GRABACIÓN de un directo que ya terminó (was_live /
// post_live) es audio legítimo —muchos conciertos viven así— y descartar
// "todo lo que no sea not_live" se la llevaría por delante.
func TestSearchDescartaLives(t *testing.T) {
	fakeYtdlp(t, emit(
		`{"title":"en vivo ahora","uploader":"a","duration":1,"url":"https://x/1","live_status":"is_live"}`,
		`{"title":"estreno","uploader":"a","duration":1,"url":"https://x/2","live_status":"is_upcoming"}`,
		`{"title":"concierto grabado","uploader":"a","duration":1,"url":"https://x/3","live_status":"was_live"}`,
		`{"title":"recién terminado","uploader":"a","duration":1,"url":"https://x/4","live_status":"post_live"}`,
		`{"title":"video normal","uploader":"a","duration":1,"url":"https://x/5","live_status":"not_live"}`,
		`{"title":"sin el campo","uploader":"a","duration":1,"url":"https://x/6"}`,
		`{"title":"valor futuro","uploader":"a","duration":1,"url":"https://x/7","live_status":"algo_nuevo"}`,
	))

	res, err := Search(context.Background(), "x", 7)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range res {
		got = append(got, r.Title)
	}
	want := []string{"concierto grabado", "recién terminado", "video normal", "sin el campo", "valor futuro"}
	if len(got) != len(want) {
		t.Fatalf("sobrevivieron %v, quería %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("resultado %d fue %q, quería %q", i, got[i], want[i])
		}
	}
}
