package cmd

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Regan-Milne/obsideo-storage-provider/api"
	"github.com/Regan-Milne/obsideo-storage-provider/config"
	"github.com/Regan-Milne/obsideo-storage-provider/store"
	"github.com/Regan-Milne/obsideo-storage-provider/tokens"
)

// Start loads config, initialises storage, and runs the HTTP server.
func Start(cfgPath string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	st, err := store.New(cfg.Data.Path)
	if err != nil {
		return fmt.Errorf("init store: %w", err)
	}

	v, err := tokens.NewVerifier(cfg.Tokens.PublicKeyPath)
	if err != nil {
		return fmt.Errorf("load coordinator public key: %w", err)
	}

	srv := api.New(st, v)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	httpSrv := &http.Server{
		Addr:         addr,
		Handler:      srv.Handler(),
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	log.Printf("provider-clean listening on %s", addr)
	return httpSrv.ListenAndServe()
}
