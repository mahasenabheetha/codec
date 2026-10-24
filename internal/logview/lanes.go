package logview

import (
	"slices"
	"time"
)

// Lanes: the matching entries over time, one lane per step of an Argo
// run, else per source (a pod, a container, a service): when each
// logged, and where its warnings and errors fall. Same axis as the
// histogram, so the two line up.

// Lanes is a log's lanes.
type Lanes struct {
	By     string    `json:"by"` // "step" or "source"
	From   time.Time `json:"from,omitzero"`
	StepMs int64     `json:"stepMs"`
	N      int       `json:"n"` // buckets
	Lanes  []Lane    `json:"lanes"`
	More   int       `json:"more,omitempty"` // lanes left out
}

// Lane is one step's or source's entries over time.
type Lane struct {
	Name    string `json:"name"`
	Entries int    `json:"entries"`
	Errors  int    `json:"errors"`
	// Cells are the buckets it logged in: bucket, entries, warnings,
	// errors (or worse), and the first entry there (the worst level's
	// first, so a click on a red cell goes to its error).
	Cells [][5]int `json:"cells"`
}

const maxLanes = 60

// Lanes lays out the given entries (all when ids is nil) in lanes over
// at most about maxBuckets steps of the log's span; entries without a
// time, step or source are left out.
func (l *Log) Lanes(ids []uint32, maxBuckets int) Lanes {
	out := Lanes{By: "source", Lanes: []Lane{}}
	key := func(r *row) uint32 { return r.source }
	if l.workflows != nil {
		out.By = "step"
		key = func(r *row) uint32 { return r.step }
	}
	start, step, n, ok := l.timeBuckets(l.summary.From, l.summary.To, maxBuckets)
	if !ok {
		return out
	}
	out.From, out.StepMs, out.N = start, step.Milliseconds(), n
	s0, st := start.UnixNano(), int64(step)

	type acc struct {
		lane  Lane
		first int
		cells map[int]*[5]int
		worst map[int]Level
	}
	by := map[uint32]*acc{}
	l.each(ids, func(i int) {
		r := &l.rows[i]
		k := key(r)
		if k == 0 || r.time < s0 {
			return
		}
		b := int((r.time - s0) / st)
		if b >= n {
			return
		}
		a := by[k]
		if a == nil {
			a = &acc{lane: Lane{Name: l.strs[k]}, first: i, cells: map[int]*[5]int{}, worst: map[int]Level{}}
			by[k] = a
		}
		a.lane.Entries++
		c := a.cells[b]
		if c == nil {
			c = &[5]int{b, 0, 0, 0, i}
			a.cells[b] = c
			a.worst[b] = r.level
		}
		c[1]++
		switch {
		case r.level >= Error:
			c[3]++
			a.lane.Errors++
		case r.level == Warn:
			c[2]++
		}
		if r.level > a.worst[b] {
			a.worst[b], c[4] = r.level, i
		}
	})
	accs := make([]*acc, 0, len(by))
	for _, a := range by {
		accs = append(accs, a)
	}
	slices.SortFunc(accs, func(x, y *acc) int { return x.first - y.first })
	if len(accs) > maxLanes {
		out.More = len(accs) - maxLanes
		accs = accs[:maxLanes]
	}
	for _, a := range accs {
		for _, c := range a.cells {
			a.lane.Cells = append(a.lane.Cells, *c)
		}
		slices.SortFunc(a.lane.Cells, func(x, y [5]int) int { return x[0] - y[0] })
		out.Lanes = append(out.Lanes, a.lane)
	}
	return out
}

// timeBuckets picks a round step that cuts from–to into at most about
// maxBuckets buckets, and where the first starts.
func (l *Log) timeBuckets(from, to time.Time, maxBuckets int) (start time.Time, step time.Duration, n int, ok bool) {
	if from.IsZero() || to.Before(from) || maxBuckets < 1 {
		return time.Time{}, 0, 0, false
	}
	span := to.Sub(from)
	step = histSteps[len(histSteps)-1]
	for _, s := range histSteps {
		if span/s < time.Duration(maxBuckets) {
			step = s
			break
		}
	}
	start = from.Truncate(step)
	return start, step, int(to.Sub(start)/step) + 1, true
}
