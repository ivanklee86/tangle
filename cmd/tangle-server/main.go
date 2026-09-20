package main

import (
	"log"

	"github.com/knadh/koanf/v2"

	"github.com/ivanklee86/tangle/internal/tangle"
)

var k = koanf.New(".")

// Injected at build time.
var version = "dev"

func main() {
	config, err := tangle.LoadConfig(k, tangle.LoadConfigOptions{})
	if err != nil {
		log.Fatalf("Cannot load configuraiton. Error: %s", err)
	}

	tangle := tangle.New(config, version)
	tangle.Start()
}
