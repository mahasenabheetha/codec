package logview

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Build and infrastructure tools' output (decision 99, T5): Docker
// BuildKit, Terraform and OpenTofu, Helm, Maven and Gradle, npm, yarn
// and pnpm, dotnet and MSBuild, compilers, go test and pytest. Most of
// their lines are plain text; these say more: a level a level word
// alone wouldn't give (npm ERR!, "error CS0103", "--- FAIL"), a step
// (a BuildKit step, a Maven module, a Gradle task), and lines that go
// on with the one before (a Terraform error box, an npm error, a failed
// test's output).

// buildMark is what a build tool's line says.
type buildMark struct {
	level  Level
	raw    string    // the level as written, when the tool has a word of its own
	msg    string    // the message, when it isn't the line as written
	step   string    // the step this line is in (BuildKit) …
	starts string    // … or starts, lasting until the next (Maven, Gradle)
	tool   bool      // the tool's bookkeeping: output, to fold
	block  blockKind // the lines that go on with it
	took   float64   // seconds a step took, as the tool reports it
}

// tookKey is the field a step's own time is kept in (BuildKit's
// "DONE 3.2s"), in seconds, for the Steps view when the log has no times.
const tookKey = "step.took"

// buildState is what reading a build's lines remembers.
type buildState struct {
	kitSteps map[string]string // BuildKit: "#5" → "[2/7] RUN make"
}

var (
	kitLine    = regexp.MustCompile(`^#(\d+) (.*)$`)
	kitStep    = regexp.MustCompile(`^\[[^\]]+\] .+`)
	kitTime    = regexp.MustCompile(`^\d+\.\d+ (.*)$`)
	kitDone    = regexp.MustCompile(`^DONE (\d+(?:\.\d+)?)s$`)
	mavenBuild = regexp.MustCompile(`^\[INFO\] Building ([\w.-]+) [\w.+-]+(?:\s+\[\d+/\d+\])?$`)
	mavenFail  = regexp.MustCompile(`^\[INFO\] (.+ (?:FAILURE|SKIPPED) \[.*\])$`)
	gradleTask = regexp.MustCompile(`^> Task (:\S+)(?: (FAILED|UP-TO-DATE|NO-SOURCE|SKIPPED|FROM-CACHE))?$`)
	// MSBuild, dotnet, tsc, gcc, clang, javac: "x.cs(4,2): error CS0103:",
	// "x.c:3:5: error:", "x.java:42: error:".
	codedIssue = regexp.MustCompile(`\b(error|warning) [A-Z]{1,6}\d{1,6}\s*:|: (?:fatal )?(error|warning): `)
	pytestHead = regexp.MustCompile(`^_{3,} (.+?) _{3,}$`)
	pytestSum  = regexp.MustCompile(`^=+ (.*\b(?:failed|error|errors)\b.*) =+$`)
	pytestFail = regexp.MustCompile(`^(?:FAILED|ERROR) \S+::\S+`)
	goFail     = regexp.MustCompile(`^(?:--- FAIL: \S+|FAIL\s+\S+|FAIL)(?:\s|$)`)
)

// readBuild reads a line of a build tool's output; ok is false for a
// line none of them marks. Each pattern is behind a cheap test of the
// line's start, as most lines are none of them.
func (b *buildState) readBuild(s string) (buildMark, bool) {
	switch c := s[0]; {
	case c == '#':
		if m := kitLine.FindStringSubmatch(s); m != nil {
			return b.buildKit(m[1], m[2]), true
		}
	case s == "╷":
		return buildMark{block: tfBlock, tool: true}, true
	case strings.HasPrefix(s, "npm ERR! "), strings.HasPrefix(s, "npm error "):
		// The word is kept as the raw level: the lines that go on with it
		// carry the same one (npm 7–9 write "ERR!", npm 10 "error").
		word, rest, _ := strings.Cut(s[4:], " ")
		return buildMark{level: Error, raw: word, msg: rest, block: npmBlock}, true
	case strings.HasPrefix(s, "error ") && len(s) > 6 && s[6] != ' ':
		return buildMark{level: Error, msg: s[6:]}, true // yarn
	case strings.HasPrefix(s, "warning ") && len(s) > 8 && s[8] != ' ':
		return buildMark{level: Warn, msg: s[8:]}, true
	case strings.HasPrefix(strings.TrimLeft(s, " "), "ERR_PNPM_"):
		return buildMark{level: Error, msg: strings.TrimSpace(s)}, true
	case strings.HasPrefix(s, "e: "):
		return buildMark{level: Error, msg: s[3:]}, true // kotlinc
	case strings.HasPrefix(s, "w: "):
		return buildMark{level: Warn, msg: s[3:]}, true
	case s == "Build FAILED." || strings.HasPrefix(s, "BUILD FAILED") || strings.HasPrefix(s, "FAILURE: "):
		return buildMark{level: Error}, true
	case strings.HasPrefix(s, "=== RUN ") || strings.HasPrefix(s, "=== PAUSE ") || strings.HasPrefix(s, "=== CONT "):
		return buildMark{tool: true}, true
	case c == '-' || c == 'F':
		if goFail.MatchString(s) {
			return buildMark{level: Error, block: indentBlock}, true
		}
		if pytestFail.MatchString(s) {
			return buildMark{level: Error}, true
		}
	case c == 'E' && pytestFail.MatchString(s):
		return buildMark{level: Error}, true
	case c == '_':
		if m := pytestHead.FindStringSubmatch(s); m != nil && !strings.Contains(m[1], " ") {
			// "____ test_add ____": a failed test's report, up to the next.
			return buildMark{level: Error, msg: m[1], block: testBlock}, true
		}
	case c == '=':
		if m := pytestSum.FindStringSubmatch(s); m != nil {
			return buildMark{level: Error, msg: m[1]}, true
		}
	case strings.HasPrefix(s, "[INFO] "):
		if m := mavenBuild.FindStringSubmatch(s); m != nil {
			return buildMark{level: Info, msg: s[len("[INFO] "):], starts: m[1]}, true
		}
		if s == "[INFO] BUILD FAILURE" {
			return buildMark{level: Error, msg: "BUILD FAILURE"}, true
		}
		if m := mavenFail.FindStringSubmatch(s); m != nil {
			return buildMark{level: Error, msg: m[1]}, true // "[INFO] core ...... FAILURE [ 1.2 s]"
		}
	case strings.HasPrefix(s, "> Task "):
		if m := gradleTask.FindStringSubmatch(s); m != nil {
			mk := buildMark{starts: m[1], tool: m[2] != "" && m[2] != "FAILED"}
			if m[2] == "FAILED" {
				mk.level = Error
			}
			return mk, true
		}
	}
	if strings.Contains(s, "error") || strings.Contains(s, "warning") {
		if m := codedIssue.FindStringSubmatch(s); m != nil {
			mk := buildMark{level: Warn}
			if m[1]+m[2] == "error" {
				mk.level = Error
			}
			return mk, true
		}
	}
	return buildMark{}, false
}

// buildKit reads a BuildKit progress line, "#5 …": its steps run side by
// side, so each line names its step by number.
func (b *buildState) buildKit(id, rest string) buildMark {
	if b.kitSteps == nil {
		b.kitSteps = map[string]string{}
	}
	if kitStep.MatchString(rest) {
		if _, ok := b.kitSteps[id]; !ok {
			b.kitSteps[id] = rest
		}
		return buildMark{step: b.kitSteps[id], msg: rest, tool: true}
	}
	step := b.kitSteps[id]
	if step == "" {
		step = "#" + id
	}
	switch {
	case strings.HasPrefix(rest, "ERROR"):
		return buildMark{step: step, level: Error, msg: rest}
	case rest == "CACHED":
		return buildMark{step: step, msg: rest, tool: true}
	}
	if m := kitDone.FindStringSubmatch(rest); m != nil {
		took, _ := strconv.ParseFloat(m[1], 64)
		return buildMark{step: step, msg: rest, tool: true, took: took}
	}
	if m := kitTime.FindStringSubmatch(rest); m != nil {
		// "#5 0.312 + make": the step's own output, after its time.
		var e Entry
		readText(m[1], time.Time{}, &e)
		return buildMark{step: step, level: e.Level, msg: m[1]}
	}
	return buildMark{step: step, msg: rest, tool: true} // resolve, transferring context, exporting layers …
}

// maxTestLines bounds a failed test's report.
const maxTestLines = 400

// inBuildBlock reports whether s goes on with a build tool's block.
func inBuildBlock(p *pending, s string) bool {
	if _, ok := readCI(s); ok {
		return false // a CI mark is the pipeline's, whatever block it is in
	}
	switch p.block {
	case tfBlock:
		return strings.HasPrefix(s, "│") || s == "╵"
	case npmBlock:
		return strings.HasPrefix(s, "npm "+p.e.RawLevel+" ")
	case indentBlock:
		return s[0] == ' ' || s[0] == '\t'
	case testBlock:
		// Up to the next test's report or the summary, and not for ever:
		// a stray "___ x ___" mustn't take the rest of the log.
		return p.e.Lines < maxTestLines && !pytestHead.MatchString(s) && !strings.HasPrefix(s, "==")
	}
	return false
}

// buildBlockLine is a block's line as its entry's message shows it, and
// what it says of the entry: Terraform's box drawing comes off, and its
// "│ Error: …" names the entry.
func buildBlockLine(p *pending, s string) (string, bool) {
	switch p.block {
	case tfBlock:
		if s == "╵" {
			return "", false
		}
		s = strings.TrimPrefix(strings.TrimPrefix(s, "│"), " ")
		if p.e.Level == LevelNone {
			if rest, ok := strings.CutPrefix(s, "Error: "); ok {
				p.e.Level, p.e.RawLevel, p.e.Tool = Error, "Error", false
				return rest, true
			}
			if rest, ok := strings.CutPrefix(s, "Warning: "); ok {
				p.e.Level, p.e.RawLevel, p.e.Tool = Warn, "Warning", false
				return rest, true
			}
		}
		return s, true
	case npmBlock:
		_, rest, _ := strings.Cut(s[4:], " ")
		return rest, true
	}
	return s, true
}

// goTestEvent reads go test -json's events: {"Action":"output",
// "Package":…,"Test":…,"Output":…}. The package is the step; a failure
// is an error; the test runner's own events are output.
func goTestEvent(e *Entry) {
	var action, pkg, test, out, elapsed string
	for _, f := range e.Fields {
		switch f.Key {
		case "Action":
			action = f.Text
		case "Package":
			pkg = f.Text
		case "Test":
			test = f.Text
		case "Output":
			out = f.Text
		case "Elapsed":
			elapsed = f.Text
		}
	}
	if action == "" || pkg == "" {
		return
	}
	e.Step = pkg
	name := test
	if name == "" {
		name = pkg
	}
	switch action {
	case "output":
		e.Message = strings.TrimRight(out, "\n")
	case "fail":
		e.Level, e.Message = Error, "FAIL "+name
	case "pass", "skip":
		e.Message = strings.ToUpper(action) + " " + name
	default: // run, pause, cont, start, bench
		e.Tool, e.Message = true, action+" "+name
	}
	if elapsed != "" && action != "output" {
		e.Message += " (" + elapsed + "s)"
	}
}
