package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"maly/internal/config"
	"maly/internal/library"
	"maly/internal/playlist"
)

func TestSaveQueueConcurrentSnapshot(t *testing.T) {
	d := newTestDaemon(t)
	go d.Run()
	music := t.TempDir()
	for _, name := range []string{"a.mp3", "b.mp3"} {
		if err := os.WriteFile(filepath.Join(music, name), []byte("dummy"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.lib.Scan(music, nil); err != nil {
		t.Fatal(err)
	}
	tracks, err := d.lib.All()
	if err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	d.q.Replace([]library.Track{tracks[0], tracks[1], tracks[0]})
	d.q.SetShuffle(true)
	d.mu.Unlock()
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			d.mu.Lock()
			d.q.Move(0, 1)
			d.mu.Unlock()
			time.Sleep(100 * time.Microsecond)
		}
	}()
	defer func() { close(stop); wg.Wait() }()
	want1 := []string{tracks[0].Path, tracks[1].Path, tracks[0].Path}
	want2 := []string{tracks[1].Path, tracks[0].Path, tracks[0].Path}
	for i := 0; i < 25; i++ {
		name := fmt.Sprintf("foto %d", i)
		n, err := playlist.SaveQueue(config.SocketPath(), config.DBPath(), name)
		if err != nil || n != 3 {
			t.Fatalf("guardado: %d / %v", n, err)
		}
		saved, err := d.lib.PlaylistTracks(name)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, tr := range saved {
			got = append(got, tr.Path)
		}
		if !reflect.DeepEqual(got, want1) && !reflect.DeepEqual(got, want2) {
			t.Fatalf("instantánea mezclada: %q", got)
		}
	}
}
