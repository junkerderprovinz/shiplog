package notify

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	kit "github.com/junkerderprovinz/parleyport/kit/notify"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// cfgEnv reads a shiplog.cfg the way rc.shiplog does before it starts the
// engine: every KEY="value" line becomes a variable.
func cfgEnv(t *testing.T, name string) map[string]string {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	env := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if ok {
			env[k] = strings.Trim(v, `"`)
		}
	}
	return env
}

func TestLoadTurnsTheOldSettingsIntoChannels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notify.json")
	got, migrated, err := Load(path, cfgEnv(t, "shiplog.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	if !migrated {
		t.Error("migrated = false for a first start")
	}
	want := []kit.Channel{
		{ID: "matrix", Kind: kit.KindMatrix, Enabled: true, On: kit.OnAlways, Matrix: &kit.Matrix{
			Homeserver: "https://matrix.example.org", Room: "#updates:example.org", Token: "syt_c2hpcGxvZw_NotARealToken_0a1b2c"}},
		{ID: "apprise", Kind: kit.KindApprise, Enabled: true, On: kit.OnAlways, Apprise: &kit.Apprise{URL: "http://apprise.lan:8000/notify/shiplog"}},
		{ID: "unraid", Kind: kit.KindUnraid, Enabled: true, On: kit.OnAlways},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d channels, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Errorf("channel %d:\n got %+v\nwant %+v", i, got[i], want[i])
		}
	}

	again, migrated, err := Load(path, nil)
	if err != nil || migrated || !reflect.DeepEqual(again, got) {
		t.Errorf("second start = %+v, migrated %v, err %v; want the saved channels", again, migrated, err)
	}
}

func TestLoadBringsAHalfFilledSettingOverSwitchedOff(t *testing.T) {
	got, _, err := Load(filepath.Join(t.TempDir(), "notify.json"), cfgEnv(t, "shiplog-half.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	want := []kit.Channel{
		{ID: "matrix", Kind: kit.KindMatrix, Enabled: false, On: kit.OnAlways, Matrix: &kit.Matrix{
			Homeserver: "https://matrix.example.org", Room: "!AbCdEfGhIjKlMnOp:example.org"}},
		{ID: "apprise", Kind: kit.KindApprise, Enabled: false, On: kit.OnAlways, Apprise: &kit.Apprise{URL: "apprise.lan:8000/notify/shiplog"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestLoadSavesAnEmptyListWithoutOldSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notify.json")
	if _, _, err := Load(path, map[string]string{"UNRAID_NOTIFY": "false"}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); strings.TrimSpace(string(b)) != "[]" {
		t.Errorf("notify.json = %q, want []", b)
	}
}

func TestLoadWithoutAFileReadsTheEnvironmentEveryStart(t *testing.T) {
	got, migrated, err := Load("", map[string]string{"APPRISE_URL": "http://apprise.lan:8000/notify/shiplog"})
	if err != nil || migrated {
		t.Fatalf("migrated %v, err %v; want the environment read as it is", migrated, err)
	}
	if len(got) != 1 || got[0].Apprise == nil || got[0].Apprise.URL != "http://apprise.lan:8000/notify/shiplog" || !got[0].Enabled {
		t.Errorf("channels = %+v", got)
	}
}

func TestLoadLeavesAFileItCannotReadAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notify.json")
	if err := os.WriteFile(path, []byte("[{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(path, map[string]string{"UNRAID_NOTIFY": "true"}); err == nil {
		t.Error("a broken notify.json loads without an error")
	}
	if b, _ := os.ReadFile(path); string(b) != "[{" {
		t.Errorf("notify.json was overwritten with %q", b)
	}
}

type runner struct{ calls [][]string }

func (r *runner) Run(_ context.Context, name string, args ...string) error {
	r.calls = append(r.calls, append([]string{name}, args...))
	return nil
}

func unraidNotifier(t *testing.T) (*Notifier, *runner) {
	t.Helper()
	r := &runner{}
	n, err := New([]kit.Channel{{ID: "unraid", Kind: kit.KindUnraid, Enabled: true, On: kit.OnAlways}}, kit.Options{Unraid: r})
	if err != nil || n == nil {
		t.Fatalf("New = %v, %v", n, err)
	}
	return n, r
}

func unraidArgs(t *testing.T, st model.UpdateStatus) []string {
	t.Helper()
	n, r := unraidNotifier(t)
	if err := n.Notify(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	if len(r.calls) != 1 {
		t.Fatalf("calls = %v", r.calls)
	}
	return r.calls[0]
}

func TestAnUpdateReachesUnraidAsBefore(t *testing.T) {
	got := unraidArgs(t, model.UpdateStatus{
		Container: model.Container{Name: "immich", Tag: "1.0", Source: "https://github.com/x/immich"},
		NewestTag: "1.2", Risk: model.RiskMedium,
	})
	want := []string{kit.UnraidNotifyScript, "-e", "ShipLog", "-s", "immich: update available",
		"-d", "1.0 → 1.2 (risk: MEDIUM)", "-i", "normal", "-l", "https://github.com/x/immich"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("args\n got %q\nwant %q", got, want)
	}
}

func argValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func TestAnUpdateNamesTheVulnerabilitiesItCloses(t *testing.T) {
	st := model.UpdateStatus{
		Container: model.Container{Name: "nginx", Tag: "1.27"}, NewestTag: "1.28", Kind: model.KindMinor, Risk: model.RiskHigh,
		Vulns: &model.Vulns{Scope: model.VulnScopeImage, Critical: 2, High: 4, NewestKnown: true, FixedCritical: 2, FixedHigh: 1},
	}
	want := "1.27 → 1.28 (risk: HIGH) · 2 critical and 4 high known vulnerabilities, the update closes 2 critical and 1 high"
	if got := argValue(unraidArgs(t, st), "-d"); got != want {
		t.Errorf("description = %q, want %q", got, want)
	}
	if e := update(st); !strings.Contains(e.HTML, "the update closes 2 critical and 1 high") {
		t.Errorf("html = %q", e.HTML)
	}
}

func TestAnUpdateKeepsTheVulnerabilitiesWithoutAFixApart(t *testing.T) {
	st := model.UpdateStatus{
		Container: model.Container{Name: "bombvault", Tag: "1.4"}, NewestTag: "1.5", Kind: model.KindMinor, Risk: model.RiskMedium,
		Vulns: &model.Vulns{Scope: model.VulnScopeImage, Critical: 10, High: 71, NoFixCritical: 9, NoFixHigh: 63, NewestKnown: true},
	}
	want := "1.4 → 1.5 (risk: MEDIUM) · 1 critical and 8 high known vulnerabilities with a fix available, the update closes none of them, plus 9 critical and 63 high with no fix yet"
	if got := argValue(unraidArgs(t, st), "-d"); got != want {
		t.Errorf("description = %q, want %q", got, want)
	}
}

func TestTheRiskSetsUnraidsImportance(t *testing.T) {
	for risk, want := range map[model.RiskLevel]string{
		model.RiskLow: "normal", model.RiskMedium: "normal", model.RiskHigh: "warning", model.RiskCritical: "alert",
	} {
		st := model.UpdateStatus{Container: model.Container{Name: "app", Tag: "1"}, NewestTag: "2", Risk: risk}
		if got := argValue(unraidArgs(t, st), "-i"); got != want {
			t.Errorf("risk %s: importance %q, want %q", risk, got, want)
		}
	}
}

func TestARollingMoveWithoutAKnownVersionNamesNoTarget(t *testing.T) {
	st := model.UpdateStatus{
		Container: model.Container{Name: "featherdrop", Tag: "latest"},
		NewestTag: "latest", Risk: model.RiskLow,
		Changelog: &model.Changelog{FromTag: "latest", ToTag: "latest", Recent: true, Entries: []model.ReleaseEntry{{Tag: "v6.1.0"}}},
	}
	if got, want := argValue(unraidArgs(t, st), "-d"), "New image for latest (risk: LOW)"; got != want {
		t.Errorf("description = %q, want %q", got, want)
	}
}

func TestAnUnmaintainedAppIsAWarning(t *testing.T) {
	args := unraidArgs(t, model.UpdateStatus{Container: model.Container{Name: "opencloud"}, Unmaintained: true,
		UnmaintainedReason: "Removed from Community Applications"})
	if argValue(args, "-s") != "opencloud: no longer maintained" || argValue(args, "-d") != "Removed from Community Applications" ||
		argValue(args, "-i") != "warning" {
		t.Errorf("args = %q", args)
	}
}

func TestAnUpdateReachesMatrixWithItsHTML(t *testing.T) {
	var body map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/directory/room/"):
			_ = json.NewEncoder(w).Encode(map[string]string{"room_id": "!abc:hs"})
		case strings.Contains(r.URL.Path, "/rooms/!abc:hs/send/"):
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &body)
			_ = json.NewEncoder(w).Encode(map[string]string{"event_id": "$x"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	n, err := New([]kit.Channel{{ID: "matrix", Kind: kit.KindMatrix, Enabled: true, On: kit.OnAlways,
		Matrix: &kit.Matrix{Homeserver: srv.URL, Room: "#room:hs", Token: "tok"}}}, kit.Options{HTTP: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	st := model.UpdateStatus{
		Container: model.Container{Name: "planka", Tag: "latest"}, NewestTag: "latest", Risk: model.RiskMedium,
		Changelog: &model.Changelog{FromTag: "2.2.1", ToTag: "2.3.0", URL: "https://github.com/plankanban/planka/releases", Entries: []model.ReleaseEntry{{Tag: "v2.3.0"}}},
	}
	if err := n.Notify(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	if want := "🚢 <b>ShipLog</b> · <b>planka</b>: update <code>latest</code> → <code>v2.3.0</code> (risk: MEDIUM)" +
		`<br/><a href="https://github.com/plankanban/planka">https://github.com/plankanban/planka</a>`; body["formatted_body"] != want {
		t.Errorf("formatted_body = %q\nwant %q", body["formatted_body"], want)
	}
	if !strings.Contains(body["body"], "planka: update available") || !strings.Contains(body["body"], "latest → v2.3.0 (risk: MEDIUM)") {
		t.Errorf("body = %q", body["body"])
	}
}

func TestNewSendsNothingWithoutAChannelSwitchedOn(t *testing.T) {
	n, err := New([]kit.Channel{{ID: "matrix", Kind: kit.KindMatrix, Enabled: false, Matrix: &kit.Matrix{Homeserver: "https://hs"}}}, kit.Options{})
	if err != nil || n != nil {
		t.Fatalf("New = %v, %v; want no notifier", n, err)
	}
	if err := n.Notify(context.Background(), model.UpdateStatus{}); err != nil {
		t.Errorf("a missing notifier fails: %v", err)
	}
}

func TestAFailedRunIsAFailure(t *testing.T) {
	if e := Summary("2 updated, 1 failed", "", true); e.Severity != kit.SeverityFailure || e.Kind != "autoupdate.run" || e.Body != "2 updated, 1 failed" {
		t.Errorf("failed run = %+v", e)
	}
	if e := Summary("2 updated", "", false); e.Severity != kit.SeveritySuccess {
		t.Errorf("clean run = %+v", e)
	}
	if e := NotApplied("Nextcloud"); e.Severity != kit.SeverityFailure || !strings.Contains(e.Title, "Nextcloud") {
		t.Errorf("update that did not apply = %+v", e)
	}
}

func TestSendNamesTheChannelThatFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer srv.Close()
	n, err := New([]kit.Channel{{ID: "apprise", Kind: kit.KindApprise, Enabled: true, On: kit.OnAlways, Attempts: 1,
		Apprise: &kit.Apprise{URL: srv.URL + "/notify/secretkey"}}}, kit.Options{HTTP: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	err = n.Send(context.Background(), NotApplied("x"))
	if err == nil || !strings.Contains(err.Error(), "apprise") || strings.Contains(err.Error(), "secretkey") {
		t.Errorf("err = %v, want the channel named and the key left out", err)
	}
}

func digestFixture() []model.UpdateStatus {
	return []model.UpdateStatus{
		{Container: model.Container{Name: "shelfmark", Tag: "1.4.0"}, NewestTag: "1.4.1", Risk: model.RiskLow},
		{Container: model.Container{Name: "immich_redis", Tag: "latest"}, NewestTag: "latest", Risk: model.RiskLow},
		{Container: model.Container{Name: "OpenBot", Tag: "2.1", Source: "https://github.com/x/openbot"}, NewestTag: "2.2", Risk: model.RiskMedium},
	}
}

func TestADigestListsTheRiskiestUpdateFirst(t *testing.T) {
	e := Digest(digestFixture())
	if e.Kind != "update.digest" || e.Title != "3 updates available" || e.Severity != kit.SeverityInfo {
		t.Fatalf("event = %+v", e)
	}
	want := "• OpenBot: 2.1 → 2.2 (MEDIUM)\n• immich_redis: new image for latest (LOW)\n• shelfmark: 1.4.0 → 1.4.1 (LOW)"
	if e.Body != want {
		t.Errorf("body\n got %q\nwant %q", e.Body, want)
	}
	if !strings.HasPrefix(e.HTML, "🚢 <b>ShipLog</b> · 3 updates available<br/>• <b>OpenBot</b>") ||
		!strings.Contains(e.HTML, `<a href="https://github.com/x/openbot">changelog</a>`) {
		t.Errorf("html = %q", e.HTML)
	}
}

// A channel that sends only problems still hears of a digest that holds one.
func TestADigestTakesTheSeverityOfItsRiskiestUpdate(t *testing.T) {
	sts := append(digestFixture(), model.UpdateStatus{Container: model.Container{Name: "db", Tag: "15"}, NewestTag: "16", Risk: model.RiskCritical})
	if e := Digest(sts); e.Severity != kit.SeverityCritical || !strings.HasPrefix(e.Body, "• db: 15 → 16 (CRITICAL)") {
		t.Errorf("severity %s, body %q", e.Severity, e.Body)
	}
}

func TestADigestReachesUnraidAsOneNotice(t *testing.T) {
	n, r := unraidNotifier(t)
	if err := n.NotifyDigest(context.Background(), digestFixture()); err != nil {
		t.Fatal(err)
	}
	if len(r.calls) != 1 || argValue(r.calls[0], "-s") != "3 updates available" {
		t.Errorf("calls = %q", r.calls)
	}
}

func TestADigestOfOneUpdateIsTheUsualNotice(t *testing.T) {
	n, r := unraidNotifier(t)
	if err := n.NotifyDigest(context.Background(), digestFixture()[:1]); err != nil {
		t.Fatal(err)
	}
	if len(r.calls) != 1 || argValue(r.calls[0], "-s") != "shelfmark: update available" {
		t.Errorf("calls = %q", r.calls)
	}
}
