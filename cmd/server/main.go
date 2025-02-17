package main

import (
	"log"
	"noobular/internal/db"
	"noobular/internal/server"
	"noobular/internal/ui"
)

func main() {
	dbClient := db.NewDbClient()
	defer dbClient.Close()
	renderer := ui.NewRenderer(".")
	port := 8080
	server := server.NewServer(port, dbClient, renderer, server.Local)
	log.Println("Listening on port", port)
	log.Fatal(server.ListenAndServe())
}
