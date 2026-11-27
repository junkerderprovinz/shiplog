package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// caUpdatesContainers runs include/ca.php against a plugin folder and the
// settings file of CA Auto Update's Docker part.
func caUpdatesContainers(t *testing.T, plugin, settings string) bool {
	t.Helper()
	var on bool
	runPHP(t, "CA", "ca.php", `echo json_encode($CA(`+phpString(plugin)+`, `+phpString(settings)+`));`, &on)
	return on
}

// caInstalled lays out CA Auto Update's folders and writes its Docker
// settings, unless settings is empty.
func caInstalled(t *testing.T, settings string) (plugin, file string) {
	t.Helper()
	dir := t.TempDir()
	plugin = filepath.Join(dir, "emhttp", "ca.update.applications")
	flash := filepath.Join(dir, "boot", "ca.update.applications")
	for _, d := range []string{plugin, flash} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	file = filepath.Join(flash, "DockerUpdateSettings.json")
	if settings != "" {
		if err := os.WriteFile(file, []byte(settings), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return plugin, file
}

func TestWithoutCAAutoUpdateNothingElseUpdatesContainers(t *testing.T) {
	dir := t.TempDir()
	if caUpdatesContainers(t, filepath.Join(dir, "emhttp"), filepath.Join(dir, "boot", "DockerUpdateSettings.json")) {
		t.Error("a server without CA Auto Update gets the warning")
	}
}

func TestCAAutoUpdateOnAScheduleUpdatesContainers(t *testing.T) {
	for _, freq := range []string{"Daily", "Weekly", "Monthly", "Custom"} {
		plugin, file := caInstalled(t, `{"cron":{"dockerCronFrequency":"`+freq+`"},"global":{"dockerUpdateAll":"yes"}}`)
		if !caUpdatesContainers(t, plugin, file) {
			t.Errorf("CA Auto Update updating containers %s gets no warning", freq)
		}
	}
}

func TestCAAutoUpdateWithItsDockerPartOffLeavesContainersAlone(t *testing.T) {
	plugin, file := caInstalled(t, `{"cron":{"dockerCronFrequency":"disabled"},"global":{"dockerUpdateAll":"yes"}}`)
	if caUpdatesContainers(t, plugin, file) {
		t.Error("CA Auto Update with its Docker part off gets the warning")
	}
}

func TestCAAutoUpdateNeverSetUpForDockerLeavesContainersAlone(t *testing.T) {
	plugin, file := caInstalled(t, "")
	if caUpdatesContainers(t, plugin, file) {
		t.Error("CA Auto Update without Docker settings gets the warning")
	}
}

func TestCAAutoUpdateSettingsThatCannotBeReadCountAsUpdating(t *testing.T) {
	for name, settings := range map[string]string{
		"broken JSON":   `{"cron":`,
		"no schedule":   `{"global":{"dockerUpdateAll":"yes"}}`,
		"not an object": `"disabled"`,
	} {
		plugin, file := caInstalled(t, settings)
		if !caUpdatesContainers(t, plugin, file) {
			t.Errorf("settings with %s get no warning", name)
		}
	}
	plugin, file := caInstalled(t, "")
	if err := os.Mkdir(file, 0o755); err != nil {
		t.Fatal(err)
	}
	if !caUpdatesContainers(t, plugin, file) {
		t.Error("a settings file that cannot be read gets no warning")
	}
}

func TestTheCAWarningFollowsCAAutoUpdatesDockerPart(t *testing.T) {
	page := read(t, "shiplog.page")
	if !strings.Contains(page, "$CA = include '/usr/local/emhttp/plugins/shiplog/include/ca.php';\n$caDocker = $CA();") {
		t.Error("shiplog.page does not ask ca.php whether CA Auto Update updates containers")
	}
	if !strings.Contains(page, "<?php if ($caDocker): ?>") {
		t.Error("the CA warning does not depend on ca.php")
	}
}
