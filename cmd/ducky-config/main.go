package main

import (
	"flag"
	"log"

	"github.com/joseferrao/ducky-drv/internal/ui"
)

func main() {
	startMinimized := flag.Bool("minimized", false, "start hidden in the system tray")
	flag.Parse()
	if err := ui.Run(ui.Options{StartMinimized: *startMinimized}); err != nil {
		log.Fatal(err)
	}
}
