package ansiblelog

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
	"time"
)

// texts joins the cleaned lines, marking groups and wrappers.
func texts(lines []Line) string {
	var out []string
	for _, l := range lines {
		s := l.Text
		switch l.Kind {
		case GroupStart:
			s = "{" + s
		case GroupEnd:
			s = "}" + s
		}
		if l.Wrap != "" {
			s = "<" + l.Wrap + ">" + s
		}
		if l.Err {
			s = "!" + s
		}
		out = append(out, s)
	}
	return strings.Join(out, "\n")
}

func TestCleanGitLab(t *testing.T) {
	// GitLab's raw log: stream markers, sections, colour codes, CRLF,
	// and a long result split into "+" continuation records.
	in := "2026-01-02T10:00:00.000001Z 00O \x1b[0KRunning with gitlab-runner\x1b[0;m\r\n" +
		"2026-01-02T10:00:00.000002Z 00O section_start:1767348000:step_script[collapsed=true]\r\n" +
		"\x1b[0K\r\n" +
		"2026-01-02T10:00:00.000003Z 00O+\x1b[0K\x1b[36;1mExecuting \"step_script\"\x1b[0;m\r\n" +
		"2026-01-02T10:00:01.000000Z 01O \x1b[0;32mPLAY [web] ****\x1b[0m\r\n" +
		"2026-01-02T10:00:02.000000Z 01O fatal: [web-1]: FAILED! => {\"msg\": \"first half \n" +
		"2026-01-02T10:00:02.000001Z 01O+second half\"}\r\n" +
		"2026-01-02T10:00:03.000000Z 01E warning on stderr\r\n" +
		"2026-01-02T10:00:04.000000Z 00O section_end:1767348004:step_script\r\n"
	got := Clean(in)
	want := strings.Join([]string{
		"Running with gitlab-runner",
		"{step_script",
		`Executing "step_script"`,
		"PLAY [web] ****",
		`fatal: [web-1]: FAILED! => {"msg": "first half second half"}`,
		"!warning on stderr",
		"}step_script",
	}, "\n")
	if texts(got) != want {
		t.Fatalf("got\n%s\nwant\n%s", texts(got), want)
	}
	if got[3].N != 5 || !got[3].Time.Equal(time.Date(2026, 1, 2, 10, 0, 1, 0, time.UTC)) {
		t.Errorf("line/time = %d %v", got[3].N, got[3].Time)
	}
}

func TestCleanAzureAndGitHub(t *testing.T) {
	in := strings.Join([]string{
		"2026-01-02T10:00:00.1234567Z ##[section]Starting: Deploy",
		"2026-01-02T10:00:01.1234567Z ##[group]Run ansible-playbook site.yml",
		"2026-01-02T10:00:01.2234567Z ansible-playbook site.yml -i hosts",
		"2026-01-02T10:00:01.3234567Z ##[endgroup]",
		"2026-01-02T10:00:02.0000000Z TASK [ping] ***",
		"2026-01-02T10:00:03.0000000Z ##[error]Process completed with exit code 2.",
		"[2026-01-02T10:00:04.000Z] PLAY RECAP ***",
		"::group::Upload logs",
		"::endgroup::",
	}, "\n")
	want := strings.Join([]string{
		"Starting: Deploy",
		"{Run ansible-playbook site.yml",
		"ansible-playbook site.yml -i hosts",
		"}",
		"TASK [ping] ***",
		"!Process completed with exit code 2.",
		"PLAY RECAP ***",
		"{Upload logs",
		"}",
	}, "\n")
	got := Clean(in)
	if texts(got) != want {
		t.Fatalf("got\n%s\nwant\n%s", texts(got), want)
	}
	if got[6].Time.IsZero() || !got[7].Time.IsZero() {
		t.Errorf("times: %v / %v", got[6].Time, got[7].Time)
	}
}

func TestCleanWrappers(t *testing.T) {
	// Packer prints Ansible inside its own prefix and colours; Compose
	// prefixes every service line. Both are learned from the headers.
	in := strings.Join([]string{
		"\x1b[1;32m==> azure-arm: Provisioning with Ansible...\x1b[0m",
		"\x1b[0;32m    azure-arm: PLAY [all] ****\x1b[0m",
		"\x1b[0;32m    azure-arm:\x1b[0m",
		"\x1b[0;32m    azure-arm: TASK [ping] ****\x1b[0m",
		"\x1b[0;32m    azure-arm: \x1b[0;33mchanged: [default]\x1b[0m",
		"web-1  | PLAY [db] ***",
		"web-1  | TASK [migrate] ***",
		"web-1  | ok: [db-1]",
		"db-1   | ready to accept connections",
		"plain line mentioning TASK [x] once",
	}, "\n")
	want := strings.Join([]string{
		"==> azure-arm: Provisioning with Ansible...",
		"<azure-arm>PLAY [all] ****",
		"<azure-arm>",
		"<azure-arm>TASK [ping] ****",
		"<azure-arm>changed: [default]",
		"<web-1>PLAY [db] ***",
		"<web-1>TASK [migrate] ***",
		"<web-1>ok: [db-1]",
		"db-1   | ready to accept connections",
		"plain line mentioning TASK [x] once",
	}, "\n")
	if got := texts(Clean(in)); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestCleanRedraw(t *testing.T) {
	got := Clean("Downloading 10%\rDownloading 60%\rDownloading 100%\r\nnext")
	if texts(got) != "Downloading 100%\nnext" {
		t.Errorf("got %q", texts(got))
	}
}

// Whatever the input, every input line lands in exactly one Line, in
// order, and Clean never panics.
func TestCleanKeepsEveryLine(t *testing.T) {
	pieces := []string{"2026-01-02T10:00:00.1Z 00O ", "2026-01-02T10:00:00.1Z 01E+", "\x1b[0;32m", "\x1b]8;;x\x07", "\r", "##[group]", "section_start:1:a",
		"    azure-arm: ", "TASK [t] **", "PLAY [p] **", "ok: [h]", "{\"a\": \"}\"", "ü", "\x1b"}
	r := rand.New(rand.NewPCG(1, 2))
	for range 2000 {
		var b strings.Builder
		n := 1 + r.IntN(12)
		for range n {
			for range r.IntN(5) {
				b.WriteString(pieces[r.IntN(len(pieces))])
			}
			b.WriteByte('\n')
		}
		in := b.String()
		lines := Clean(in)
		total := strings.Count(strings.TrimSuffix(in, "\n"), "\n") + 1
		prev := 0
		for _, l := range lines {
			if l.N <= prev || l.N > total {
				t.Fatalf("line numbers out of order in %q: %v", in, lines)
			}
			prev = l.N
		}
		if lines[0].N != 1 {
			t.Fatalf("first line %d in %q", lines[0].N, in)
		}
	}
}

func BenchmarkClean(b *testing.B) {
	var sb strings.Builder
	for i := range 20000 {
		fmt.Fprintf(&sb, "2026-01-02T10:00:00.%06dZ 01O \x1b[0;32m    azure-arm: TASK [step %d] ****\x1b[0m\n", i, i)
		fmt.Fprintf(&sb, "2026-01-02T10:00:00.%06dZ 01O \x1b[0;32m    azure-arm: ok: [default] => {\"changed\": false}\x1b[0m\n", i)
	}
	in := sb.String()
	b.SetBytes(int64(len(in)))
	for b.Loop() {
		Clean(in)
	}
}
