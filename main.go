package main

import (
	"crypto/rand"
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
	envStr := os.Getenv("ENVIRONMENT")
	env := internal.Environment(envStr)

	jwtSecretHex := os.Getenv("JWT_SECRET")
	if jwtSecretHex == "" {
		token := make([]byte, 32)
		rand.Read(token)
		log.Println("Example: set -x JWT_SECRET", hex.EncodeToString(token))
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
		RPDisplayName: "Noobular",        // Display Name for your site
		RPID:          urlUrl.Hostname(), // Generally the domain name for your site
		RPOrigins:     []string{urlStr},  // The origin URL for WebAuthn requests
	})
	if err != nil {
		log.Fatal(err)
	}

	certChainFilepath := os.Getenv("CERT_PATH")
	privKeyFilepath := os.Getenv("PRIV_KEY_PATH")

	port := 8080
	dbClient := db.NewDbClient()
	defer dbClient.Close()
	renderer := internal.NewRenderer(".")
	server := internal.NewServer(dbClient, renderer, webAuthn, jwtSecret, port, env)
	fmt.Println("Listening on port", server.Addr)

	if env == internal.Local {
		log.Fatal(server.ListenAndServe())
	} else if env == internal.Production {
		log.Fatal(server.ListenAndServeTLS(certChainFilepath, privKeyFilepath))
	} else {
		log.Fatal("ENVIRONMENT must be either 'local' or 'production'")
	}
}
