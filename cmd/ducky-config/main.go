package main

import (
	"log"

	"github.com/joseferrao/ducky-drv/internal/ui"
)

func main() {
	if err := ui.Run(); err != nil {
		log.Fatal(err)
	}
}
