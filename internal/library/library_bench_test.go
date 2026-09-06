package library

import (
	"fmt"
	"path/filepath"
	"testing"
)

// Benchmarks persistentes para Search/Scan (hallazgo TEST-3 de la auditoría
// técnica): las rondas de rendimiento anteriores (1.7.1, 1.7.3, ver
// docs/history/roadmap-1.0-1.7.md) midieron con benchmarks desechables + pprof, borrados tras
// medir — sin nada commiteado, una regresión futura (p. ej. alguien
// reintroduce un Search("") sin LIMIT en un camino nuevo, exactamente lo
// que causó la 1.7.3) solo se detecta en la próxima auditoría manual, no en
// cada cambio. No están en CI a propósito: el hardware de los runners de
// GitHub Actions varía entre corridas y no da una línea base fiable: se
// corren a mano antes de cada release y se comparan contra los números ya
// documentados en el roadmap.
//
//	go test ./internal/library/ -run '^$' -bench . -benchtime=3x

// BenchmarkScan mide el costo de indexar una biblioteca desde cero.
func BenchmarkScan(b *testing.B) {
	const n = 5000
	dir := fakeMusicDir(b, n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		lib, err := Open(filepath.Join(b.TempDir(), fmt.Sprintf("bench%d.db", i)))
		if err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		if _, err := lib.Scan(dir, nil); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		lib.Close()
	}
}

// BenchmarkSearchLimit mide el caso que rompió la 1.7.3: el completado de
// shell pide 30 candidatos con la palabra parcial vacía (completeFetch), que
// SearchLimit resuelve con un LIMIT en SQL en vez de materializar la
// biblioteca entera. Una regresión que vuelva a caer en Search("")/All() se
// vería aquí como un salto de orden de magnitud, no solo unos ms.
func BenchmarkSearchLimit(b *testing.B) {
	const n = 20000
	dir := fakeMusicDir(b, n)
	lib, err := Open(filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer lib.Close()
	if _, err := lib.Scan(dir, nil); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := lib.SearchLimit("", 30); err != nil {
			b.Fatal(err)
		}
	}
}
