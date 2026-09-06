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

// BenchmarkState mide el trabajo que hace UN push: armar la foto (state) y
// serializarla. Los dos juntos a propósito: es la unidad que el suscriptor
// paga cada 250 ms, y el marshal es la mitad cara (toInfos copia la cola,
// json.Marshal la recorre otra vez emitiendo ~250 B por pista).
//
// La métrica que importa no es ns/op sino B/push, que es lo que A-05 baja:
// se reporta aparte para poder comparar antes/después sin depender de la
// máquina.
func BenchmarkState(b *testing.B) {
	for _, n := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("cola=%d", n), func(b *testing.B) {
			d := newTestDaemon(b)
			d.q.Add(benchTracks(n)...)
			// Fuera del cronómetro: mide el push, no la primera vuelta.
			bytesPerPush := 0
			if data, err := json.Marshal(d.state("es")); err == nil {
				bytesPerPush = len(data)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				data, err := json.Marshal(d.state("es"))
				if err != nil {
					b.Fatal(err)
				}
				if len(data) == 0 {
					b.Fatal("push vacío")
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(bytesPerPush), "B/push")
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
