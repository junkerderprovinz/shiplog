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
	"strconv"
	"syscall"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/api"
	"github.com/junkerderprovinz/shiplog/internal/autoupdate"
	"github.com/junkerderprovinz/shiplog/internal/bannerlog"
	"github.com/junkerderprovinz/shiplog/internal/changelog"
	"github.com/junkerderprovinz/shiplog/internal/config"
	"github.com/junkerderprovinz/shiplog/internal/dockercli"
	"github.com/junkerderprovinz/shiplog/internal/engine"
	"github.com/junkerderprovinz/shiplog/internal/notify"
	"github.com/junkerderprovinz/shiplog/internal/resolver"
	"github.com/junkerderprovinz/shiplog/internal/store"
	"github.com/junkerderprovinz/shiplog/internal/summarize"
	"github.com/junkerderprovinz/shiplog/internal/updater"
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
		resolver.New().
			WithDockerHubAuth(cfg.DockerHubUser, cfg.DockerHubToken).
			WithGitHubToken(cfg.GithubToken),
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

	// Optional Matrix notifications. nil when unconfigured. Whoami at startup so
	// the log says plainly whether notifications will work.
	notifier := notify.New(cfg.MatrixHomeserver, cfg.MatrixToken, cfg.MatrixRoom)
	if notifier != nil {
		eng.WithNotifier(notifier)
		whoCtx, cancelWho := context.WithTimeout(context.Background(), 10*time.Second)
		if werr := notifier.Whoami(whoCtx); werr != nil {
			log.Printf("shiplog: Matrix configured (%s) but NOT working: %v", cfg.MatrixHomeserver, werr)
		} else {
			log.Printf("shiplog: Matrix OK — notifications enabled (%s, room %s)", cfg.MatrixHomeserver, cfg.MatrixRoom)
		}
		cancelWho()
	}

	// Cancel everything on SIGTERM/SIGINT (the binary is PID 1 in the container).
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go eng.Run(ctx)

	// Scheduled SemVer-gated auto-update (Unraid plugin only; off unless enabled).
	// It reads the engine's already-classified statuses from the store and applies
	// only what the policy allows, on the configured cadence.
	if cfg.AutoUpdate.Enabled {
		upd := updater.Unraid{}
		if !upd.Supported() {
			log.Printf("shiplog: auto-update is enabled but not supported here (needs the Unraid plugin / template dir) — skipping")
		} else {
			exec := autoupdate.NewExecutor(db, upd)
			go runAutoUpdate(ctx, cfg.AutoUpdate, exec, db, notifier)
			log.Printf("shiplog: auto-update ON (level=%s, digest=%v, schedule=%s, dry-run=%v)",
				cfg.AutoUpdate.Level, cfg.AutoUpdate.Digest, cfg.AutoUpdate.SchedMode, cfg.AutoUpdate.DryRun)
		}
	}

	srv := &http.Server{
		Handler:           api.New(db, db, eng).Handler(),
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

// runAutoUpdate drives the scheduled auto-update loop: every minute it asks the
// schedule whether a run is due, and when it is, applies the policy over the
// store's classified statuses, records each real action, and sends a run
// summary. It waits one tick before the first check so the initial sweep has
// populated the store (matters for the "boot" schedule).
func runAutoUpdate(ctx context.Context, cfg config.AutoUpdateConfig, exec *autoupdate.Executor, st *store.Store, notifier *notify.Matrix) {
	sched := autoupdate.Schedule{Mode: cfg.SchedMode, Time: cfg.SchedTime, Every: cfg.SchedEvery}
	policy := autoupdate.Policy{Level: autoupdate.ParseLevel(cfg.Level), Digest: cfg.Digest}
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	var last time.Time
	// Rehydrate the last run time across restarts so a reboot does not re-trigger
	// an off-schedule run for the daily/hours/days cadences. "boot" is left zero on
	// purpose — it is meant to fire once per process start.
	if cfg.SchedMode != "boot" {
		if v, err := st.GetMeta(lastRunKey); err == nil && v != "" {
			if u, perr := strconv.ParseInt(v, 10, 64); perr == nil {
				last = time.Unix(u, 0)
			}
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		now := time.Now()
		if !sched.Due(last, now) {
			continue
		}
		last = now
		_ = st.SetMeta(lastRunKey, strconv.FormatInt(now.Unix(), 10))
		res := exec.Run(ctx, policy, cfg.DryRun)
		if !res.DryRun {
			for _, o := range res.Outcomes {
				errStr := ""
				if o.Err != nil {
					errStr = o.Err.Error()
				}
				_ = st.LogAutoUpdate(store.AutoUpdateRecord{
					Name: o.Name, FromVer: o.From, ToVer: o.To, Level: o.Level,
					Success: o.Err == nil, Err: errStr, At: now.Unix(),
				})
			}
		}
		// Always log the itemised run summary so the plan is visible even without
		// Matrix (matters for dry-run — the whole point is to SEE what would update);
		// then also push it to Matrix when configured. Empty when nothing was eligible.
		if text, html := autoupdate.RenderSummary(res); text != "" {
			log.Printf("shiplog: %s", text)
			if notifier != nil {
				if nerr := notifier.SendMessage(ctx, text, html); nerr != nil {
					log.Printf("shiplog: auto-update notify: %v", nerr)
				}
			}
		}
	}
}

// lastRunKey is the meta store key holding the unix time of the last scheduled
// auto-update run, so the cadence survives a daemon restart.
const lastRunKey = "autoupdate_last_run"
