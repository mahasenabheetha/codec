package commands

import (
	"path"
	"strings"
)

// Part is a piece of a command line and what it does.
type Part struct {
	Start, End int    // byte offsets in the line
	Kind       string // see the kinds below
	Text       string // as the command sees it (quotes removed)
	Summary    string // what it does; "" when codec can't say
	Danger     *Danger
	// For a flag: its first name as the reference writes it, so the
	// builder can find it; for an argument, the argument's name.
	Ref string
}

// The kinds of part.
const (
	KindTool     = "tool"     // the command itself: git, kubectl, ls
	KindCommand  = "command"  // a subcommand: commit, get, rollout restart
	KindFlag     = "flag"     // -m, --namespace=x
	KindValue    = "value"    // a flag's value given apart from it
	KindArg      = "arg"      // a positional argument
	KindEnd      = "end"      // --, the end of the options
	KindEnv      = "env"      // NAME=value before the command
	KindWrapper  = "wrapper"  // sudo, time, nohup, … and their options
	KindOp       = "operator" // |, &&, ||, ;, &
	KindRedirect = "redirect" // >, 2>&1, …
	KindTarget   = "target"   // a redirection's file
	KindComment  = "comment"
	KindUnknown  = "unknown" // a flag or command codec's reference doesn't have
)

// Segment is one command of the line: the parts from its first word to
// the operator after it.
type Segment struct {
	From, To int      // its parts: Parts[From:To]
	Tool     string   // "" when codec doesn't know it
	Path     []string // tool and subcommands: git, stash, push
	Summary  string   // what the command does
	Danger   *Danger  // the gravest of its command's and flags'
}

// Explanation is a whole line explained.
type Explanation struct {
	Parts    []Part
	Segments []Segment
}

// wrapper is a command that runs the command after it: its own options
// come first, then (for timeout) a fixed number of arguments.
type wrapper struct {
	summary string
	valued  string // its flags that take a value
	leading int    // arguments before the command (timeout's duration)
	assigns bool   // takes NAME=value pairs first (env)
}

var wrappers = map[string]wrapper{
	"sudo":    {summary: "runs the command as another user, root unless -u says otherwise", valued: "-u -g -C -D -h -p -r -t -U"},
	"doas":    {summary: "runs the command as another user, root unless -u says otherwise", valued: "-u -C"},
	"time":    {summary: "runs the command and prints how long it took", valued: "-f -o"},
	"nohup":   {summary: "runs the command so it keeps going after the terminal closes; output goes to nohup.out"},
	"nice":    {summary: "runs the command at a lower CPU priority", valued: "-n"},
	"ionice":  {summary: "runs the command with another disk priority", valued: "-c -n -p"},
	"env":     {summary: "runs the command with the environment changed", valued: "-u -C -S", assigns: true},
	"watch":   {summary: "runs the command again every few seconds and shows its output", valued: "-n -d"},
	"timeout": {summary: "runs the command and stops it after a time limit", valued: "-s -k", leading: 1},
	"exec":    {summary: "replaces the shell with the command", valued: "-a"},
	"command": {summary: "runs the command itself, not a function or alias of the same name"},
	"xargs":   {summary: "reads words from its input and runs the command with them as arguments", valued: "-I -n -P -L -d -s -a -E"},
}

var opSummaries = map[string]string{
	"|":  "passes this command's output to the next one as its input",
	"|&": "passes this command's output and errors to the next one",
	"&&": "runs the next command only if this one succeeds",
	"||": "runs the next command only if this one fails",
	";":  "then runs the next command, whatever this one did",
	"\n": "then runs the next command, whatever this one did",
	"&":  "runs this command in the background",
	";;": "ends a case branch",
}

// Explain says what every part of a command line does. It never fails:
// a command codec doesn't know is still split into its parts.
func Explain(line string) Explanation {
	tools, _ := load() // a pack that fails to load is caught by the tests
	var ex Explanation
	toks := Read(line)
	start := 0
	for i := 0; i <= len(toks); i++ {
		if i < len(toks) && toks[i].Kind != Op {
			continue
		}
		if i > start || i < len(toks) {
			ex.segment(toks[start:i], tools)
		}
		if i < len(toks) {
			t := toks[i]
			ex.Parts = append(ex.Parts, Part{Start: t.Start, End: t.End, Kind: KindOp, Text: t.Text, Summary: opSummaries[t.Text]})
		}
		start = i + 1
	}
	return ex
}

// segment explains one command's tokens.
func (ex *Explanation) segment(toks []Token, tools map[string]*Tool) {
	seg := Segment{From: len(ex.Parts)}
	add := func(t Token, kind, summary string) *Part {
		ex.Parts = append(ex.Parts, Part{Start: t.Start, End: t.End, Kind: kind, Text: t.Text, Summary: summary})
		return &ex.Parts[len(ex.Parts)-1]
	}
	i := 0
	// Redirections and comments can be anywhere; the rest is read in order.
	skip := func() bool {
		switch toks[i].Kind {
		case Redirect:
			r := toks[i]
			add(r, KindRedirect, redirectSummary(r.Text))
			if i+1 < len(toks) && toks[i+1].Kind == Word && !strings.HasSuffix(r.Text, "&1") && !strings.HasSuffix(r.Text, "&2") {
				i++
				add(toks[i], KindTarget, "")
			}
			i++
			return true
		case Comment:
			add(toks[i], KindComment, "a comment: the shell ignores it")
			i++
			return true
		}
		return false
	}
	for i < len(toks) {
		if skip() {
			continue
		}
		t := toks[i]
		if eq := strings.IndexByte(t.Text, '='); eq > 0 && isName(t.Text[:eq]) {
			add(t, KindEnv, "sets "+t.Text[:eq]+" for this command only")
			i++
			continue
		}
		w, ok := wrappers[t.Text]
		if !ok {
			break
		}
		add(t, KindWrapper, w.summary)
		i++
		leading := w.leading
		for i < len(toks) {
			if skip() {
				continue
			}
			s := toks[i].Text
			switch {
			case strings.HasPrefix(s, "-") && s != "-":
				add(toks[i], KindWrapper, "")
				i++
				if strings.Contains(" "+w.valued+" ", " "+s+" ") && i < len(toks) && toks[i].Kind == Word {
					add(toks[i], KindWrapper, "")
					i++
				}
				continue
			case w.assigns && strings.IndexByte(s, '=') > 0 && isName(s[:strings.IndexByte(s, '=')]):
				add(toks[i], KindEnv, "sets "+s[:strings.IndexByte(s, '=')]+" for the command")
				i++
				continue
			case leading > 0:
				add(toks[i], KindWrapper, "")
				leading--
				i++
				continue
			}
			break
		}
	}
	for i < len(toks) && skip() {
	}
	if i >= len(toks) {
		seg.To = len(ex.Parts)
		ex.Segments = append(ex.Segments, seg)
		return
	}

	name := toks[i].Text
	tool := tools[name]
	if tool == nil {
		tool = tools[path.Base(strings.ReplaceAll(name, `\`, "/"))] // /usr/bin/git, C:\…\kubectl.exe
	}
	if tool == nil {
		tool = tools[strings.TrimSuffix(strings.ToLower(path.Base(strings.ReplaceAll(name, `\`, "/"))), ".exe")]
	}
	if tool == nil {
		add(toks[i], KindUnknown, "codec's reference doesn't have this command")
		for i++; i < len(toks); i++ {
			if toks[i].Kind == Redirect || toks[i].Kind == Comment {
				skip()
				i--
				continue
			}
			add(toks[i], KindArg, "")
		}
		seg.To = len(ex.Parts)
		ex.Segments = append(ex.Segments, seg)
		return
	}

	seg.Tool, seg.Path = tool.Name, []string{tool.Name}
	cmd := &tool.Command
	chain := []*Command{cmd}
	tp := add(toks[i], KindTool, tool.Summary)
	tp.Danger = tool.Danger
	i++
	positional, afterDD := 0, false
	worst := tool.Danger
	note := func(d *Danger) {
		if d != nil && (worst == nil || worst.Level == "caution" && d.Level == "danger") {
			worst = d
		}
	}
	for i < len(toks) {
		if skip() {
			continue
		}
		t := toks[i]
		s := t.Text
		switch {
		case afterDD:
			ex.arg(t, cmd, &positional)
		case s == "--" && !t.Quoted:
			add(t, KindEnd, "ends the options: everything after it is an argument, even if it starts with -")
			afterDD = true
		case strings.HasPrefix(s, "-") && len(s) > 1:
			// The flag, and its value when that is the next word.
			n := ex.flag(toks, i, chain, len(chain) == 1)
			for _, p := range ex.Parts[len(ex.Parts)-n:] {
				note(p.Danger)
			}
			i += n - 1
		default:
			if positional == 0 {
				if sub := cmd.sub(s); sub != nil {
					cmd = sub
					chain = append(chain, sub)
					seg.Path = append(seg.Path, sub.Name)
					p := add(t, KindCommand, sub.Summary)
					p.Danger = sub.Danger
					note(sub.Danger)
					break
				}
			}
			ex.arg(t, cmd, &positional)
		}
		i++
	}
	seg.Summary, seg.Danger = cmd.Summary, worst
	seg.To = len(ex.Parts)
	ex.Segments = append(ex.Segments, seg)
}

// arg explains a positional argument by the command's arguments, in
// order; the last one that repeats takes the rest.
func (ex *Explanation) arg(t Token, cmd *Command, positional *int) {
	p := Part{Start: t.Start, End: t.End, Kind: KindArg, Text: t.Text}
	if n := len(cmd.Args); n > 0 {
		k := *positional
		if k >= n && cmd.Args[n-1].Repeat {
			k = n - 1
		}
		if k < n {
			a := cmd.Args[k]
			p.Ref, p.Summary = a.Name, a.Summary
			if c := choice(a.Choices, t.Text); c != nil && c.Summary != "" {
				p.Summary += ": " + c.Summary
			}
		}
	}
	*positional++
	ex.Parts = append(ex.Parts, p)
}

// flag explains the flag at toks[i] (and its value when that is the
// next token); it returns how many tokens it took.
func (ex *Explanation) flag(toks []Token, i int, chain []*Command, atRoot bool) int {
	t := toks[i]
	s := t.Text
	name, value, hasValue := s, "", false
	if strings.HasPrefix(s, "--") || !strings.HasPrefix(s, "--") && strings.IndexByte(s, '=') > 0 {
		if eq := strings.IndexByte(s, '='); eq > 0 {
			name, value, hasValue = s[:eq], s[eq+1:], true
		}
	}
	f := findFlag(chain, atRoot, name)
	if f == nil && hasValue {
		// "-n=x" isn't a spelling every tool takes; try it whole.
		name, value, hasValue = s, "", false
		f = findFlag(chain, atRoot, s)
	}
	if f != nil {
		ex.Parts = append(ex.Parts, flagPart(t, f, value, hasValue))
		if f.Value != "" && !hasValue && !f.OptionalValue && i+1 < len(toks) && toks[i+1].Kind == Word {
			v := toks[i+1]
			ex.Parts = append(ex.Parts, valuePart(v, f))
			return 2
		}
		return 1
	}
	// Short flags written together: -rf, -xzvf, -nshop (a value after the
	// last one that takes one).
	if len(s) > 2 && s[0] == '-' && s[1] != '-' {
		var flags []*Flag
		rest := ""
		for k := 1; k < len(s); k++ {
			g := findFlag(chain, atRoot, "-"+s[k:k+1])
			if g == nil {
				flags = nil
				break
			}
			flags = append(flags, g)
			if g.Value != "" && !g.OptionalValue {
				rest = s[k+1:]
				break
			}
		}
		if flags != nil {
			p := Part{Start: t.Start, End: t.End, Kind: KindFlag, Text: s, Ref: flags[0].Names[0]}
			var parts []string
			for _, g := range flags {
				parts = append(parts, short(g)+": "+g.Summary)
				if g.Danger != nil && (p.Danger == nil || g.Danger.Level == "danger") {
					p.Danger = g.Danger // the gravest of them
				}
			}
			p.Summary = strings.Join(parts, "; ")
			last := flags[len(flags)-1]
			if rest != "" {
				p.Summary += " (" + last.Value + " " + rest + ")"
			}
			ex.Parts = append(ex.Parts, p)
			if last.Value != "" && !last.OptionalValue && rest == "" && i+1 < len(toks) && toks[i+1].Kind == Word {
				ex.Parts = append(ex.Parts, valuePart(toks[i+1], last))
				return 2
			}
			return 1
		}
	}
	ex.Parts = append(ex.Parts, Part{Start: t.Start, End: t.End, Kind: KindUnknown, Text: s, Summary: "codec's reference doesn't have this flag"})
	return 1
}

func flagPart(t Token, f *Flag, value string, hasValue bool) Part {
	p := Part{Start: t.Start, End: t.End, Kind: KindFlag, Text: t.Text, Summary: f.Summary, Danger: f.Danger, Ref: f.Names[0]}
	if hasValue {
		if c := choice(f.Choices, value); c != nil && c.Summary != "" {
			p.Summary += " (" + value + ": " + c.Summary + ")"
		}
	}
	return p
}

func valuePart(t Token, f *Flag) Part {
	p := Part{Start: t.Start, End: t.End, Kind: KindValue, Text: t.Text, Summary: "the " + f.Value + " for " + short(f), Ref: f.Names[0]}
	if c := choice(f.Choices, t.Text); c != nil && c.Summary != "" {
		p.Summary += ": " + c.Summary
	}
	return p
}

// short is a flag's shortest name, for "the namespace for -n".
func short(f *Flag) string {
	best := f.Names[0]
	for _, n := range f.Names {
		if len(n) < len(best) {
			best = n
		}
	}
	return best
}

// findFlag looks a flag up in the command, then in the persistent flags
// of the commands above it; a root flag that isn't persistent only
// counts before the first subcommand.
func findFlag(chain []*Command, atRoot bool, name string) *Flag {
	numeric := len(name) > 1 && name[0] == '-' && allDigits(name[1:])
	for k := len(chain) - 1; k >= 0; k-- {
		c := chain[k]
		for j := range c.Flags {
			f := &c.Flags[j]
			if k < len(chain)-1 && !f.Persistent && !(k == 0 && atRoot) {
				continue
			}
			for _, n := range f.Names {
				if n == name || numeric && n == "-<n>" {
					return f
				}
			}
		}
	}
	return nil
}

func choice(cs []Choice, v string) *Choice {
	for i := range cs {
		if cs[i].Value == v {
			return &cs[i]
		}
	}
	return nil
}

func redirectSummary(op string) string {
	k := 0
	for k < len(op) && op[k] >= '0' && op[k] <= '9' {
		k++
	}
	n, rest := op[:k], op[k:]
	what := "the output"
	switch n {
	case "2":
		what = "errors"
	case "0":
		what = "input"
	}
	switch {
	case rest == ">&1" || rest == ">&2" || strings.HasPrefix(rest, ">&") && len(rest) == 3:
		to := "the output"
		if rest == ">&2" {
			to = "errors"
		}
		if n == "" {
			n = "1"
		}
		if n == "1" {
			what = "the output"
		}
		return "sends " + what + " to where " + to + " goes"
	case rest == ">":
		return "writes " + what + " to the file, replacing what was in it"
	case rest == ">|":
		return "writes " + what + " to the file, replacing it even if the shell is set not to"
	case rest == ">>":
		return "adds " + what + " to the end of the file"
	case rest == "&>":
		return "writes the output and errors to the file, replacing it"
	case rest == "&>>":
		return "adds the output and errors to the end of the file"
	case rest == "<":
		return "reads the input from the file"
	case rest == "<<" || rest == "<<-":
		return "reads the input from the lines that follow, up to the marker word"
	case rest == "<<<":
		return "passes the word as the input"
	case rest == "<>":
		return "opens the file for reading and writing"
	}
	return ""
}

func isName(s string) bool {
	for i, c := range s {
		if !(c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return s != ""
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}
