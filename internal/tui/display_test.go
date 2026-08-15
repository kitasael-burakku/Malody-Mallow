package tui

import (
	"strings"
	"testing"

	"maly/internal/ipc"
	"maly/internal/library"
)

// TestCleanTitleCasosReales usa títulos tal como los deja yt-dlp en la
// biblioteca del dueño (los de las capturas del rediseño), más las variantes
// que la lista de ruido promete cubrir.
func TestCleanTitleCasosReales(t *testing.T) {
	cases := []struct {
		artist, title, want string
	}{
		// El artista repetido dentro del título, con y sin cambio de caja.
		{"Duki", "DUKI — Malbec x Bizarrap", "Malbec x Bizarrap"},
		{"Duki", "Duki - Ameri", "Ameri"},
		{"kaisoyeon", "kaisoyeon | Colchón Vacío", "Colchón Vacío"},
		// Dos veces el artista y un sufijo de formato, todo junto.
		{"blackbear", "blackbear - blackbear - hot girl bummer (with Khea) [Lyric Video]",
			"hot girl bummer (with Khea)"},
		// Sufijos de formato, en sus dos idiomas y con y sin acento.
		{"X", "Song (Official Video)", "Song"},
		{"X", "Song (Official Music Video)", "Song"},
		{"X", "Song [Official Audio]", "Song"},
		{"X", "Song (Lyric Video)", "Song"},
		{"X", "Song [Audio]", "Song"},
		{"X", "Song (Visualizer)", "Song"},
		{"Aitana", "AITANA - Vas A Quedarte (Vídeo Oficial)", "Vas A Quedarte"},
		{"X", "Canción (VIDEO OFICIAL)", "Canción"},
		// Combinaciones que una lista de frases cerrada no cubriría, y que el
		// corpus real trae: la regla es "todas las palabras son de formato".
		{"Deo", "Deo - Solo Tú (Videoclip Oficial)", "Solo Tú"},
		{"Oliver Tree", "Oliver Tree - Jerk [Music Video]", "Jerk"},
		{"KHEA", "KHEA - Creo Que ft. Asan (Official Animated Video)", "Creo Que ft. Asan"},
		{"X", "Song (Official HD Video)", "Song"},
		{"X", "Song (Oficial)", "Song"},
		// Y basta UNA palabra ajena para conservar el grupo entero.
		{"X", "Song (Live Video)", "Song (Live Video)"},
		{"X", "Song (2016-2017)", "Song (2016-2017)"},
		{"X", "Song (prod. Orodembow)", "Song (prod. Orodembow)"},
		{"X", "Song (sped up)", "Song (sped up)"},
		// Varios grupos a la vez, y el espacio doble que deja borrarlos.
		{"X", "Song (Official Video) [Deluxe]", "Song [Deluxe]"},
		{"X", "Song (Audio) (Official)", "Song"},

		// --- Lo que NO se toca: es información de la pista, no formato ---
		{"X", "Song (Remix)", "Song (Remix)"},
		{"X", "Song (feat. Y)", "Song (feat. Y)"},
		{"X", "Song (with Khea)", "Song (with Khea)"},
		{"X", "Song (Live)", "Song (Live)"},
		{"X", "Song (Acústico)", "Song (Acústico)"},
		{"X", "Song (Slowed + Reverb)", "Song (Slowed + Reverb)"},
		// Un guion sin espacios es parte del nombre, no un separador.
		{"Jay-Z", "Jay-Z Song", "Jay-Z Song"},
		// El artista solo se quita si está al PRINCIPIO.
		{"Duki", "Malbec x Bizarrap — Duki", "Malbec x Bizarrap — Duki"},
		// Prefijo parecido pero distinto: no es el artista.
		{"Duki", "Dukiman - Otra", "Dukiman - Otra"},

		// --- Nunca dejar la fila en blanco ---
		{"X", "(Official Video)", "(Official Video)"},
		{"Duki", "Duki", "Duki"},
		// El título era solo el artista y un separador colgando: se recorta el
		// separador, pero el texto sobrevive (la regla es no dejar la fila en
		// blanco, no conservar la basura tal cual).
		{"Duki", "Duki — ", "Duki"},
		{"X", "", ""},
	}
	for _, c := range cases {
		if got := cleanTitle(c.artist, c.title); got != c.want {
			t.Errorf("cleanTitle(%q, %q) = %q, quería %q", c.artist, c.title, got, c.want)
		}
	}
}

// TestCleanTitleIdempotente: limpiar dos veces no puede seguir comiendo texto.
func TestCleanTitleIdempotente(t *testing.T) {
	for _, title := range []string{
		"DUKI — Malbec x Bizarrap (Official Video)",
		"Song (Remix)", "Song", "blackbear - hot girl bummer [Audio]",
	} {
		once := cleanTitle("Duki", title)
		if twice := cleanTitle("Duki", once); twice != once {
			t.Errorf("%q: primera pasada %q, segunda %q", title, once, twice)
		}
	}
}

// TestTrackLabelFormato: el label de display mantiene "Artista — Título" y
// aguanta un artista vacío (pistas sin tag), igual que ipc.TrackInfo.String.
func TestTrackLabelFormato(t *testing.T) {
	if got, want := trackLabel("Duki", "DUKI — Ameri"), "Duki — Ameri"; got != want {
		t.Errorf("trackLabel = %q, quería %q", got, want)
	}
	if got, want := trackLabel("", "Song (Official Video)"), "Song"; got != want {
		t.Errorf("trackLabel sin artista = %q, quería %q", got, want)
	}
}

// TestLimpiezaSoloDeDisplay es el criterio de aceptación de la fase: la
// limpieza NO puede filtrarse a los datos. La biblioteca guarda lo que traía
// la etiqueta ID3 y el protocolo lo transporta tal cual, así que `maly
// search`, la CLI y cualquier script que parsee su salida siguen encontrando
// por el título original.
func TestLimpiezaSoloDeDisplay(t *testing.T) {
	const raw = "DUKI — Malbec x Bizarrap (Official Video)"

	ti := ipc.TrackInfo{Artist: "Duki", Title: raw}
	if got := ti.String(); !strings.Contains(got, "Official Video") {
		t.Errorf("ipc.TrackInfo.String() = %q: el protocolo no debe limpiar nada", got)
	}
	lt := library.Track{Artist: "Duki", Title: raw}
	if got := lt.String(); !strings.Contains(got, "Official Video") {
		t.Errorf("library.Track.String() = %q: la biblioteca no debe limpiar nada", got)
	}
	// Y la búsqueda de la biblioteca sigue plegando el título ENTERO.
	if !strings.Contains(library.Fold(raw), "official video") {
		t.Error("library.Fold dejó de ver el título original")
	}
}

// TestArbolYColaMuestranElTituloLimpio comprueba los dos paneles que el brief
// nombra (más el picker, que comparte songItems): lo que se dibuja es el
// título limpio, aunque la pista de la biblioteca conserve el original.
func TestArbolYColaMuestranElTituloLimpio(t *testing.T) {
	tr := library.Track{
		ID: 1, Path: "/m/a.mp3", Artist: "blackbear", Album: "Anonymous",
		Title: "blackbear - hot girl bummer (with Khea) [Lyric Video]",
	}
	tree := buildTree([]library.Track{tr}, nil)
	tree.filter = "hot"
	tree.flatten()
	if len(tree.rows) == 0 {
		t.Fatal("el filtro no encontró la pista")
	}
	row := tree.rows[0].label
	if strings.Contains(row, "Lyric Video") || strings.Contains(strings.ToLower(row), "blackbear - blackbear") {
		t.Errorf("árbol: fila sucia %q", row)
	}
	if !strings.Contains(row, "hot girl bummer (with Khea)") {
		t.Errorf("árbol: se perdió información real de la pista: %q", row)
	}

	if got := songItems([]library.Track{tr})[0].label; strings.Contains(got, "Lyric Video") {
		t.Errorf("picker: entrada sucia %q", got)
	}

	m := newLayoutTestModel(160, 42)
	m.queue = []ipc.TrackInfo{{Artist: tr.Artist, Title: tr.Title, Duration: 100}}
	m.status.Track = &ipc.TrackInfo{Artist: tr.Artist, Title: tr.Title, Duration: 100}
	out := m.View()
	if strings.Contains(out, "Lyric Video") {
		t.Error("la vista sigue mostrando el sufijo de formato")
	}
	if !strings.Contains(out, "hot girl bummer") {
		t.Error("la vista perdió el título")
	}
}
