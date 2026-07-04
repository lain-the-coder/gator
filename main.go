package main

import (
	"fmt"
	"log"

	"github.com/lain-the-coder/gator/internal/config"
)

func main() {
	cfg, err := config.Read()
	if err != nil {
		log.Fatalf("error reading config file: %v", err)
	}
	err = cfg.SetUser("lain")
	if err != nil {
		log.Fatalf("error writing username to config file: %v", err)
	}
	cfg, err = config.Read()
	if err != nil {
		log.Fatalf("error reading config file: %v", err)
	}
	fmt.Printf("Database URL written in config file is: %s\n", cfg.DbURL)
	fmt.Printf("Current Username written in config file is: %s\n", cfg.CurrentUsername)
}
