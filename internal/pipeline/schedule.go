package pipeline

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Stage is one step of the pipeline and how often it should run.
type Stage struct {
	Name  string
	Every time.Duration // 0 disables the stage
	// At is a time of day ("07:30") for stages that run once daily. When set it
	// takes precedence over Every.
	At  string
	Run func(ctx context.Context) (string, error)

	last time.Time
}

// Due reports whether a stage should run at the given moment.
//
// A stage that has never run is due immediately: starting the daemon should
// bring the feed up to date, not wait an hour to begin. A daily stage is due
// once its time of day has passed and it has not already run since.
func (s *Stage) Due(now time.Time) bool {
	if s.At != "" {
		target, ok := timeOfDay(now, s.At)
		if !ok {
			return false // an unparseable time is an off switch, not a crash
		}
		if now.Before(target) {
			return false
		}
		return s.last.Before(target)
	}
	if s.Every <= 0 {
		return false
	}
	if s.last.IsZero() {
		return true
	}
	return now.Sub(s.last) >= s.Every
}

// Ran records when a stage last went, due or not.
func (s *Stage) Ran(at time.Time) { s.last = at }

// LastRun is when the stage last went, zero if never.
func (s *Stage) LastRun() time.Time { return s.last }

// timeOfDay resolves "07:30" against the day `now` falls on, in local time —
// the reader's morning, not UTC's.
func timeOfDay(now time.Time, hhmm string) (time.Time, bool) {
	hour, minute, ok := parseHHMM(hhmm)
	if !ok {
		return time.Time{}, false
	}
	return time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location()), true
}

func parseHHMM(v string) (hour, minute int, ok bool) {
	h, m, found := strings.Cut(strings.TrimSpace(v), ":")
	if !found {
		return 0, 0, false
	}
	hour, err := strconv.Atoi(h)
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, false
	}
	minute, err = strconv.Atoi(m)
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, false
	}
	return hour, minute, true
}

// String describes a stage's cadence for a startup banner.
func (s *Stage) String() string {
	switch {
	case s.At != "":
		return fmt.Sprintf("%s at %s", s.Name, s.At)
	case s.Every > 0:
		return fmt.Sprintf("%s every %s", s.Name, s.Every)
	}
	return s.Name + " off"
}
