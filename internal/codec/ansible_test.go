package codec

import (
	"strings"
	"testing"
)

// fixtureFailed is a trimmed version of a real ansible -vv failure.
// Raw string literal: the \n inside the JSON stay as literal
// backslash-n, exactly as ansible prints them.
const fixtureFailed = `TASK [ifsinstaller : Executing ifs installer] **********************************
task path: /builds/ifsgl/dev/ansible/roles/ifsinstaller/tasks/ifsinstaller.install.yml:240
fatal: [localhost]: FAILED! => {"changed": false, "cmd": "# Running the installer\nbash ./installer.sh --timeout 30m --set action=\"mtinstaller\"\n", "delta": "0:00:13.244235", "end": "2026-08-18 09:35:57.276976", "msg": "non-zero return code", "rc": 1, "start": "2026-08-18 09:35:44.032741", "stderr": "[Tue Aug 18][INSTALLER] - INFO: Installer Action: mtinstaller\n[Tue Aug 18][MTINSTALL] - SEVERE: failed to perform \"FetchReference\": not found\n[Tue Aug 18][INSTALLER] - INFO: Installer finished with SEVERE errors", "stderr_lines": ["[Tue Aug 18][INSTALLER] - INFO: Installer Action: mtinstaller", "[Tue Aug 18][MTINSTALL] - SEVERE: failed to perform \"FetchReference\": not found", "[Tue Aug 18][INSTALLER] - INFO: Installer finished with SEVERE errors"], "stdout": "", "stdout_lines": []}`

func TestParseAnsibleFailedTask(t *testing.T) {
	task, err := ParseAnsible(fixtureFailed)
	if err != nil {
		t.Fatalf("ParseAnsible returned error: %v", err)
	}

	if task.Name != "ifsinstaller : Executing ifs installer" {
		t.Errorf("Name = %q", task.Name)
	}
	if !strings.HasSuffix(task.Path, "ifsinstaller.install.yml:240") {
		t.Errorf("Path = %q", task.Path)
	}
	if task.Host != "localhost" {
		t.Errorf("Host = %q, want localhost", task.Host)
	}
	if task.Status != "FAILED" {
		t.Errorf("Status = %q, want FAILED", task.Status)
	}

	// rc must be the exact integer "1", flagged bad.
	rc := findField(t, task, "rc")
	if rc.Value != "1" || !rc.Bad {
		t.Errorf("rc = %+v, want Value=1 Bad=true", rc)
	}
	msg := findField(t, task, "msg")
	if !msg.Bad {
		t.Errorf("msg %q should be flagged bad", msg.Value)
	}

	// The command section must have real lines (not \n escapes), with
	// each --flag prettified onto its own indented line.
	cmd := findSection(t, task, "command")
	if len(cmd.Lines) < 4 {
		t.Fatalf("command section has %d lines, want >= 4:\n%+v", len(cmd.Lines), cmd.Lines)
	}
	if cmd.Lines[1].Text != "bash ./installer.sh" {
		t.Errorf("command line 2 = %q, want the bare command", cmd.Lines[1].Text)
	}
	if !strings.HasPrefix(cmd.Lines[2].Text, "  --timeout") {
		t.Errorf("command line 3 = %q, want an indented --flag", cmd.Lines[2].Text)
	}

	// The engine's diagnosis must be the SEVERE stderr line.
	if task.Cause == nil {
		t.Fatal("Cause is nil for a failed task")
	}
	if !strings.Contains(task.Cause.Text, "FetchReference") {
		t.Errorf("Cause.Text = %q, want the SEVERE line", task.Cause.Text)
	}
	if task.Cause.Source != "stderr" {
		t.Errorf("Cause.Source = %q, want stderr", task.Cause.Source)
	}

	// stderr must come from stderr_lines, with the SEVERE line
	// classified as an error and INFO lines left unclassified.
	stderr := findSection(t, task, "stderr")
	if len(stderr.Lines) != 3 {
		t.Fatalf("stderr has %d lines, want 3", len(stderr.Lines))
	}
	if stderr.Lines[0].Level != "" {
		t.Errorf("INFO line classified as %q, want \"\"", stderr.Lines[0].Level)
	}
	if stderr.Lines[1].Level != "error" {
		t.Errorf("SEVERE line classified as %q, want error", stderr.Lines[1].Level)
	}

	// Empty stdout must not produce a section.
	for _, sec := range task.Sections {
		if sec.Title == "stdout" {
			t.Error("empty stdout produced a section")
		}
	}
}

func TestParseAnsibleOKTask(t *testing.T) {
	task, err := ParseAnsible(`ok: [web01] => {"changed": false, "ping": "pong"}`)
	if err != nil {
		t.Fatalf("ParseAnsible returned error: %v", err)
	}
	if task.Status != "OK" {
		t.Errorf("Status = %q, want OK", task.Status)
	}
	if task.Host != "web01" {
		t.Errorf("Host = %q, want web01", task.Host)
	}
	// "ping" is an unknown scalar: it should land in the summary.
	if findField(t, task, "ping").Value != "pong" {
		t.Error("unknown scalar key did not reach the summary")
	}
}

func TestParseAnsibleStripsANSICodes(t *testing.T) {
	in := "\x1b[0;31mfatal: [h1]: FAILED! => {\"rc\": 2}\x1b[0m"
	task, err := ParseAnsible(in)
	if err != nil {
		t.Fatalf("ParseAnsible returned error: %v", err)
	}
	if findField(t, task, "rc").Value != "2" {
		t.Error("ANSI codes were not stripped before parsing")
	}
}

func TestParseAnsibleRejectsNonAnsible(t *testing.T) {
	for _, in := range []string{"", `{"a":1}`, "just some text", "TASK [x] but no result line"} {
		if _, err := ParseAnsible(in); err == nil {
			t.Errorf("ParseAnsible(%q) succeeded, want an error", in)
		}
	}
}

func TestParseAnsibleLoopItem(t *testing.T) {
	task, err := ParseAnsible(`failed: [localhost] (item=nginx) => {"rc": 2, "msg": "package not found"}`)
	if err != nil {
		t.Fatalf("ParseAnsible returned error: %v", err)
	}
	if task.Status != "FAILED" {
		t.Errorf("Status = %q, want FAILED (from the 'failed' verb)", task.Status)
	}
	if task.Item != "nginx" {
		t.Errorf("Item = %q, want nginx", task.Item)
	}
}

func TestParseAnsibleItemAfterArrow(t *testing.T) {
	// Some modules emit the loop item BETWEEN two arrows.
	in := `changed: [localhost] => (item=1.34.9) => {"changed": true, "kube_id": "1.34.9", "msg": "OK (60752056 bytes)", "status_code": 200}`
	task, err := ParseAnsible(in)
	if err != nil {
		t.Fatalf("ParseAnsible returned error: %v", err)
	}
	if task.Status != "CHANGED" {
		t.Errorf("Status = %q, want CHANGED", task.Status)
	}
	if task.Item != "1.34.9" {
		t.Errorf("Item = %q, want 1.34.9", task.Item)
	}
	if findField(t, task, "status_code").Value != "200" {
		t.Error("status_code did not reach the summary")
	}
}

func TestParseAnsibleWrapperPrefix(t *testing.T) {
	// Packer and CI wrappers prefix every line, e.g. "    azure-arm: ".
	in := "    azure-arm: TASK [Checkout the HEAD revision] ****\n" +
		"    azure-arm: task path: /builds/playbooks/sanity.yml:48\n" +
		`    azure-arm: changed: [default] => {"changed": true, "rc": 0, "stderr": "HEAD is now at 0c966d1", "stderr_lines": ["HEAD is now at 0c966d1"]}`
	task, err := ParseAnsible(in)
	if err != nil {
		t.Fatalf("ParseAnsible returned error: %v", err)
	}
	if task.Status != "CHANGED" {
		t.Errorf("Status = %q, want CHANGED", task.Status)
	}
	if task.Host != "default" {
		t.Errorf("Host = %q, want default", task.Host)
	}
	if task.Name != "Checkout the HEAD revision" {
		t.Errorf("Name = %q", task.Name)
	}
	if len(findSection(t, task, "stderr").Lines) != 1 {
		t.Error("stderr section missing")
	}
}

func TestParseAnsibleSkipped(t *testing.T) {
	task, err := ParseAnsible(`skipping: [localhost] => {"changed": false, "skip_reason": "Conditional result was False"}`)
	if err != nil {
		t.Fatalf("ParseAnsible returned error: %v", err)
	}
	if task.Status != "SKIPPED" {
		t.Errorf("Status = %q, want SKIPPED", task.Status)
	}
	if task.Cause != nil {
		t.Error("skipped task should not get a Cause diagnosis")
	}
}

func TestParseAnsibleRetries(t *testing.T) {
	in := "FAILED - RETRYING: Wait for service (3 retries left).\n" +
		"FAILED - RETRYING: Wait for service (2 retries left).\n" +
		`fatal: [localhost]: FAILED! => {"rc": 1, "msg": "timed out"}`
	task, err := ParseAnsible(in)
	if err != nil {
		t.Fatalf("ParseAnsible returned error: %v", err)
	}
	if task.Retries != 2 {
		t.Errorf("Retries = %d, want 2", task.Retries)
	}
}

func TestParseAnsibleWarningsKey(t *testing.T) {
	task, err := ParseAnsible(`ok: [h1] => {"changed": false, "warnings": ["Platform linux is using a discovered python"]}`)
	if err != nil {
		t.Fatalf("ParseAnsible returned error: %v", err)
	}
	w := findSection(t, task, "warnings")
	if len(w.Lines) != 1 || w.Lines[0].Level != "warn" {
		t.Errorf("warnings section = %+v, want one warn-level line", w.Lines)
	}
}

func TestClassifyLine(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		// Explicit tags win over keywords.
		{"[ts][MTINSTALL] - INFO: chartVersion not found in properties, using default", ""},
		{"[ts][MTINSTALL] - SEVERE: failed to fetch chart", "error"},
		{"[ts][INSTALLER] - WARNING: something looks off", "warn"},
		{"[ts][INSTALLER] - INFO: Installer finished with SEVERE errors", ""},
		// Untagged lines use keywords; warn wins over error keywords.
		{"Connection timed out after 30s", "error"},
		{"permission denied while connecting", "error"},
		{"FAILED - RETRYING: wait for it (3 retries left)", "warn"},
		{"everything is fine here", ""},
		// PLAY RECAP zero-counts must not trigger.
		{"localhost : ok=3 changed=1 unreachable=0 failed=0", ""},
	}
	for _, tt := range tests {
		if got := classifyLine(tt.line); got != tt.want {
			t.Errorf("classifyLine(%q) = %q, want %q", tt.line, got, tt.want)
		}
	}
}

func TestDetectAnsible(t *testing.T) {
	if got := Detect(fixtureFailed); got != KindAnsible {
		t.Errorf("Detect(fixture) = %v, want KindAnsible", got)
	}
}

func TestTransformAnsible(t *testing.T) {
	out, kind, err := Transform(fixtureFailed)
	if err != nil {
		t.Fatalf("Transform returned error: %v", err)
	}
	if kind != KindAnsible {
		t.Errorf("kind = %v, want KindAnsible", kind)
	}
	if !strings.Contains(out, "STATUS  FAILED") {
		t.Errorf("text rendering missing status:\n%s", out)
	}
}

func findField(t *testing.T, task *ParsedTask, key string) Field {
	t.Helper()
	for _, f := range task.Summary {
		if f.Key == key {
			return f
		}
	}
	t.Fatalf("summary has no %q field: %+v", key, task.Summary)
	return Field{}
}

func findSection(t *testing.T, task *ParsedTask, title string) Section {
	t.Helper()
	for _, s := range task.Sections {
		if s.Title == title {
			return s
		}
	}
	t.Fatalf("no %q section", title)
	return Section{}
}
