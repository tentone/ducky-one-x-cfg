package main

import (
	"flag"
	"log"

	"github.com/tentone/ducky-drv/internal/ui"
)

func main() {
	startMinimized := flag.Bool("minimized", false, "start hidden in the system tray")
	autoStart := flag.Bool("autostart", false, "start at login unless disabled in settings")
	flag.Parse()
	if err := ui.Run(ui.Options{StartMinimized: *startMinimized, AutoStart: *autoStart}); err != nil {
		log.Fatal(err)
	}
}
