package main

import (
	"log"

	"noobular/internal/db"
	"noobular/internal/server"
	"noobular/internal/ui"
)

func main() {
	db := db.NewDb()
	defer db.Close()
	renderer := ui.NewRenderer(".")
	cfg := server.ParseServerConfig()
	srv := server.NewServer(db, renderer, cfg)
	log.Println("Listening on port", cfg.Port)
	if cfg.Env == server.Local {
		log.Fatal(srv.ListenAndServe())
	} else if cfg.Env == server.Production {
		log.Fatal(srv.ListenAndServeTLS(cfg.CertChainFilepath, cfg.PrivKeyFilepath))
	}
}
