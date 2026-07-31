package main

import (
	"fmt"
	"os"

	"maly/internal/i18n"
	"maly/internal/update"
	"maly/internal/version"
)

// runUpdate busca un release nuevo (tags del repo vía git) y, si lo hay,
// delega en mallow-install.sh --update: él recompila, reinstala y recuerda
// reiniciar el servicio. Con un binario de un gestor de paquetes
// (version.Packaged()), en cambio, ni toca el instalador: mallow-install.sh
// nunca pisa la ruta del paquete, así que correrlo dejaría una segunda
// copia por detrás de pacman — mejor remitir al gestor.
func runUpdate([]string) error {
	latest, err := update.Latest()
	if err != nil {
		return err
	}
	update.SaveCache(latest)
	if !update.Newer(latest, version.Version) {
		fmt.Println(i18n.Tf("up.current", version.Version))
		return nil
	}
	if version.Packaged() {
		fmt.Println(i18n.Tf("up.found_packaged", latest, version.Version))
		return nil
	}
	fmt.Println(i18n.Tf("up.found", latest, version.Version))
	cmd, cleanup, err := update.InstallerCmd(latest)
	if err != nil {
		return err
	}
	defer cleanup()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
