package main

import (
	"codex-files/servers/http"
	"log"
)

func main() {
	container, err := BuildContainer()
	if err != nil {
		log.Fatalf("Fatal error building container: %v", err)
	}

	if err := container.Invoke(func(srv *http.Server) error {
		return srv.Start()
	}); err != nil {
		log.Fatalf("Fatal error running application: %v", err)
	}
}
