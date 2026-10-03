package ansiblelog

import (
	"regexp"
	"strings"
	"time"
)

// Log is a cleaned log cut into Ansible runs and the output around them.
type Log struct {
	Lines  []Line
	Blocks []Block // in order, covering every line exactly once
}

// Block is a stretch of lines: one Ansible run, or other output (CI
// steps, shell commands, other tools) when Run is nil.
type Block struct {
	From, To int    // line indexes, [From, To)
	Title    string // the CI group it sits in ("step_script"), if any
	Run      *Run
}

// Run is one ansible-playbook run as its output shows it.
type Run struct {
	Playbook string // from "PLAYBOOK: site.yml" (-v and up), if printed
	JSON     bool   // the json stdout callback: one document, no headers
	Plays    []Play
	Recap    []int       // the host rows after PLAY RECAP (line indexes)
	Notes    []int       // [WARNING], [DEPRECATION WARNING] and ERROR! lines
	Complete bool        // ended with its recap
	Hosts    []HostRecap // the recap read by Read
}

// Play is a PLAY header and the tasks under it.
type Play struct {
	Line  int // header line index; -1 when tasks came without one
	Name  string
	Tasks []Task
}

// Task is a TASK or RUNNING HANDLER header and the lines after it up
// to the next header: its results, and any stray lines in between.
type Task struct {
	Line     int    // header line index
	Name     string // without the role
	Role     string // "role : name" headers
	Handler  bool
	From, To int           // body line indexes, [From, To)
	Path     string        // "task path:" (-vv): the file defining it
	PathLine int           // and the line there
	Duration time.Duration // from a timer callback (profile_tasks), if any
	Results  []Result      // filled in by Read
	Other    []int         // body lines that aren't results (-vvv, interleaved output)
}

var (
	header   = regexp.MustCompile(`^(PLAY|TASK|RUNNING HANDLER) \[(.*)\](?: \**)?\s*$`)
	recapHdr = regexp.MustCompile(`^PLAY RECAP\b`)
	recapRow = regexp.MustCompile(`^\S.*?\s*:\s*ok=\d+\s+changed=\d+\s+unreachable=\d+\s+failed=\d+`)
	playbook = regexp.MustCompile(`^PLAYBOOK: (.+?) \*+\s*$`)
	note     = regexp.MustCompile(`^(?:\[(?:WARNING|DEPRECATION WARNING)\]|ERROR!)`)
	// What may sit just before a PLAY and still belong to its run.
	preamble = regexp.MustCompile(`^(?:PLAYBOOK: |\[(?:WARNING|DEPRECATION WARNING)\]|Using .+ as config file|\s*$)`)
	// Keys at the top of the json stdout callback's document.
	jsonTop = regexp.MustCompile(`^\s*"(?:custom_stats|global_custom_stats|plays|stats)":`)
)

// Segment finds the runs in cleaned lines. Only Ansible's own headers
// decide what is Ansible; everything else is other output, or a stray
// line inside the task it appears under.
func Segment(lines []Line) *Log {
	s := &segmenter{log: &Log{Lines: lines}}
	for i := 0; i < len(lines); i++ {
		i = s.line(i)
	}
	if s.run != nil {
		s.endRun(len(lines), len(s.run.Recap) > 0)
	}
	s.endOther(len(lines))
	return s.log
}

type segmenter struct {
	log        *Log
	run        *Run
	runFrom    int
	runWrap    string
	lastHeader int // the run's last header line; a cut run keeps it
	inRecap    bool
	other      int    // start of the other output being collected
	title      string // the open CI group
	lastTitle  string // the group the current block started in
}

// line handles lines[i] and returns the last index it consumed.
func (s *segmenter) line(i int) int {
	l := s.log.Lines[i]
	switch l.Kind {
	case GroupStart:
		if s.run == nil {
			s.endOther(i)
			s.lastTitle = l.Text
		}
		s.title = l.Text
		return i
	case GroupEnd:
		// A step ending closes its run, even one that never printed
		// its recap; the marker goes with the output after the run.
		if s.run != nil {
			s.endRun(i, len(s.run.Recap) > 0)
		}
		s.endOther(i + 1)
		s.title, s.lastTitle = "", ""
		return i
	}
	t := l.Text
	if s.inRecap {
		switch {
		case recapRow.MatchString(t):
			s.run.Recap = append(s.run.Recap, i)
			return i
		case strings.TrimSpace(t) == "":
			return i
		}
		s.endRun(i, len(s.run.Recap) > 0)
	}
	if m := header.FindStringSubmatch(t); m != nil {
		s.header(i, m[1], m[2], l.Wrap)
		return i
	}
	if recapHdr.MatchString(t) {
		s.startRun(i, l.Wrap)
		s.closeTask(i)
		s.lastHeader = i
		s.inRecap = true
		return i
	}
	if s.run != nil {
		if note.MatchString(t) {
			s.run.Notes = append(s.run.Notes, i)
		}
		return i
	}
	if strings.TrimSpace(t) == "{" {
		if end, ok := s.jsonRun(i); ok {
			return end
		}
	}
	return i
}

func (s *segmenter) header(i int, kind, name, wrap string) {
	s.startRun(i, wrap)
	r := s.run
	s.closeTask(i)
	s.lastHeader = i
	if kind == "PLAY" {
		r.Plays = append(r.Plays, Play{Line: i, Name: name})
		return
	}
	if len(r.Plays) == 0 {
		r.Plays = append(r.Plays, Play{Line: -1})
	}
	t := Task{Line: i, Name: name, Handler: kind == "RUNNING HANDLER", From: i + 1, To: -1}
	if role, rest, ok := strings.Cut(name, " : "); ok {
		t.Role, t.Name = role, rest
	}
	p := &r.Plays[len(r.Plays)-1]
	p.Tasks = append(p.Tasks, t)
}

// startRun opens a run at line i unless one is open, taking along the
// lines just before it that belong to it (PLAYBOOK:, warnings).
func (s *segmenter) startRun(i int, wrap string) {
	if s.run != nil {
		return
	}
	from := i
	for j := i - 1; j >= s.other; j-- {
		t := s.log.Lines[j].Text
		if s.log.Lines[j].Kind != Text || !preamble.MatchString(t) {
			break
		}
		if strings.TrimSpace(t) != "" {
			from = j
		}
	}
	s.endOther(from)
	s.run, s.runFrom, s.runWrap, s.inRecap = &Run{}, from, wrap, false
	for j := from; j < i; j++ {
		t := s.log.Lines[j].Text
		if m := playbook.FindStringSubmatch(t); m != nil {
			s.run.Playbook = m[1]
		} else if note.MatchString(t) {
			s.run.Notes = append(s.run.Notes, j)
		}
	}
}

// endRun closes the open run before line end. A run cut short (no
// recap) gives back its trailing lines that can't be Ansible's: blank
// ones, and, for a wrapped run, lines without its wrapper.
func (s *segmenter) endRun(end int, complete bool) {
	if s.run == nil {
		return
	}
	if !complete {
		for end > max(s.runFrom, s.lastHeader)+1 {
			l := s.log.Lines[end-1]
			if l.Kind == Text && strings.TrimSpace(l.Text) != "" && (s.runWrap == "" || l.Wrap == s.runWrap) {
				break
			}
			end--
		}
	}
	s.closeTask(end)
	s.run.Complete = complete
	s.log.Blocks = append(s.log.Blocks, Block{From: s.runFrom, To: end, Title: s.title, Run: s.run})
	s.run, s.inRecap, s.other = nil, false, end
	s.lastTitle = s.title
}

// closeTask ends the open task's body at line end.
func (s *segmenter) closeTask(end int) {
	if s.run == nil || len(s.run.Plays) == 0 {
		return
	}
	p := &s.run.Plays[len(s.run.Plays)-1]
	if n := len(p.Tasks); n > 0 && p.Tasks[n-1].To < 0 {
		p.Tasks[n-1].To = max(end, p.Tasks[n-1].From)
	}
}

// endOther closes the other output collected before line end.
func (s *segmenter) endOther(end int) {
	if end > s.other {
		// Blank lines between CI steps aren't worth a block of their own.
		if n := len(s.log.Blocks); n > 0 && s.log.Blocks[n-1].Run == nil && s.blank(s.other, end) {
			s.log.Blocks[n-1].To = end
		} else {
			s.log.Blocks = append(s.log.Blocks, Block{From: s.other, To: end, Title: s.lastTitle})
		}
	}
	s.other = max(s.other, end)
	s.lastTitle = s.title
}

func (s *segmenter) blank(from, to int) bool {
	for _, l := range s.log.Lines[from:to] {
		if l.Kind != Text || strings.TrimSpace(l.Text) != "" {
			return false
		}
	}
	return true
}

// jsonRun recognises the json stdout callback's document starting at
// line i and records it as a run. It returns the document's last line.
func (s *segmenter) jsonRun(i int) (int, bool) {
	lines := s.log.Lines
	top := false
	for j := i + 1; j < min(len(lines), i+12) && !top; j++ {
		top = jsonTop.MatchString(lines[j].Text)
	}
	if !top {
		return 0, false
	}
	end, _, ok := BraceEnd(lines, i, strings.IndexByte(lines[i].Text, '{'))
	if !ok {
		return 0, false
	}
	s.endOther(i)
	s.log.Blocks = append(s.log.Blocks, Block{From: i, To: end + 1, Title: s.title, Run: &Run{JSON: true, Complete: true}})
	s.other = end + 1
	return end, true
}

// BraceEnd finds where the JSON value opening at lines[line].Text[col]
// closes, skipping braces inside strings. It returns the line and the
// byte just past the closing brace or bracket.
func BraceEnd(lines []Line, line, col int) (endLine, endCol int, ok bool) {
	depth, inStr, esc := 0, false, false
	for i := line; i < len(lines); i++ {
		t := lines[i].Text
		start := 0
		if i == line {
			start = col
		}
		for j := start; j < len(t); j++ {
			c := t[j]
			switch {
			case esc:
				esc = false
			case inStr:
				if c == '\\' {
					esc = true
				} else if c == '"' {
					inStr = false
				}
			case c == '"':
				inStr = true
			case c == '{' || c == '[':
				depth++
			case c == '}' || c == ']':
				depth--
				if depth == 0 {
					return i, j + 1, true
				}
			}
		}
	}
	return 0, 0, false
}
