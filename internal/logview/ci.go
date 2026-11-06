package logview

import (
	"regexp"
	"strings"
)

// Pipeline logs (decision 99, T4): Azure DevOps, GitHub Actions, GitLab
// CI and Jenkins mark where their steps start and end and which lines
// are errors and warnings, with commands of their own in the log. The
// reader follows those marks: each entry gets the step it was logged
// in, an error or warning mark becomes the entry's level, a location
// becomes its caller, and the tool's own bookkeeping lines are output,
// folded with the rest.

type ciKind uint8

const (
	ciNone         ciKind = iota
	ciSectionStart        // a step starts: Azure's "Starting:", a GitLab section, a Jenkins stage
	ciSectionEnd          // … and ends (the line still belongs to it)
	ciGroupStart          // a collapsible group: a step when nothing else names one (GitHub)
	ciGroupEnd            // a group's end: the tool's own line
	ciLine                // a line with a level or location, or the tool's own
)

// ciMark is what a CI command on a line says.
type ciMark struct {
	kind   ciKind
	name   string // the step's name, for a start
	level  Level
	msg    string // the entry's message, the command taken off
	caller string // "file:line[:col]", from an issue's location
	tool   bool   // the CI tool's own line: output, to fold
}

var (
	// GitLab writes its sections before a carriage return that hides
	// them on a terminal: "section_start:1700000000:build_script\r\x1b[0K
	// Header". They are read before the redraw takes them away.
	gitlabSection = regexp.MustCompile(`section_(start|end):\d+:([^\s\[\r]+)(?:\[[^\]\r]*\])?\r?`)

	// Azure DevOps and GitHub Actions: "##[error]text".
	hashCmd = regexp.MustCompile(`^##\[(\w+)\](.*)$`)
	// Azure DevOps logging commands: "##vso[task.logissue type=error;
	// sourcepath=x;linenumber=3;]text".
	vsoCmd = regexp.MustCompile(`^##vso\[([\w.]+)([^\]]*)\](.*)$`)
	// GitHub Actions workflow commands, when a log shows them as written:
	// "::error file=x,line=3,col=2::text".
	ghCmd = regexp.MustCompile(`^::([\w-]+)(?: ([^:]*))?::(.*)$`)
	// Jenkins pipeline steps: "[Pipeline] { (Build)" opens a stage.
	jenkinsCmd   = regexp.MustCompile(`^\[Pipeline\] (.*)$`)
	jenkinsStage = regexp.MustCompile(`^\{ \((.+)\)$`)
)

// readCI reads a CI command at the start of a line's payload.
func readCI(s string) (ciMark, bool) {
	if len(s) < 3 || (s[0] != '#' && s[0] != ':' && s[0] != '[') {
		return ciMark{}, false
	}
	if m := vsoCmd.FindStringSubmatch(s); m != nil {
		if m[1] != "task.logissue" {
			return ciMark{kind: ciLine, msg: s, tool: true}, true // setvariable, uploadfile, …
		}
		p := params(m[2], ';')
		return ciMark{kind: ciLine, level: issueLevel(p["type"]), msg: m[3], caller: location(p["sourcepath"], p["linenumber"], p["columnnumber"])}, true
	}
	if m := hashCmd.FindStringSubmatch(s); m != nil {
		cmd, text := strings.ToLower(m[1]), m[2]
		switch cmd {
		case "section":
			if name, ok := strings.CutPrefix(text, "Starting: "); ok {
				return ciMark{kind: ciSectionStart, name: strings.TrimSpace(name), msg: text, tool: true}, true
			}
			if _, ok := strings.CutPrefix(text, "Finishing: "); ok {
				return ciMark{kind: ciSectionEnd, msg: text, tool: true}, true
			}
			return ciMark{kind: ciLine, msg: text, tool: true}, true
		case "group":
			return ciMark{kind: ciGroupStart, name: strings.TrimSpace(text), msg: text}, true
		case "endgroup":
			return ciMark{kind: ciGroupEnd, msg: s, tool: true}, true
		case "error", "warning", "debug", "notice", "info":
			return ciMark{kind: ciLine, level: issueLevel(cmd), msg: text, tool: cmd == "debug"}, true
		case "command":
			return ciMark{kind: ciLine, msg: text}, true
		}
		return ciMark{}, false
	}
	if m := ghCmd.FindStringSubmatch(s); m != nil {
		switch cmd := m[1]; cmd {
		case "group":
			return ciMark{kind: ciGroupStart, name: strings.TrimSpace(m[3]), msg: m[3]}, true
		case "endgroup":
			return ciMark{kind: ciGroupEnd, msg: s, tool: true}, true
		case "error", "warning", "notice", "debug":
			p := params(m[2], ',')
			msg := m[3]
			if t := p["title"]; t != "" {
				msg = t + ": " + msg
			}
			return ciMark{kind: ciLine, level: issueLevel(cmd), msg: msg, caller: location(p["file"], p["line"], p["col"]), tool: cmd == "debug"}, true
		case "add-mask", "set-output", "set-env", "add-path", "save-state", "stop-commands", "echo":
			return ciMark{kind: ciLine, msg: s, tool: true}, true
		}
		return ciMark{}, false
	}
	if m := jenkinsCmd.FindStringSubmatch(s); m != nil {
		if st := jenkinsStage.FindStringSubmatch(m[1]); st != nil {
			return ciMark{kind: ciSectionStart, name: st[1], msg: "Stage " + st[1], tool: true}, true
		}
		if m[1] == "// stage" {
			return ciMark{kind: ciSectionEnd, msg: s, tool: true}, true
		}
		return ciMark{kind: ciLine, msg: s, tool: true}, true // [Pipeline] sh, node, }, …
	}
	return ciMark{}, false
}

// params reads "k=v" pairs separated by sep: Azure's "type=error;
// sourcepath=x;" and GitHub's "file=x,line=3".
func params(s string, sep byte) map[string]string {
	out := map[string]string{}
	for _, kv := range strings.Split(strings.TrimSpace(s), string(sep)) {
		if k, v, ok := strings.Cut(kv, "="); ok {
			out[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
		}
	}
	return out
}

func issueLevel(t string) Level {
	switch strings.ToLower(t) {
	case "error":
		return Error
	case "warning":
		return Warn
	case "debug":
		return Debug
	case "notice", "info":
		return Info
	}
	return LevelNone
}

// location writes an issue's file and line as a caller ("x.ts:3:2"),
// which the Log viewer shows as a link to the source.
func location(file, line, col string) string {
	if file == "" || !allDigits(line) {
		return file
	}
	if allDigits(col) {
		return file + ":" + line + ":" + col
	}
	return file + ":" + line
}

// ciSteps follows a log's steps as it is read. Sections (Azure's tasks,
// GitLab's sections, Jenkins' stages) nest; a group (GitHub's "Run …")
// names the step only outside any section, and lasts until the next one.
type ciSteps struct {
	sections []string
	group    string
}

func (c *ciSteps) current() string {
	if n := len(c.sections); n > 0 {
		return c.sections[n-1]
	}
	return c.group
}

func (c *ciSteps) start(name string) { c.sections = append(c.sections, name) }

// end closes the named section, or the innermost one when name is ""
// (Azure's "Finishing:", Jenkins' "// stage").
func (c *ciSteps) end(name string) {
	for i := len(c.sections) - 1; i >= 0; i-- {
		if name == "" || c.sections[i] == name {
			c.sections = c.sections[:i]
			return
		}
	}
}

// gitlab applies the GitLab section marks of a line (colour codes
// removed, carriage returns kept). A section of GitLab's own is named
// by its id (step_script); one the job made, by the header after its
// start.
func (c *ciSteps) gitlab(clean, header string) {
	for _, m := range gitlabSection.FindAllStringSubmatch(clean, -1) {
		if m[1] == "end" {
			// Sections close in order; an end carries only the id.
			c.end("")
			continue
		}
		name := strings.TrimSpace(header)
		if gitlabOwn[m[2]] || strings.HasPrefix(m[2], "step_") || name == "" || len(name) > 80 {
			name = m[2]
		}
		c.start(name)
	}
}

// gitlabOwn are the sections GitLab's runner writes for every job.
var gitlabOwn = map[string]bool{
	"prepare_executor": true, "prepare_script": true, "get_sources": true, "restore_cache": true,
	"download_artifacts": true, "build_script": true, "after_script": true, "archive_cache": true,
	"archive_cache_on_failure": true, "upload_artifacts_on_success": true, "upload_artifacts_on_failure": true,
	"cleanup_file_variables": true, "resolve_secrets": true,
}
