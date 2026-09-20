package autoupdate

import (
	"testing"
	"time"
)

func TestScheduleDue(t *testing.T) {
	base := time.Date(2026, 7, 12, 4, 0, 0, 0, time.Local)

	if (Schedule{Mode: "off"}).Due(time.Time{}, base) {
		t.Error("off is never due")
	}
	if !(Schedule{Mode: "boot"}).Due(time.Time{}, base) {
		t.Error("boot is due when never run")
	}
	if (Schedule{Mode: "boot"}).Due(base.Add(-time.Hour), base) {
		t.Error("boot is not due after the first run")
	}
	if !(Schedule{Mode: "hours", Every: 6}).Due(base.Add(-7*time.Hour), base) {
		t.Error("hours=6 is due after 7h")
	}
	if (Schedule{Mode: "hours", Every: 6}).Due(base.Add(-5*time.Hour), base) {
		t.Error("hours=6 is not due after 5h")
	}
	if !(Schedule{Mode: "days", Every: 2}).Due(base.Add(-49*time.Hour), base) {
		t.Error("days=2 is due after 49h")
	}
	if !(Schedule{Mode: "daily", Time: "04:00"}).Due(base.Add(-25*time.Hour), base) {
		t.Error("daily is due at 04:00 when last run was yesterday")
	}
	if (Schedule{Mode: "daily", Time: "05:00"}).Due(base.Add(-25*time.Hour), base) {
		t.Error("daily 05:00 is not due at 04:00 (before the time)")
	}
	if (Schedule{Mode: "daily", Time: "04:00"}).Due(base, base) {
		t.Error("daily is not due again the same day after already running at 04:00")
	}
	if !(Schedule{Mode: "daily", Time: "nonsense"}).Due(base.Add(-25*time.Hour), base) {
		t.Error("daily with bad time defaults to 04:00 and is due")
	}
	if !(Schedule{Mode: "hours", Every: 6}).Due(time.Time{}, base) {
		t.Error("hours is due on the first run (never run before)")
	}
	if !(Schedule{Mode: "days", Every: 2}).Due(time.Time{}, base) {
		t.Error("days is due on the first run (never run before)")
	}
	// An interval below 1 is clamped, so it does not fire on every tick.
	if (Schedule{Mode: "hours", Every: 0}).Due(base.Add(-30*time.Minute), base) {
		t.Error("hours=0 clamps to 1h and is not due 30min after the last run")
	}
	if (Schedule{Mode: "hours", Every: -5}).Due(base.Add(-30*time.Minute), base) {
		t.Error("hours=-5 clamps to 1h and is not due 30min after the last run")
	}
	if (Schedule{Mode: "days", Every: 0}).Due(base.Add(-2*time.Hour), base) {
		t.Error("days=0 clamps to 1 day and is not due 2h after the last run")
	}
	if !(Schedule{Mode: "daily", Time: "25:70"}).Due(base.Add(-25*time.Hour), base) {
		t.Error("daily 25:70 falls back to 04:00 and is due")
	}
	if (Schedule{Mode: "daily", Time: "25:70"}).Due(base, base) {
		t.Error("daily 25:70 falls back to 04:00 and is not due again the same day")
	}
}
