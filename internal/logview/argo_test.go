package logview

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestArgoPod(t *testing.T) {
	for _, c := range []struct {
		pod       string
		workflows []string
		wf, step  string
		ok        bool
	}{
		{"build-7f2kd-checkout-1234567890", nil, "build-7f2kd", "checkout", true},
		{"build-xkqpz-run-unit-tests-123456789", nil, "build-xkqpz", "run-unit-tests", true},
		{"build-7f2kd-1234567890", nil, "build-7f2kd", "build-7f2kd-1234567890", true}, // older naming: the pod is the step
		{"nightly-checkout-1234567", []string{"nightly"}, "nightly", "checkout", true}, // a named workflow, from the fields
		{"wf-a1b2c-main-2222222222", []string{"wf-a1b2c"}, "wf-a1b2c", "main", true},
		{"web-7d9c8b6f5-x2x9z", nil, "", "", false}, // a Deployment's pod
		{"db-0", nil, "", "", false},
		{"report-12345", nil, "", "", false},                   // too short a number
		{"plain-name-checkout-1234567890", nil, "", "", false}, // no generated suffix, no workflow named
	} {
		wf, step, ok := argoPod(c.pod, c.workflows)
		if wf != c.wf || step != c.step || ok != c.ok {
			t.Errorf("%s: got %q %q %v", c.pod, wf, step, ok)
		}
	}
	if pod, c := podName("pod/build-7f2kd-test-1/wait"); pod != "build-7f2kd-test-1" || c != "wait" {
		t.Errorf("kubectl source: %q %q", pod, c)
	}
}

func stepsOf(l *Log) string {
	var out []string
	for _, s := range l.Summary().Steps {
		out = append(out, fmt.Sprintf("%s:%d/%de%dw", s.Name, s.Entries, s.Errors, s.Warnings))
	}
	return strings.Join(out, " ")
}

func TestSteps(t *testing.T) {
	// Steps from pod names; the executor's lines count as output.
	l := buildFixtures(t, "argo.log")
	if got := stepsOf(l); !strings.HasPrefix(got, "checkout:") || !strings.Contains(got, " test:") {
		t.Errorf("steps %s", got)
	}
	if wf := l.Summary().Workflows; len(wf) != 1 || wf[0] != "build-7f2kd" {
		t.Errorf("workflows %v", wf)
	}
	if l.Step(0) != "checkout" || !l.plain(0) || l.plain(2) {
		t.Errorf("entry 0: step %q, plain %v; entry 2 plain %v", l.Step(0), l.plain(0), l.plain(2))
	}
	if m, err := l.Match("step:test level>=error", time.Time{}, time.Time{}); err != nil || m.Total == 0 {
		t.Errorf("step:test: %v %v", m, err)
	}

	// Steps from fields win over the pod's name, for the pod's lines
	// without them too; a pod with no fields gets its template.
	l = buildFixtures(t, "argo-mixed.log")
	if l.Step(0) != "deploy" || l.Step(2) != "main" {
		t.Errorf("mixed: %q %q", l.Step(0), l.Step(2))
	}

	// No prefixes (saved from the Argo UI): plain lines take the step
	// of the entry before them.
	l = buildText(t, `{"argo_workflow_name":"wf","argo_step_name":"migrate","log.level":"info","message":"start"}
ALTER DATABASE x
{"argo_workflow_name":"wf","argo_step_name":"verify","log.level":"error","message":"check failed"}
`)
	if l.Step(0) != "migrate" || l.Step(1) != "migrate" || l.Step(2) != "verify" || stepsOf(l) != "migrate:2/0e0w verify:1/1e0w" {
		t.Errorf("no prefixes: %q %q %q %s", l.Step(0), l.Step(1), l.Step(2), stepsOf(l))
	}

	// Not Argo: no steps.
	l = buildFixtures(t, "ecs.log")
	if l.Summary().Steps != nil || l.Summary().Workflows != nil || l.Step(0) != "" {
		t.Errorf("ecs: %+v", l.Summary().Steps)
	}
}
