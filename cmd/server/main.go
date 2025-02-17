package main

import (
	"log"
	"noobular/internal/db"
	"noobular/internal/server"
)

func main() {
	dbClient := db.NewDbClient()
	port := 8080
	server := server.NewServer(port, dbClient)
	log.Println("Listening on port", port)
	log.Fatal(server.ListenAndServe())
}
