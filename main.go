package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"noobular/internal"
	"noobular/internal/db"
)

func main() {
	jwtSecretHex := os.Getenv("JWT_SECRET")
	if jwtSecretHex == "" {
		log.Fatal("JWT_SECRET must be set")
	}
	jwtSecret, err := hex.DecodeString(jwtSecretHex)
	if err != nil {
		log.Fatal("JWT_SECRET must be a valid hex string")
	}
	port := 8080
	dbClient := db.NewDbClient()
	defer dbClient.Close()
	server := internal.NewServer(dbClient, jwtSecret, port)
	fmt.Println("Listening on port", server.Addr)
	log.Fatal(server.ListenAndServe())
}
