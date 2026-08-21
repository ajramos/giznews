// Package learn turns the reader's own history into a bounded importance
// adjustment.
//
// Nothing here is a model. It is a table of rates you can read, argue with and
// switch off: how much of a source you throw away unopened, what you star. The
// point is not to predict taste, it is to stop the classifier from being told
// the same thing every day and never hearing the answer.
package learn

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/ajramos/giznews/internal/db"
)

// Options configures a learning pass.
type Options struct {
	WindowDays int // how far back to look (0 = 90)
	MinSamples int // articles a source or tag needs before it may move anything (0 = 20)
	MaxDelta   int // the most importance may be moved, up or down (0 = 1)
}

func (o Options) windowDays() int {
	if o.WindowDays > 0 {
		return o.WindowDays
	}
	return 90
}

func (o Options) minSamples() int {
	if o.MinSamples > 0 {
		return o.MinSamples
	}
	return 20
}

func (o Options) maxDelta() int {
	if o.MaxDelta > 0 {
		return o.MaxDelta
	}
	return 1
}

// Signal is what one source or one tag has earned.
type Signal struct {
	Kind    string `json:"kind"` // source | tag
	Key     string `json:"key"`
	Label   string `json:"label"`
	Samples int    `json:"samples"`
	// ReadRate is reported but never acted on: the reader opens whatever the
	// cursor lands on, so "read" says as much about the list order as about the
	// article. Starring and throwing away are deliberate; those decide.
	ReadRate float64 `json:"read_rate"`
	DropRate float64 `json:"drop_rate"` // archived without ever being opened
	StarRate float64 `json:"star_rate"`
	Delta    int     `json:"delta"`
	// Match is a regex that would catch this source's articles, so a signal
	// strong enough to be a rule can be proposed as one.
	Match string `json:"match,omitempty"`
}

// verdict is one article the reader acted on.
type verdict struct {
	sourceID   int64
	sourceName string
	sourceURL  string
	tags       []string
	read       bool
	archived   bool
	starred    bool
}

// Compute reads the history and works out what each source and tag has earned.
// It writes nothing.
func Compute(ctx context.Context, d *db.DB, opts Options) ([]Signal, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -opts.windowDays()).Format(time.RFC3339)
	rows, err := d.SQL().QueryContext(ctx, `
		SELECT a.source_id, COALESCE(s.name, ''), COALESCE(s.url, ''), a.tags,
		       MAX(CASE WHEN e.event = 'read' THEN 1 ELSE 0 END),
		       MAX(CASE WHEN e.event = 'archived' THEN 1 ELSE 0 END),
		       MAX(CASE WHEN e.event = 'starred' THEN 1 ELSE 0 END)
		FROM articles a
		LEFT JOIN sources s ON s.id = a.source_id
		JOIN article_events e ON e.article_id = a.id AND e.actor = 'user'
		WHERE a.fetched_at >= ?
		GROUP BY a.id`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("learn: read history: %w", err)
	}
	defer rows.Close()

	var verdicts []verdict
	for rows.Next() {
		var (
			v                     verdict
			tagsRaw               string
			read, archived, starr int
		)
		if err := rows.Scan(&v.sourceID, &v.sourceName, &v.sourceURL, &tagsRaw, &read, &archived, &starr); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(tagsRaw), &v.tags)
		v.read, v.archived, v.starred = read == 1, archived == 1, starr == 1
		verdicts = append(verdicts, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return signalsFrom(verdicts, opts), nil
}

// tally accumulates the verdicts for one source or tag.
type tally struct {
	label   string
	match   string
	n       int
	read    int
	dropped int
	starred int
}

func signalsFrom(verdicts []verdict, opts Options) []Signal {
	sources := map[string]*tally{}
	tags := map[string]*tally{}

	for _, v := range verdicts {
		key := fmt.Sprintf("%d", v.sourceID)
		add(sources, key, v.sourceName, domainOf(v.sourceURL), v)
		for _, t := range v.tags {
			if t == "" {
				continue
			}
			add(tags, t, t, "", v)
		}
	}

	out := make([]Signal, 0, len(sources)+len(tags))
	for key, t := range sources {
		out = append(out, t.signal("source", key, opts))
	}
	for key, t := range tags {
		out = append(out, t.signal("tag", key, opts))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].Samples != out[j].Samples {
			return out[i].Samples > out[j].Samples
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func add(into map[string]*tally, key, label, match string, v verdict) {
	t := into[key]
	if t == nil {
		t = &tally{label: label, match: match}
		into[key] = t
	}
	t.n++
	if v.read {
		t.read++
	}
	if v.archived && !v.read {
		t.dropped++
	}
	if v.starred {
		t.starred++
	}
}

// Thresholds for a verdict. They are deliberately far apart: a signal that only
// just clears the bar is noise wearing a number.
const (
	dropIsHostile   = 0.7  // thrown away unopened this often
	starIsAffection = 0.2  // starred this often
	starRescues     = 0.05 // any real starring cancels a hostile drop rate
)

func (t *tally) signal(kind, key string, opts Options) Signal {
	s := Signal{
		Kind: kind, Key: key, Label: t.label, Samples: t.n, Match: t.match,
		ReadRate: ratio(t.read, t.n),
		DropRate: ratio(t.dropped, t.n),
		StarRate: ratio(t.starred, t.n),
	}
	if t.n < opts.minSamples() {
		return s // not enough evidence to move anything
	}
	switch {
	case s.StarRate >= starIsAffection:
		s.Delta = opts.maxDelta()
	case s.DropRate >= dropIsHostile && s.StarRate < starRescues:
		s.Delta = -opts.maxDelta()
	}
	return s
}

func ratio(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total)
}

// domainOf is the host part of a source URL, escaped so it can go in a rule.
func domainOf(raw string) string {
	host := raw
	for _, prefix := range []string{"https://", "http://"} {
		if len(host) > len(prefix) && host[:len(prefix)] == prefix {
			host = host[len(prefix):]
			break
		}
	}
	for i, r := range host {
		if r == '/' || r == '?' {
			host = host[:i]
			break
		}
	}
	if host == "" {
		return ""
	}
	escaped := make([]rune, 0, len(host)+4)
	for _, r := range host {
		if r == '.' {
			escaped = append(escaped, '\\')
		}
		escaped = append(escaped, r)
	}
	return string(escaped)
}

// Store persists what a pass learned, replacing what the last one thought.
func Store(ctx context.Context, d *db.DB, signals []Signal) error {
	now := db.Now()
	for _, s := range signals {
		_, err := d.SQL().ExecContext(ctx, `
			INSERT INTO signals (kind, key, label, samples, read_rate, drop_rate, star_rate, delta, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(kind, key) DO UPDATE SET
				label = excluded.label, samples = excluded.samples,
				read_rate = excluded.read_rate, drop_rate = excluded.drop_rate,
				star_rate = excluded.star_rate, delta = excluded.delta,
				updated_at = excluded.updated_at`,
			s.Kind, s.Key, s.Label, s.Samples, s.ReadRate, s.DropRate, s.StarRate, s.Delta, now)
		if err != nil {
			return fmt.Errorf("learn: store %s %q: %w", s.Kind, s.Key, err)
		}
	}
	return nil
}

// Adjustments is what has been learned, ready to apply: "source:3" or "tag:agents"
// to a bounded delta. Only the entries that actually move something are kept.
type Adjustments map[string]int

// Load reads the stored adjustments. An empty map means nobody has run a
// learning pass yet, and the classifier behaves exactly as it did before.
func Load(ctx context.Context, d *db.DB) (Adjustments, error) {
	rows, err := d.SQL().QueryContext(ctx,
		"SELECT kind, key, delta FROM signals WHERE delta != 0")
	if err != nil {
		return nil, fmt.Errorf("learn: load signals: %w", err)
	}
	defer rows.Close()
	out := Adjustments{}
	for rows.Next() {
		var (
			kind, key string
			delta     int
		)
		if err := rows.Scan(&kind, &key, &delta); err != nil {
			return nil, err
		}
		out[kind+":"+key] = delta
	}
	return out, rows.Err()
}

// For returns the adjustment an article has earned: its source's, plus its
// tags', bounded so no pile-up of weak signals can move an article more than
// one step.
func (a Adjustments) For(sourceID int64, tags []string, maxDelta int) int {
	if len(a) == 0 {
		return 0
	}
	if maxDelta <= 0 {
		maxDelta = 1
	}
	delta := a[fmt.Sprintf("source:%d", sourceID)]
	for _, t := range tags {
		delta += a["tag:"+t]
	}
	if delta > maxDelta {
		return maxDelta
	}
	if delta < -maxDelta {
		return -maxDelta
	}
	return delta
}
