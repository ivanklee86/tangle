package main

import (
	"github.com/gofiber/fiber/v2/log"
	"github.com/knadh/koanf/v2"

	"github.com/ivanklee86/tangle/internal/tangle"
)

var k = koanf.New(".")

func main() {
	config, err := tangle.LoadConfig(k, tangle.LoadConfigOptions{})
	if err != nil {
		log.Fatalf("Cannot load configuraiton. Error: %s", err)
	}

	tangle := tangle.New(config)
	tangle.Start()
}
