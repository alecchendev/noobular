package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"noobular/internal"
	"noobular/internal/client"
	"noobular/internal/db"

	"github.com/go-webauthn/webauthn/webauthn"
)

const usage = `Usage: noobular [<auth> <course_id> <module_id> <filepath>]`

func main() {
	if len(os.Args) != 1 && len(os.Args) != 5 {
		log.Fatal(usage)
	}

	envStr := os.Getenv("ENVIRONMENT")
	env := internal.Environment(envStr)

	urlStr := "http://localhost:8080"
	if env == internal.Production {
		urlStr = "https://noobular.com"
	}
	urlUrl, err := url.Parse(urlStr)
	if err != nil {
		log.Fatal("PUBLIC_URL must be a valid URL")
	}

	if len(os.Args) == 5 {
		auth := os.Args[1]
		courseIdInt, err := strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatal("course_id must be an integer")
		}
		courseId := int64(courseIdInt)
		moduleIdInt, err := strconv.Atoi(os.Args[3])
		if err != nil {
			log.Fatal("module_id must be an integer")
		}
		moduleId := int64(moduleIdInt)
		filepath := os.Args[4]

		session_token := http.Cookie{
			Name:     "session_token",
			Value:    auth,
			Expires:  time.Now().Add(1 * time.Minute),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   true,
			Path:     "/",
		}
		client := client.NewClient(urlStr, &session_token)

		data, err := os.ReadFile(filepath)
		if err != nil {
			log.Fatal(err)
		}
		resp, err := client.UploadModule(courseId, moduleId, string(data))
		if err != nil {
			log.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			log.Fatal("Upload failed")
		}
		log.Println("Upload successful")
		return
	}

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
