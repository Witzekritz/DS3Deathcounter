package main

import (
	"log"

	"dsdeathcounter/internal/counter"
	"dsdeathcounter/internal/web"
)

func main() {
    // Create the counter service
    counterService := counter.NewService()

    // Start monitoring games
    counterService.Start()

    // Create and start the web server
    server := web.NewServer(counterService, "127.0.0.1:8080")
    log.Println("Starting web server on http://127.0.0.1:8080")
    if err := server.Start(); err != nil {
        log.Fatal(err)
    }
}