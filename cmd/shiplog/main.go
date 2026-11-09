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
	"strings"
	"syscall"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/api"
	"github.com/junkerderprovinz/shiplog/internal/autoupdate"
	"github.com/junkerderprovinz/shiplog/internal/bannerlog"
	"github.com/junkerderprovinz/shiplog/internal/changelog"
	"github.com/junkerderprovinz/shiplog/internal/config"
	"github.com/junkerderprovinz/shiplog/internal/dockercli"
	"github.com/junkerderprovinz/shiplog/internal/engine"
	"github.com/junkerderprovinz/shiplog/internal/model"
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

	docker := dockercli.New(cfg.DockerSocket)
	// The logins in Docker's config.json are the ones Unraid's own update check
	// uses, so private images resolve for ShipLog too.
	dockerCfg := filepath.Join(cfg.DockerConfigDir, "config.json")
	reg := resolver.New().
		WithDockerHubAuth(cfg.DockerHubUser, cfg.DockerHubToken).
		WithGitHubToken(cfg.GithubToken).
		WithDockerConfig(dockerCfg)
	if n := reg.DockerLogins(); n > 0 {
		log.Printf("shiplog: %d registry login(s) from %s", n, dockerCfg)
	}
	eng := engine.New(
		docker,
		reg,
		changelog.Chain{changelog.NewFile(), changelog.New(cfg.GithubToken), changelog.Fallback{}},
		db,
		cfg.PollInterval,
	)

	eng.WithIgnoreUnmanaged(cfg.IgnoreUnmanaged)
	eng.WithCAFeed(cfg.DataDir)

	// Optional integrations are pinged once, so the log says whether they work.
	if ls := summarize.NewLlamaSwap(cfg.LlamaSwapURL, cfg.LlamaSwapModel); ls != nil {
		eng.WithSummarizer(ls)
		logSummarizerPing("llama-swap", cfg.LlamaSwapURL, cfg.LlamaSwapModel, ls.Ping)
	} else if o := summarize.New(cfg.OllamaURL, cfg.OllamaModel); o != nil {
		eng.WithSummarizer(o)
		logSummarizerPing("Ollama", cfg.OllamaURL, cfg.OllamaModel, o.Ping)
	}

	var sinks []notify.Sink
	if m := notify.New(cfg.MatrixHomeserver, cfg.MatrixToken, cfg.MatrixRoom); m != nil {
		sinks = append(sinks, m)
		whoCtx, cancelWho := context.WithTimeout(context.Background(), 10*time.Second)
		if werr := m.Whoami(whoCtx); werr != nil {
			log.Printf("shiplog: Matrix configured (%s) but not working: %v", cfg.MatrixHomeserver, werr)
		} else {
			log.Printf("shiplog: Matrix OK, notifications enabled (%s, room %s)", cfg.MatrixHomeserver, cfg.MatrixRoom)
		}
		cancelWho()
	}
	if a := notify.NewApprise(cfg.AppriseURL); a != nil {
		sinks = append(sinks, a)
		log.Printf("shiplog: Apprise notifications enabled (%s)", a.Where())
	} else if strings.TrimSpace(cfg.AppriseURL) != "" {
		log.Printf("shiplog: APPRISE_URL is not an http(s) URL; Apprise notifications off")
	}
	if u := notify.NewUnraid(cfg.UnraidNotify); u != nil {
		sinks = append(sinks, u)
		log.Printf("shiplog: native Unraid notifications enabled")
	} else if cfg.UnraidNotify {
		log.Printf("shiplog: Unraid notifications requested but the notify tool (%s) was not found; skipping (Unraid host only)", notify.UnraidScriptPath)
	}
	notifier := notify.NewFanout(sinks...)
	if notifier != nil {
		eng.WithNotifier(notifier)
	}

	upd := updater.Unraid{}
	autoUpdating := cfg.AutoUpdate.Enabled && upd.Supported()
	policy := autoUpdatePolicy(cfg.AutoUpdate)
	sched := autoupdate.Schedule{Mode: cfg.AutoUpdate.SchedMode, Time: cfg.AutoUpdate.SchedTime, Every: cfg.AutoUpdate.SchedEvery}
	// A "boot" run may be months away and a dry run applies nothing, so their
	// updates keep their notifications.
	if autoUpdating && !cfg.AutoUpdate.DryRun && sched.Recurs() {
		eng.WithAutoApply(func(st model.UpdateStatus) bool { return autoupdate.WillApply(st, policy) })
	}

	// The binary is PID 1 in the container.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go eng.Run(ctx)

	switch {
	case autoUpdating:
		exec := autoupdate.NewExecutor(db, upd).WithDocker(docker)
		go runAutoUpdate(ctx, cfg.AutoUpdate, sched, policy, exec, db, notifier)
		log.Printf("shiplog: auto-update enabled (level=%s, digest=%v, schedule=%s, dry-run=%v)",
			cfg.AutoUpdate.Level, cfg.AutoUpdate.Digest, cfg.AutoUpdate.SchedMode, cfg.AutoUpdate.DryRun)
	case cfg.AutoUpdate.Enabled:
		log.Printf("shiplog: auto-update is enabled but not supported here (needs the Unraid plugin / template dir); skipping")
	}

	srv := &http.Server{
		Handler:           api.New(db, db, eng, cfg.GithubToken).WithUpdateCheck(docker, notifier).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ln, addr, err := listen(cfg)
	if err != nil {
		log.Fatalf("shiplog: listen: %v", err)
	}
	bannerlog.Ready(os.Stdout, addr)

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

// listen opens the API's socket when one is configured, since the API has no
// login of its own, and the port on every interface otherwise, as the container
// image needs.
func listen(cfg config.Config) (net.Listener, string, error) {
	if cfg.Socket == "" {
		ln, err := net.Listen("tcp", ":"+cfg.Port)
		return ln, "http://0.0.0.0:" + cfg.Port, err
	}
	// A socket left by an unclean stop would fail the listen.
	_ = os.Remove(cfg.Socket)
	ln, err := net.Listen("unix", cfg.Socket)
	if err != nil {
		return nil, "", err
	}
	if err := os.Chmod(cfg.Socket, 0o660); err != nil {
		_ = ln.Close()
		return nil, "", err
	}
	return ln, "unix:" + cfg.Socket, nil
}

// runAutoUpdate checks every minute whether a run is due. The first check waits
// one tick, so the initial sweep has filled the store before a "boot" run.
func runAutoUpdate(ctx context.Context, cfg config.AutoUpdateConfig, sched autoupdate.Schedule, policy autoupdate.Policy, exec *autoupdate.Executor, st *store.Store, notifier *notify.Fanout) {
	if len(policy.ExcludeContainers) > 0 {
		log.Printf("shiplog: auto-update leaves these to manual updates: %s", strings.Join(policy.ExcludeContainers, ", "))
	}
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	var last time.Time
	// The stored last run keeps a restart from triggering an extra run; "boot"
	// is meant to run once per process start.
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
				if o.Blocked || o.Skipped || o.UpToDate {
					continue
				}
				if o.Output != "" {
					log.Printf("shiplog: auto-update of %s, the update script printed:\n%s", o.Name, o.Output)
				}
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
		// The summary also goes to the log, so a dry run shows its plan without any
		// notification channel. A run that only skipped excluded containers is
		// not worth a message every night.
		if text, html := autoupdate.RenderSummary(res); text != "" {
			log.Printf("shiplog: %s", text)
			if notifier != nil && res.Notable() {
				if nerr := notifier.SendMessage(ctx, text, html); nerr != nil {
					log.Printf("shiplog: auto-update notify: %v", nerr)
				}
			}
		}
	}
}

func autoUpdatePolicy(cfg config.AutoUpdateConfig) autoupdate.Policy {
	return autoupdate.Policy{
		Level:             autoupdate.ParseLevel(cfg.Level),
		Digest:            cfg.Digest,
		ExcludeWords:      autoupdate.ParseExcludeWords(cfg.ExcludeWords),
		ExcludeContainers: autoupdate.ParseExcludeContainers(cfg.ExcludeContainers),
	}
}

// lastRunKey holds the unix time of the last scheduled auto-update run.
const lastRunKey = "autoupdate_last_run"

func logSummarizerPing(name, url, model string, ping func(context.Context) error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := ping(ctx); err != nil {
		log.Printf("shiplog: %s configured (%s, model %s) but not working: %v", name, url, model, err)
		return
	}
	log.Printf("shiplog: %s OK, AI summaries enabled (%s, model %s)", name, url, model)
}
