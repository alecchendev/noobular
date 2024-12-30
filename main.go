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

func main() {
	if len(os.Args) != 1 && len(os.Args) != 5 {
		log.Fatal(`Usage: noobular [<auth> <course_id> <module_id> <filepath>]`)
	}

	envStr := os.Getenv("ENVIRONMENT")
	env := internal.Environment(envStr)
	envNotSet := env != internal.Local && env != internal.Production

	if len(os.Args) == 5 {
		if envNotSet {
			log.Println("No environment set: defaulting to production for upload")
			env = internal.Production
		}
		cfg := parseUploadConfig(env)
		uploadModule(cfg)
	} else {
		if envNotSet {
			log.Println("No environment set: defaulting to local for server")
			env = internal.Local
		}
		cfg := parseServerConfig(env)
		runServer(cfg)
	}
}

type uploadConfig struct {
	baseUrl  string
	auth     string
	courseId int64
	moduleId int64
	filepath string
}

func parseUploadConfig(env internal.Environment) uploadConfig {
	urlStr := "http://localhost:8080"
	if env == internal.Production {
		urlStr = "https://noobular.com"
	}

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

	return uploadConfig{urlStr, auth, courseId, moduleId, filepath}
}

func uploadModule(cfg uploadConfig) {
	session_token := http.Cookie{
		Name:     "session_token",
		Value:    cfg.auth,
		Expires:  time.Now().Add(1 * time.Minute),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
		Path:     "/",
	}
	client := client.NewClient(cfg.baseUrl, &session_token)

	data, err := os.ReadFile(cfg.filepath)
	if err != nil {
		log.Fatal(err)
	}
	resp, err := client.UploadModule(cfg.courseId, cfg.moduleId, string(data))
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		log.Fatal("Upload failed")
	}
	log.Println("Upload successful")
	return
}

type serverConfig struct {
	env               internal.Environment
	port              int
	jwtSecret         []byte
	certChainFilepath string
	privKeyFilepath   string
	webAuthn          *webauthn.WebAuthn
}

func parseServerConfig(env internal.Environment) serverConfig {
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

	urlStr := "http://localhost:8080"
	if env == internal.Production {
		urlStr = "https://noobular.com"
	}
	urlUrl, err := url.Parse(urlStr)
	if err != nil {
		log.Fatal(err)
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

	return serverConfig{env, 8080, jwtSecret, certChainFilepath, privKeyFilepath, webAuthn}
}

func runServer(cfg serverConfig) {
	dbClient := db.NewDbClient()
	defer dbClient.Close()
	renderer := internal.NewRenderer(".")
	server := internal.NewServer(dbClient, renderer, cfg.webAuthn, cfg.jwtSecret, cfg.port, cfg.env)
	fmt.Println("Listening on port", server.Addr)

	if cfg.env == internal.Local {
		log.Fatal(server.ListenAndServe())
	} else if cfg.env == internal.Production {
		log.Fatal(server.ListenAndServeTLS(cfg.certChainFilepath, cfg.privKeyFilepath))
	}
}
