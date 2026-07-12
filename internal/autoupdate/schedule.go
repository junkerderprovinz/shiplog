package autoupdate

import (
	"strconv"
	"strings"
	"time"
)

// Schedule is the auto-update cadence.
type Schedule struct {
	Mode  string // off | daily | boot | hours | days
	Time  string // "HH:MM" for daily (defaults to 04:00 on parse errors)
	Every int    // interval for hours|days (clamped to >= 1)
}

// Due reports whether a run is due at now, given the last run time (zero = never
// ran): off → never; boot → once when never run; hours/days → interval elapsed;
// daily → now is at/after today's HH:MM and the last run was before it.
func (s Schedule) Due(last, now time.Time) bool {
	switch s.Mode {
	case "boot":
		return last.IsZero()
	case "hours":
		return sinceElapsed(last, now, time.Duration(max1(s.Every))*time.Hour)
	case "days":
		return sinceElapsed(last, now, time.Duration(max1(s.Every))*24*time.Hour)
	case "daily":
		at := todayAt(now, s.Time)
		return !now.Before(at) && last.Before(at)
	default: // off / unknown
		return false
	}
}

func sinceElapsed(last, now time.Time, d time.Duration) bool {
	if last.IsZero() {
		return true
	}
	return now.Sub(last) >= d
}

// todayAt returns today's HH:MM in now's location; unparseable input → 04:00.
func todayAt(now time.Time, hhmm string) time.Time {
	h, m := 4, 0
	if parts := strings.SplitN(hhmm, ":", 2); len(parts) == 2 {
		if hv, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil && hv >= 0 && hv < 24 {
			h = hv
		}
		if mv, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil && mv >= 0 && mv < 60 {
			m = mv
		}
	}
	return time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}
