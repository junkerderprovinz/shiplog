// Command shiplog runs the ShipLog engine: a read-only background poller that
// reports what changes between each running container image and the newest one.
package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/api"
	"github.com/junkerderprovinz/shiplog/internal/bannerlog"
	"github.com/junkerderprovinz/shiplog/internal/changelog"
	"github.com/junkerderprovinz/shiplog/internal/config"
	"github.com/junkerderprovinz/shiplog/internal/dockercli"
	"github.com/junkerderprovinz/shiplog/internal/engine"
	"github.com/junkerderprovinz/shiplog/internal/resolver"
	"github.com/junkerderprovinz/shiplog/internal/store"
	"github.com/junkerderprovinz/shiplog/internal/summarize"
)

func main() {
	bannerlog.Init(os.Stdout)
	cfg := config.Load()

	db, err := store.Open(filepath.Join(cfg.DataDir, "shiplog.db"))
	if err != nil {
		log.Fatalf("shiplog: open store: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Collaborators. The changelog chain tries GitHub (via the OCI source label)
	// then always-succeeds with the fallback.
	eng := engine.New(
		dockercli.New(cfg.DockerSocket),
		resolver.New(),
		changelog.Chain{changelog.New(cfg.GithubToken), changelog.Fallback{}},
		db,
		cfg.PollInterval,
	)

	// Optional AI summaries (Ollama). nil when unconfigured → silently skipped.
	// Ping once at startup so the log says plainly whether summaries will work.
	if sum := summarize.New(cfg.OllamaURL, cfg.OllamaModel); sum != nil {
		eng.WithSummarizer(sum)
		pingCtx, cancelPing := context.WithTimeout(context.Background(), 10*time.Second)
		if perr := sum.Ping(pingCtx); perr != nil {
			log.Printf("shiplog: Ollama configured (%s, model %s) but NOT working: %v", cfg.OllamaURL, cfg.OllamaModel, perr)
		} else {
			log.Printf("shiplog: Ollama OK — AI summaries enabled (%s, model %s)", cfg.OllamaURL, cfg.OllamaModel)
		}
		cancelPing()
	}

	// Cancel everything on SIGTERM/SIGINT (the binary is PID 1 in the container).
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go eng.Run(ctx)

	srv := &http.Server{
		Handler:           api.New(db, eng).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		log.Fatalf("shiplog: listen: %v", err)
	}
	bannerlog.Ready(os.Stdout, "0.0.0.0:"+cfg.Port)

	go func() {
		if serr := srv.Serve(ln); serr != nil && serr != http.ErrServerClosed {
			log.Fatalf("shiplog: serve: %v", serr)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
