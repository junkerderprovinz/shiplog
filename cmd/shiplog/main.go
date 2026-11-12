// Command shiplog runs the ShipLog engine: a read-only background poller that
// reports what changes between each running container image and the newest one.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	kitfortress "github.com/junkerderprovinz/parleyport/kit/fortress"
	kitnotify "github.com/junkerderprovinz/parleyport/kit/notify"
	"github.com/junkerderprovinz/parleyport/kit/status"

	"github.com/junkerderprovinz/shiplog/internal/api"
	"github.com/junkerderprovinz/shiplog/internal/autoupdate"
	"github.com/junkerderprovinz/shiplog/internal/backup"
	"github.com/junkerderprovinz/shiplog/internal/bannerlog"
	"github.com/junkerderprovinz/shiplog/internal/changelog"
	"github.com/junkerderprovinz/shiplog/internal/config"
	"github.com/junkerderprovinz/shiplog/internal/dockercli"
	"github.com/junkerderprovinz/shiplog/internal/engine"
	"github.com/junkerderprovinz/shiplog/internal/fortress"
	"github.com/junkerderprovinz/shiplog/internal/model"
	"github.com/junkerderprovinz/shiplog/internal/notify"
	"github.com/junkerderprovinz/shiplog/internal/resolver"
	"github.com/junkerderprovinz/shiplog/internal/store"
	"github.com/junkerderprovinz/shiplog/internal/summarize"
	"github.com/junkerderprovinz/shiplog/internal/templates"
	"github.com/junkerderprovinz/shiplog/internal/updater"
	"github.com/junkerderprovinz/shiplog/internal/updates"
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

	notifier := loadNotifier(cfg)
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

	run := &fortress.Run{}
	report := &fortress.Source{App: app(), Instance: instance(db), Store: db, Run: run, Now: time.Now}
	exec := autoupdate.NewExecutor(db, upd).WithDocker(docker).WithProgress(run.Updating)
	// The scheduled auto-update and the runs other apps ask for take turns.
	// Held until the executor is complete, it also makes a run asked for
	// while the fortress opens wait for its safeguard.
	updating := &sync.Mutex{}
	updating.Lock()
	managed := &updates.Managed{Store: db, Policy: policy,
		On: autoUpdating && !cfg.AutoUpdate.DryRun && cfg.AutoUpdate.SchedMode != "off"}
	actions := []kitfortress.Action{managed.Action()}
	if upd.Supported() {
		runs := &updates.Runs{
			Store: db, Exec: exec, Exclude: policy.ExcludeContainers, Context: ctx, Lock: updating,
			Began: run.Begin,
			Ended: func(res autoupdate.Result) { run.End(); keepOutcomes(db, res, time.Now()) },
			File:  cfg.RunsFile,
		}
		if err := runs.Restore(); err != nil {
			log.Printf("shiplog: could not read the update runs from %s, starting without them: %v", cfg.RunsFile, err)
		}
		actions = append(actions, runs.Action())
	}
	door, backups := openFortress(cfg, report, actions...)
	if door != nil {
		defer func() { _ = door.Close() }()
		go door.Watch(ctx)
		exec.WithSafeguard(&autoupdate.Safeguard{
			Backup:        backups,
			Images:        docker,
			Templates:     templates.Folder(templates.Dir),
			Guards:        db,
			WithoutBackup: autoupdate.ParseExcludeContainers(cfg.AutoUpdate.WithoutBackup),
		})
	}
	updating.Unlock()
	switch {
	case autoUpdating:
		go runAutoUpdate(ctx, cfg.AutoUpdate, sched, policy, exec, db, notifier, run, updating)
		log.Printf("shiplog: auto-update enabled (level=%s, digest=%v, schedule=%s, dry-run=%v)",
			cfg.AutoUpdate.Level, cfg.AutoUpdate.Digest, cfg.AutoUpdate.SchedMode, cfg.AutoUpdate.DryRun)
	case cfg.AutoUpdate.Enabled:
		log.Printf("shiplog: auto-update is enabled but not supported here (needs the Unraid plugin / template dir); skipping")
	}

	var msg api.Messenger
	if notifier != nil {
		msg = notifier
	}
	a := api.New(db, db, eng, cfg.GithubToken).
		WithUpdateCheck(docker, msg).
		WithFortress(report).
		WithGuards(db, func() bool { return backups != nil && backups.Available() })
	if door != nil && cfg.Socket != "" {
		a.WithDoor(door)
	}
	srv := a.Server()

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

// openFortress takes part in the fortress the plugin joined and returns the
// door to it with the way to its backup app, or nil without a place for a
// membership. The backup finds no app while ShipLog is out of the fortress,
// so the updates then go on as they did before it.
func openFortress(cfg config.Config, report *fortress.Source, actions ...kitfortress.Action) (*fortress.Door, *backup.Client) {
	if cfg.FortressDir == "" {
		return nil, nil
	}
	door := &fortress.Door{Dir: cfg.FortressDir, App: report.App, Src: report, Actions: actions, Device: report.Instance.Name}
	if err := door.Open(); err != nil {
		log.Printf("shiplog: not taking part in the fortress: %v", err)
	}
	if door.State().Joined {
		log.Printf("shiplog: in a fortress as %s; with a backup app on this server, every auto-update is backed up first", door.Device)
	}
	return door, &backup.Client{
		Fortress: backup.Kit{Fortress: door},
		Device:   door.Device,
		Timeout:  cfg.AutoUpdate.BackupTimeout,
		Poll:     10 * time.Second,
	}
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
func runAutoUpdate(ctx context.Context, cfg config.AutoUpdateConfig, sched autoupdate.Schedule, policy autoupdate.Policy, exec *autoupdate.Executor, st *store.Store, notifier *notify.Notifier, run *fortress.Run, updating sync.Locker) {
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
		updating.Lock()
		if !cfg.DryRun {
			run.Begin(now)
		}
		res := exec.Run(ctx, policy, cfg.DryRun)
		run.End()
		updating.Unlock()
		if !res.DryRun {
			keepOutcomes(st, res, now)
		}
		// The summary also goes to the log, so a dry run shows its plan without any
		// notification channel. A run that only skipped excluded containers is
		// not worth a message every night.
		if text, html := autoupdate.RenderSummary(res); text != "" {
			log.Printf("shiplog: %s", text)
			if res.Notable() {
				if nerr := notifier.Send(ctx, notify.Summary(text, html, res.Failed())); nerr != nil {
					log.Printf("shiplog: auto-update notify: %v", nerr)
				}
			}
		}
	}
}

// keepOutcomes logs every update a run tried, which the fortress's log and
// the Docker tab read back.
func keepOutcomes(st *store.Store, res autoupdate.Result, now time.Time) {
	for _, o := range res.Outcomes {
		if o.Blocked || o.Skipped || o.UpToDate {
			continue
		}
		if o.Output != "" {
			log.Printf("shiplog: update of %s, the update script printed:\n%s", o.Name, o.Output)
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

// version is set by plugin/pkg_build.sh.
var version = "dev"

func app() status.App {
	a := status.App{ID: "shiplog", Name: "ShipLog", Version: strings.TrimPrefix(version, "v")}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 7 {
				a.Commit = s.Value[:7]
			}
		}
	}
	return a
}

// instanceKey holds the id this ShipLog was given on its first start.
const instanceKey = "instance_id"

// instance names this ShipLog. The id is made once and kept in the store, so a
// reader can tell this server from another one with the same name.
func instance(db *store.Store) status.Instance {
	id, _ := db.GetMeta(instanceKey)
	if id == "" {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		id = hex.EncodeToString(b)
		if err := db.SetMeta(instanceKey, id); err != nil {
			log.Printf("shiplog: keep the instance id: %v", err)
		}
	}
	name, err := os.Hostname()
	if err != nil || name == "" {
		name = "ShipLog"
	}
	return status.Instance{ID: id, Name: name}
}

// loadNotifier returns nil when no channel is switched on or the channels
// cannot be read; ShipLog then runs without notifications.
func loadNotifier(cfg config.Config) *notify.Notifier {
	channels, migrated, err := notify.Load(cfg.NotifyFile, cfg.OldNotify)
	if err != nil {
		log.Printf("shiplog: notifications off: %v", err)
		return nil
	}
	if migrated {
		log.Printf("shiplog: notification settings moved to %s", cfg.NotifyFile)
	}
	var on, off []string
	for _, c := range channels {
		if c.Enabled {
			on = append(on, c.ID)
		} else {
			off = append(off, c.ID)
		}
	}
	if len(off) > 0 {
		log.Printf("shiplog: notification channels switched off: %s", strings.Join(off, ", "))
	}
	n, err := notify.New(channels, kitnotify.Options{})
	if err != nil {
		log.Printf("shiplog: notifications off: %v", err)
		return nil
	}
	if n != nil {
		log.Printf("shiplog: notifications on: %s", strings.Join(on, ", "))
	}
	return n
}
