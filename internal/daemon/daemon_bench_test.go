package daemon

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"maly/internal/library"
)

// Benchmark persistente del PUSH de suscripción, por tamaño de cola (O-05.2).
// Mismo criterio que BenchmarkScan/BenchmarkSearchLimit en internal/library:
// no está en CI —el hardware de los runners varía y no da línea base— sino
// que se corre a mano y se compara contra los números del roadmap.
//
//	go test ./internal/daemon/ -run '^$' -bench BenchmarkState -benchtime=100x
//
// Existe porque ninguna de las rondas de rendimiento anteriores (1.7.1,
// 1.7.3) midió este camino: midieron `status` por IPC (<0,1 ms) y la
// búsqueda, y el push quedó fuera. Lo que costaba de verdad era esto —5,3
// MB/s con una cola de 5.000 pistas, cuatro veces por segundo y por cliente
// (A-05)— y solo se vio en la auditoría del 2026-09, con un suscriptor
// externo contando bytes.

// BenchmarkState mide el trabajo que hace UN push: armar la foto (stateFor) y
// serializarla. Los dos juntos a propósito: es la unidad que el suscriptor
// paga cada 250 ms, y el marshal es la mitad cara (toInfos copia la cola,
// json.Marshal la recorre otra vez emitiendo ~250 B por pista).
//
// Lo cronometrado es el push EN RÉGIMEN, que es el que ocurre cuatro veces
// por segundo: la cola sin cambios. Se reportan dos métricas de tamaño, que
// son las que A-05 separa y lo que importa mirar, más que los ns/op:
//
//   - B/push  — el push en régimen. Antes de A-05 llevaba la cola entera.
//   - B/full  — el push que SÍ lleva la cola (el inicial, y cada mutación).
//     Es lo que medía este mismo benchmark antes del arreglo, así que sirve
//     para comparar contra la línea base del commit anterior.
func BenchmarkState(b *testing.B) {
	for _, n := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("cola=%d", n), func(b *testing.B) {
			d := newTestDaemon(b)
			d.q.Add(benchTracks(n)...)
			s := &subscriber{lang: "es"}
			// Primer push fuera del cronómetro: es el que lleva la cola y
			// deja al suscriptor al día. Su tamaño es B/full.
			full, err := json.Marshal(d.stateFor(s))
			if err != nil {
				b.Fatal(err)
			}
			steady := 0
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				data, err := json.Marshal(d.stateFor(s))
				if err != nil {
					b.Fatal(err)
				}
				steady = len(data)
			}
			b.StopTimer()
			b.ReportMetric(float64(steady), "B/push")
			b.ReportMetric(float64(len(full)), "B/full")
		})
	}
}

// benchTracks fabrica n pistas con campos de largo realista (~250 B de JSON
// cada una, que es la escala con la que este proyecto ya benchmarkeó su
// biblioteca). No tocan el disco: state() solo lee la cola en memoria.
func benchTracks(n int) []library.Track {
	out := make([]library.Track, n)
	for i := range out {
		out[i] = library.Track{
			ID:          int64(i + 1),
			Path:        filepath.Join("/home/usuario/Música", fmt.Sprintf("Artista Que Escribe Largo/Álbum Doble Remasterizado/%02d - Título De La Canción.mp3", i%99+1)),
			Title:       fmt.Sprintf("Título De La Canción %d", i),
			Artist:      "Artista Que Escribe Largo",
			Album:       "Álbum Doble Remasterizado",
			AlbumArtist: "Artista Que Escribe Largo",
			Genre:       "Rock Progresivo",
			TrackNo:     i%99 + 1,
			Duration:    212.5,
		}
	}
	return out
}
