package main

import (
	"log"

	"waveguide/internal/api"
	"waveguide/internal/profile"
)

// listenAddr is the fixed port the service binds to.
const listenAddr = ":8080"

func main() {
	store := profile.NewStore()
	profile.SeedBuiltin(store)

	srv := api.NewServer(store)
	log.Printf("waveguide mode service listening on %s", listenAddr)
	if err := srv.Run(listenAddr); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
