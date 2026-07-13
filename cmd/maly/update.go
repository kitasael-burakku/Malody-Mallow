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
// reiniciar el servicio.
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
	fmt.Println(i18n.Tf("up.found", latest, version.Version))
	cmd, cleanup, err := update.InstallerCmd()
	if err != nil {
		return err
	}
	defer cleanup()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
