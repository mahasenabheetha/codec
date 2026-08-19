package codec

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ParsedTask is the structured form of one Ansible -vv task block.
// The json tags matter: this struct is serialized straight into the
// web API response, so the field names here ARE the frontend contract.
type ParsedTask struct {
	Name     string    `json:"name,omitempty"`
	Path     string    `json:"path,omitempty"`
	Host     string    `json:"host"`
	Item     string    `json:"item,omitempty"`    // loop item, if any
	Status   string    `json:"status"`            // FAILED, UNREACHABLE, OK, CHANGED, SKIPPED, ...
	Retries  int       `json:"retries,omitempty"` // "FAILED - RETRYING" lines seen
	Cause    *Cause    `json:"cause,omitempty"`   // best guess at the root cause
	Summary  []Field   `json:"summary,omitempty"`
	Sections []Section `json:"sections,omitempty"`
}

// Cause is the engine's diagnosis: the single most likely reason the
// task failed, and where that conclusion came from.
type Cause struct {
	Text   string `json:"text"`
	Source string `json:"source"` // "stderr", "stdout", "msg", or a field name
}

// Field is one key/value pair worth surfacing up top. Bad marks values
// the UI should highlight as problems (rc != 0, error-bearing msg).
type Field struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Bad   bool   `json:"bad,omitempty"`
}

// Section is a titled block of output lines (command, stderr, ...).
type Section struct {
	Title string `json:"title"`
	Lines []Line `json:"lines"`
}

// Line is one output line with its detected severity: "", "warn", or
// "error". Classification lives here, in the engine, so every UI colors
// the same lines the same way.
type Line struct {
	Text  string `json:"text"`
	Level string `json:"level,omitempty"`
}

// Compiled once at package load. MustCompile panics on a bad pattern,
// which is correct for literals: a typo here is a programming error,
// not a runtime condition.
var (
	// ANSI terminal color codes, e.g. \x1b[0;31m — present when logs
	// are copied from raw CI output.
	ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

	// TASK [ifsinstaller : Executing ifs installer] ******
	// Unanchored: wrappers like Packer prefix every line
	// ("    azure-arm: TASK [...]"), so line starts can't be trusted.
	taskRe = regexp.MustCompile(`TASK \[(.+?)\]`)

	// task path: /builds/.../ifsinstaller.install.yml:240
	pathRe = regexp.MustCompile(`(?m)task path: (.+)$`)

	// Result lines, all of these shapes:
	//   fatal: [localhost]: FAILED! => {
	//   fatal: [web01]: UNREACHABLE! => {
	//   ok: [web01] => {
	//   changed: [db01] => {
	//   skipping: [localhost] => {
	//   failed: [localhost] (item=nginx) => {          item BEFORE arrow
	//   changed: [localhost] => (item=1.34.9) => {     item AFTER arrow
	//       azure-arm: changed: [default] => {          wrapper prefix
	//
	// Because a prefix may precede the verb, the verb is a closed set —
	// matching any \w+ unanchored would be far too loose.
	resultRe = regexp.MustCompile(`(?m)^.*?\b(ok|changed|failed|fatal|skipping)\b: \[([^\]]+)\](?: \(item=(.+?)\))?(?:: ([A-Z]+)!)? =>(?: \(item=(.+?)\) =>)? `)

	// FAILED - RETRYING: Wait for service (3 retries left).
	retryRe = regexp.MustCompile(`\bFAILED - RETRYING`)

	// Explicit level tags like "] - INFO:" or "- SEVERE:" — when a log
	// line declares its own level, we trust it over keyword guessing.
	levelTagRe = regexp.MustCompile(`-\s*(INFO|FINE|FINER|FINEST|CONFIG|DEBUG|TRACE|WARNING|WARN|SEVERE|ERROR)\s*:`)

	// "failed=0" in PLAY RECAP lines is good news, not an error —
	// neutralize zero-counts before keyword matching.
	zeroCountRe = regexp.MustCompile(`\b(failed|unreachable|rescued|ignored|skipped|errors?)=0\b`)

	errLineRe  = regexp.MustCompile(`(?i)\b(severe|errors?|fatal|failed|failure|panic|traceback|non-zero|exception|denied|refused|unreachable|timed[ -]?out|timeout|unauthorized|forbidden|not found|no such|unable to|cannot)\b`)
	warnLineRe = regexp.MustCompile(`(?i)\b(warn(ing)?s?|deprecat(ed|ion|ions)|retrying|skipp(ed|ing))\b`)
)

// summaryKeys are pulled to the top, in this order, when present.
var summaryKeys = []string{"msg", "rc", "changed", "skip_reason", "delta", "start", "end"}

// verbStatus maps a result-line verb to a status when there is no
// explicit "FAILED!"-style marker.
var verbStatus = map[string]string{
	"ok":       "OK",
	"changed":  "CHANGED",
	"skipping": "SKIPPED",
	"failed":   "FAILED",
	"fatal":    "FAILED",
}

// ParseAnsible parses one Ansible -vv task block: optional TASK/path
// header lines followed by a result line whose payload is the module's
// JSON dict. Anything it cannot recognize is an error — callers fall
// back to showing the input untouched.
func ParseAnsible(input string) (*ParsedTask, error) {
	input = ansiRe.ReplaceAllString(input, "")

	// FindStringSubmatchIndex gives byte offsets; we need them to know
	// where the JSON payload starts (right after the "=> " match ends).
	loc := resultRe.FindStringSubmatchIndex(input)
	if loc == nil {
		return nil, fmt.Errorf(`no ansible result line ("fatal: [host]: FAILED! => {...}") found`)
	}
	m := resultRe.FindStringSubmatch(input)

	task := &ParsedTask{Host: m[2], Status: m[4]}

	// The loop item appears either before the arrow (m[3]) or between
	// two arrows (m[5]), depending on the ansible version and module.
	task.Item = m[3]
	if task.Item == "" {
		task.Item = m[5]
	}

	if task.Status == "" {
		if s, ok := verbStatus[strings.ToLower(m[1])]; ok {
			task.Status = s
		} else {
			task.Status = strings.ToUpper(m[1])
		}
	}
	if tm := taskRe.FindStringSubmatch(input); tm != nil {
		task.Name = tm[1]
	}
	if pm := pathRe.FindStringSubmatch(input); pm != nil {
		task.Path = pm[1]
	}
	task.Retries = len(retryRe.FindAllString(input, -1))

	// Everything after "=> " is the payload. A Decoder reads exactly
	// one JSON value and ignores trailing text, so noise after the
	// closing brace doesn't break parsing. UseNumber keeps rc=1 as
	// the literal "1" instead of a float64.
	dec := json.NewDecoder(strings.NewReader(input[loc[1]:]))
	dec.UseNumber()
	var result map[string]any
	if err := dec.Decode(&result); err != nil {
		return nil, fmt.Errorf("parse result payload: %w", err)
	}

	task.buildFrom(result)
	task.deriveCause()
	return task, nil
}

// buildFrom reorganizes the raw result dict into summary fields and
// display sections, consuming known keys and rendering the rest
// generically so unknown modules still produce readable output.
func (t *ParsedTask) buildFrom(result map[string]any) {
	consumed := make(map[string]bool)

	for _, key := range summaryKeys {
		v, ok := result[key]
		if !ok {
			continue
		}
		consumed[key] = true
		val := scalarString(v)
		t.Summary = append(t.Summary, Field{
			Key:   key,
			Value: val,
			Bad: (key == "rc" && val != "0") ||
				(key == "msg" && errLineRe.MatchString(zeroCountRe.ReplaceAllString(val, ""))),
		})
	}

	// The command that ran, prettified: one flag per line so a giant
	// one-liner becomes a readable invocation.
	if v, ok := result["cmd"]; ok {
		consumed["cmd"] = true
		if lines := commandLines(v); len(lines) > 0 {
			t.addSection("command", lines, false)
		}
	}

	// stderr/stdout: prefer the pre-split *_lines arrays and drop the
	// duplicate string form. classify=true colors these by severity.
	for _, name := range []string{"stderr", "stdout", "module_stderr", "module_stdout"} {
		consumed[name], consumed[name+"_lines"] = true, true

		lines := stringSlice(result[name+"_lines"])
		if lines == nil {
			lines = toLines(result[name])
		}
		if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
			continue // don't render empty sections
		}
		t.addSection(name, lines, true)
	}

	// Ansible attaches warnings/deprecations as arrays; every entry is
	// warn-level by definition, whatever words it contains.
	for _, name := range []string{"warnings", "deprecations"} {
		consumed[name] = true
		sec := Section{Title: name}
		for _, text := range stringSlice(result[name]) {
			sec.Lines = append(sec.Lines, Line{Text: text, Level: "warn"})
		}
		if len(sec.Lines) > 0 {
			t.Sections = append(t.Sections, sec)
		}
	}

	// Whatever is left: scalars join the summary, structures become
	// pretty-printed sections. Sorted for deterministic output — map
	// iteration order is random in Go. "invocation" (the module's echo
	// of its own arguments) is pure noise, so it always renders last.
	var rest []string
	hasInvocation := false
	for k := range result {
		if consumed[k] {
			continue
		}
		if k == "invocation" {
			hasInvocation = true
			continue
		}
		rest = append(rest, k)
	}
	sort.Strings(rest)
	if hasInvocation {
		rest = append(rest, "invocation")
	}

	for _, k := range rest {
		switch v := result[k].(type) {
		case map[string]any, []any:
			if b, err := json.MarshalIndent(v, "", "  "); err == nil {
				t.addSection(k, strings.Split(string(b), "\n"), false)
			}
		default:
			t.Summary = append(t.Summary, Field{Key: k, Value: scalarString(v)})
		}
	}
}

// deriveCause picks the single most likely root cause for a failed
// task: the first error line in stderr/stdout, else a bad msg, else
// any other bad summary field.
func (t *ParsedTask) deriveCause() {
	if t.Status != "FAILED" && t.Status != "UNREACHABLE" {
		return
	}

	for _, sec := range t.Sections {
		switch sec.Title {
		case "stderr", "stdout", "module_stderr", "module_stdout":
			for _, ln := range sec.Lines {
				if ln.Level == "error" {
					t.Cause = &Cause{Text: strings.TrimSpace(ln.Text), Source: sec.Title}
					return
				}
			}
		}
	}

	for _, f := range t.Summary {
		if f.Bad && f.Key == "msg" {
			t.Cause = &Cause{Text: f.Value, Source: "msg"}
			return
		}
	}
	for _, f := range t.Summary {
		if f.Bad {
			t.Cause = &Cause{Text: f.Key + " = " + f.Value, Source: f.Key}
			return
		}
	}
}

func (t *ParsedTask) addSection(title string, lines []string, classify bool) {
	sec := Section{Title: title}
	for _, text := range lines {
		level := ""
		if classify {
			level = classifyLine(text)
		}
		sec.Lines = append(sec.Lines, Line{Text: text, Level: level})
	}
	t.Sections = append(t.Sections, sec)
}

// classifyLine decides a line's severity. Lines that declare their own
// level (e.g. "] - INFO:") are trusted outright — this prevents an
// INFO line that merely *mentions* "not found" from being flagged.
// Only untagged lines fall back to keyword matching, warn checked
// first so "FAILED - RETRYING" reads as a warning, not an error.
func classifyLine(s string) string {
	if m := levelTagRe.FindStringSubmatch(s); m != nil {
		switch m[1] {
		case "SEVERE", "ERROR":
			return "error"
		case "WARNING", "WARN":
			return "warn"
		default:
			return ""
		}
	}

	s = zeroCountRe.ReplaceAllString(s, "")
	switch {
	case warnLineRe.MatchString(s):
		return "warn"
	case errLineRe.MatchString(s):
		return "error"
	default:
		return ""
	}
}

// Text renders the task as plain text — the CLI's view, and what the
// web UI's Copy button copies.
func (t *ParsedTask) Text() string {
	var b strings.Builder

	if t.Name != "" {
		fmt.Fprintf(&b, "TASK    %s\n", t.Name)
	}
	if t.Path != "" {
		fmt.Fprintf(&b, "PATH    %s\n", t.Path)
	}
	fmt.Fprintf(&b, "HOST    %s\n", t.Host)
	if t.Item != "" {
		fmt.Fprintf(&b, "ITEM    %s\n", t.Item)
	}
	fmt.Fprintf(&b, "STATUS  %s\n", t.Status)
	if t.Retries > 0 {
		fmt.Fprintf(&b, "RETRIES %d\n", t.Retries)
	}
	if t.Cause != nil {
		fmt.Fprintf(&b, "\nCAUSE   %s  (from %s)\n", t.Cause.Text, t.Cause.Source)
	}

	if len(t.Summary) > 0 {
		b.WriteString("\n")
		for _, f := range t.Summary {
			fmt.Fprintf(&b, "%-12s %s\n", f.Key, f.Value)
		}
	}

	for _, sec := range t.Sections {
		fmt.Fprintf(&b, "\n──── %s ────\n", sec.Title)
		for _, ln := range sec.Lines {
			b.WriteString(ln.Text)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// commandLines prettifies a cmd value: argv arrays stay one element
// per line; shell strings are split on newlines, then each " --flag"
// moves to its own indented line so long invocations become readable.
// Heuristic caveat: a literal " --" inside a quoted value would split
// too — acceptable for a display-only view.
func commandLines(v any) []string {
	if arr, ok := v.([]any); ok {
		return stringSlice(arr)
	}

	raw := strings.TrimSpace(scalarString(v))
	if raw == "" {
		return nil
	}

	var out []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, " --")
		out = append(out, parts[0])
		for _, p := range parts[1:] {
			out = append(out, "  --"+p)
		}
	}
	return out
}

// scalarString renders a single JSON value as display text. This is a
// type switch: v's concrete type picks the branch, and x is already
// converted inside each case.
func scalarString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case json.Number:
		return x.String()
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", x)
	}
}

// stringSlice converts a JSON array into []string, or nil if v is not
// an array. (JSON arrays always decode to []any, never []string.)
func stringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		out = append(out, scalarString(item))
	}
	return out
}

// toLines renders a value as display lines: strings split on newlines,
// arrays one element per line.
func toLines(v any) []string {
	if arr := stringSlice(v); arr != nil {
		return arr
	}
	s := strings.TrimSpace(scalarString(v))
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
