.PHONY: build vet test install clean

# `go build ./...` NO regenera ./maly (compila cada paquete a su propio
# caché, sin dejar binario en la raíz) — por eso el target explícito.
build:
	go build -o maly ./cmd/maly

vet:
	go vet ./...

test:
	go test ./...

install: build
	install -Dm755 maly $(HOME)/.local/bin/maly

clean:
	rm -f maly
