package logview

import (
	"cmp"
	"slices"
	"strings"
	"time"
)

// Argo workflows: a run's log is read step by step (decision 87). An
// entry's step is, best first: its own argo_step_name field; what other
// entries of the same pod said; the pod's name, which Argo makes
// "<workflow>-<template>-<number>" (the workflow ending in Kubernetes'
// five-character random suffix). Plain lines with no source take the
// step of the entry before them. Lines of Argo's own executor (its init
// and wait containers) count as output, so they fold with it.

const (
	stepKey     = "argo_step_name"
	workflowKey = "argo_workflow_name"
)

// StepStat is one step of an Argo run, over the whole log.
type StepStat struct {
	Name     string    `json:"name"`
	Pods     int       `json:"pods"` // sources that logged it
	Entries  int       `json:"entries"`
	Errors   int       `json:"errors"` // error or worse
	Warnings int       `json:"warnings"`
	From     time.Time `json:"from,omitzero"`
	To       time.Time `json:"to,omitzero"`
	First    int       `json:"first"` // its first entry
}

// Step is entry i's Argo step; "" when the log isn't an Argo run.
func (l *Log) Step(i int) string { return l.strs[l.rows[i].step] }

// podName takes the pod out of a source: "pod/name/container" from
// kubectl, else the source itself (argo logs, stern).
func podName(source string) (pod, container string) {
	s := source
	if rest, ok := strings.CutPrefix(s, "pod/"); ok {
		s = rest
	} else if rest, ok := strings.CutPrefix(s, "pods/"); ok {
		s = rest
	} else {
		return source, ""
	}
	pod, container, _ = strings.Cut(s, "/")
	return pod, container
}

// argoPod reads an Argo pod name, "<workflow>-<template>-<number>", or
// "<workflow>-<number>" from Argo's older naming, where the step is the
// pod itself. A workflow named in the log's fields is taken as is;
// otherwise its name ends in a generated five-character suffix.
func argoPod(pod string, workflows []string) (workflow, step string, ok bool) {
	i := strings.LastIndexByte(pod, '-')
	if i <= 0 || len(pod)-i-1 < 6 || len(pod)-i-1 > 10 || !allDigits(pod[i+1:]) {
		return "", "", false
	}
	name := pod[:i]
	for _, wf := range workflows {
		if name == wf {
			return wf, pod, true
		}
		if rest, ok := strings.CutPrefix(name, wf+"-"); ok {
			return wf, rest, true
		}
	}
	segs := strings.Split(name, "-")
	for k := 1; k < len(segs); k++ {
		if len(segs[k]) == 5 && kubeRandom(segs[k]) {
			wf := strings.Join(segs[:k+1], "-")
			if k == len(segs)-1 {
				return wf, pod, true
			}
			return wf, strings.Join(segs[k+1:], "-"), true
		}
	}
	return "", "", false
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

// steps works out every entry's step and Argo's executor lines, once
// the log is read. A log with no sign of Argo is left as it is.
func (l *Log) steps() {
	stepID, hasStep := l.keyIDs[stepKey]
	wfID, hasWf := l.keyIDs[workflowKey]
	argoID, hasArgo := l.keyIDs["argo"]
	field := func(i int, key uint32) string {
		for _, c := range l.fields(i) {
			if c.key == key {
				return c.text
			}
		}
		return ""
	}

	// What the fields say, per row and per source.
	explicit := make([]string, len(l.rows))
	bySource := map[uint32]string{}
	var workflows []string
	for i := range l.rows {
		r := &l.rows[i]
		if hasStep {
			if s := field(i, stepID); s != "" {
				explicit[i] = s
				if _, ok := bySource[r.source]; !ok && r.source != 0 {
					bySource[r.source] = s
				}
			}
		}
		if hasWf {
			if wf := field(i, wfID); wf != "" && !slices.Contains(workflows, wf) {
				workflows = append(workflows, wf)
			}
		}
	}

	// The pods' names, and their executor containers.
	fromPod := map[uint32]string{}
	executor := map[uint32]bool{}
	argo := len(workflows) > 0 || len(bySource) > 0
	for id := 1; id < len(l.strs); id++ {
		pod, container := podName(l.strs[id])
		if container == "init" || container == "wait" {
			executor[uint32(id)] = true
		}
		if wf, step, ok := argoPod(pod, workflows); ok {
			fromPod[uint32(id)] = step
			argo = true
			if !slices.Contains(workflows, wf) {
				workflows = append(workflows, wf)
			}
		}
	}
	if !argo {
		return
	}
	l.workflows = workflows

	last := map[uint16]uint32{} // per file, the step of the entry before
	for i := range l.rows {
		r := &l.rows[i]
		step := explicit[i]
		if step == "" {
			step = bySource[r.source]
		}
		if step == "" {
			step = fromPod[r.source]
		}
		if step != "" {
			r.step = l.str(step)
		} else if r.source == 0 {
			r.step = last[r.file]
		}
		last[r.file] = r.step
		if executor[r.source] || hasArgo && field(i, argoID) == "true" {
			r.argo = true
		}
	}
}

// stepStats sums the entries by step, in the order the steps start.
func (l *Log) stepStats() []StepStat {
	if l.workflows == nil {
		return nil
	}
	by := map[uint32]*StepStat{}
	pods := map[uint32]map[uint32]bool{}
	for i := range l.rows {
		r := &l.rows[i]
		if r.step == 0 {
			continue
		}
		s := by[r.step]
		if s == nil {
			s = &StepStat{Name: l.strs[r.step], First: i}
			by[r.step] = s
			pods[r.step] = map[uint32]bool{}
		}
		s.Entries++
		switch {
		case r.level >= Error:
			s.Errors++
		case r.level == Warn:
			s.Warnings++
		}
		if r.source != 0 {
			pods[r.step][r.source] = true
		}
		if r.time != 0 {
			t := time.Unix(0, r.time).UTC()
			if s.From.IsZero() || t.Before(s.From) {
				s.From = t
			}
			if t.After(s.To) {
				s.To = t
			}
		}
	}
	out := make([]StepStat, 0, len(by))
	for id, s := range by {
		s.Pods = len(pods[id])
		out = append(out, *s)
	}
	slices.SortFunc(out, func(a, b StepStat) int { return cmp.Compare(a.First, b.First) })
	return out
}
