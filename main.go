package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"net/url"
	"os"

	"noobular/internal"
	"noobular/internal/db"

	"github.com/go-webauthn/webauthn/webauthn"
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
	urlStr := os.Getenv("PUBLIC_URL")
	if urlStr == "" {
		urlStr = "http://localhost:8080"
	}
	urlUrl, err := url.Parse(urlStr)
	if err != nil {
		log.Fatal("PUBLIC_URL must be a valid URL")
	}
	webAuthn, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "WebAuthn Demo",   // Display Name for your site
		RPID:          urlUrl.Hostname(), // Generally the domain name for your site
		RPOrigins:     []string{urlStr},  // The origin URL for WebAuthn requests
	})
	if err != nil {
		log.Fatal(err)
	}

	port := 8080
	dbClient := db.NewDbClient()
	defer dbClient.Close()
	renderer := internal.NewRenderer(".")
	server := internal.NewServer(dbClient, renderer, webAuthn, jwtSecret, port)
	fmt.Println("Listening on port", server.Addr)
	log.Fatal(server.ListenAndServe())
}
