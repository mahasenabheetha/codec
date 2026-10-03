package web

import (
	"fmt"
	"strings"
	"time"

	"github.com/mahasenabheetha/codec/v2/internal/cron"
	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/timeutil"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// schedule is a cron expression found in a file, with the zone it runs
// in and what the platform adds.
type schedule struct {
	expr     string
	zone     string // "" = UTC
	zoneNote string // where the zone comes from
	notes    []string
}

// scheduleAt finds the cron schedule under the cursor: a Kubernetes
// CronJob's spec.schedule, an Argo CronWorkflow's spec.schedule(s),
// GitHub's on.schedule[].cron and Azure's schedules[].cron.
func scheduleAt(f *provider.File, tool string, at yamlkit.Pos) (*schedule, *yamlkit.Node) {
	if f.YAML == nil {
		return nil, nil
	}
	doc := f.YAML.DocAt(at)
	if doc == nil {
		return nil, nil
	}
	path, n := doc.PathAt(at)
	if n == nil || n.Kind != yamlkit.KindScalar {
		return nil, nil
	}
	p := path.String()
	root := doc.Root
	switch {
	case tool == "kubernetes" && root.Get("kind").Str() == "CronJob" && p == "spec.schedule":
		s := &schedule{expr: n.Str(), zone: root.Get("spec").Get("timeZone").Str()}
		if s.zone == "" {
			s.zoneNote = "spec.timeZone isn't set: the controller's local zone, usually UTC"
		} else {
			s.zoneNote = "from spec.timeZone"
		}
		if strings.HasPrefix(s.expr, "CRON_TZ=") || strings.HasPrefix(s.expr, "TZ=") {
			s.notes = append(s.notes, "Kubernetes rejects CRON_TZ= and TZ= in the schedule; set spec.timeZone instead")
		}
		return s, n
	case tool == "argo-workflows" && root.Get("kind").Str() == "CronWorkflow" && (p == "spec.schedule" || strings.HasPrefix(p, "spec.schedules[")):
		s := &schedule{expr: n.Str(), zone: root.Get("spec").Get("timezone").Str()}
		if s.zone == "" {
			s.zoneNote = "spec.timezone isn't set: the controller's local zone, usually UTC"
		} else {
			s.zoneNote = "from spec.timezone"
		}
		return s, n
	case tool == "github-actions" && strings.HasPrefix(p, "on.schedule[") && strings.HasSuffix(p, "].cron"):
		return &schedule{expr: n.Str(), zoneNote: "GitHub runs schedules in UTC", notes: []string{
			"Runs only from the default branch, at most every 5 minutes, and can start late when GitHub is busy",
		}}, n
	case tool == "azure-pipelines" && strings.HasPrefix(p, "schedules[") && strings.HasSuffix(p, "].cron"):
		return &schedule{expr: n.Str(), zoneNote: "Azure Pipelines runs schedules in UTC"}, n
	}
	return nil, nil
}

// cronHover explains a schedule: plain words, zone, next runs.
func cronHover(f *provider.File, tool string, at yamlkit.Pos, now time.Time) *provider.Hover {
	s, n := scheduleAt(f, tool, at)
	if s == nil {
		return nil
	}
	h := &provider.Hover{Range: n.Range, Title: "Cron schedule"}
	sched, err := cron.Parse(s.expr)
	if err != nil {
		h.Rows = append(h.Rows, provider.HoverRow{Label: "Problem", Value: err.Error()})
		return h
	}
	zone := s.zone
	if sched.Zone != "" {
		zone, s.zoneNote = sched.Zone, "from CRON_TZ="
	}
	loc, err := timeutil.Zone(zone)
	if err != nil {
		h.Rows = append(h.Rows, provider.HoverRow{Label: "Problem", Value: fmt.Sprintf("unknown time zone %q", zone)})
		return h
	}
	h.Rows = append(h.Rows, provider.HoverRow{Label: "Runs", Value: sched.Describe()})
	h.Rows = append(h.Rows, provider.HoverRow{Label: "Time zone", Value: loc.String() + " · " + s.zoneNote})
	runs, skipped := sched.Next(now, 3, loc)
	if len(runs) == 0 {
		h.Rows = append(h.Rows, provider.HoverRow{Label: "Next", Value: "never: no date matches"})
	}
	for i, r := range runs {
		label := "Next"
		if i > 0 {
			label = "Then"
		}
		h.Rows = append(h.Rows, provider.HoverRow{Label: label, Value: r.Format("Mon 2 Jan 2006 15:04 MST") + " · " + timeutil.Relative(r, now)})
	}
	for _, sk := range skipped {
		h.Rows = append(h.Rows, provider.HoverRow{Label: "Skipped", Value: sk.Wall + ": the clocks jump over it"})
	}
	s.notes = append(sched.Notes(), s.notes...)
	for _, note := range s.notes {
		h.Rows = append(h.Rows, provider.HoverRow{Label: "Note", Value: note})
	}
	return h
}
