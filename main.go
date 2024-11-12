package main

import (
	"fmt"
	"internal"
	"log"
)

func main() {
	port := 8080
	dbClient := internal.NewDbClient()
	defer dbClient.Close()
	server := internal.NewServer(dbClient, port)
	fmt.Println("Listening on port", server.Addr)
	log.Fatal(server.ListenAndServe())
}
