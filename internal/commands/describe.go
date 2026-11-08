package commands

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Describe writes an explanation as plain text, for the terminal (codec
// explain) and Smart paste: each command with its sentence and risk,
// its parts in columns, and the operators between commands.
//
//	git push --force origin main
//	Sends local commits to a remote repository.
//	! danger: commits on the remote that you don't have locally are lost
//	  safer: git push --force-with-lease
//
//	  git       command     Git, the version control system.
//	  push      subcommand  Sends local commits to a remote repository.
//	  --force   flag        overwrites the remote branch with the local one !
func Describe(ex Explanation) string {
	var b strings.Builder
	said := map[string]bool{} // risks already given, by their note
	for si, s := range ex.Segments {
		if si > 0 {
			b.WriteString("\n")
		}
		parts := ex.Parts[s.From:s.To]
		var words []string
		for _, p := range parts {
			if p.Kind != KindComment {
				words = append(words, ex.line[p.Start:p.End])
			}
		}
		b.WriteString(strings.Join(words, " ") + "\n")
		switch {
		case s.Summary != "":
			b.WriteString(s.Summary + "\n")
		case s.Tool == "" && len(parts) > 0:
			b.WriteString("Not in codec's reference: its parts are shown, without explanations.\n")
		}
		if d := s.Danger; d != nil {
			writeDanger(&b, d)
			said[d.Note] = true
		}
		width := 0
		for _, p := range parts {
			width = max(width, utf8.RuneCountInString(p.Text))
		}
		width = min(width, 24) // a long value doesn't push every line over
		b.WriteString("\n")
		for _, p := range parts {
			text := p.Text
			if text == "" {
				text = "(empty)"
			}
			line := fmt.Sprintf("  %-*s  %-11s %s", width, text, kindLabel(p.Kind), p.Summary)
			if p.Danger != nil {
				line += " !"
			}
			b.WriteString(strings.TrimRight(line, " ") + "\n")
		}
		if s.To < len(ex.Parts) && ex.Parts[s.To].Kind == KindOp {
			op := ex.Parts[s.To]
			text := op.Text
			if text == "\n" {
				text = "↵"
			}
			fmt.Fprintf(&b, "\n%s  %s\n", text, op.Summary)
		}
	}
	// The flags' risks, after the parts they belong to.
	for _, p := range ex.Parts {
		if p.Danger != nil && p.Kind == KindFlag && !said[p.Danger.Note] {
			said[p.Danger.Note] = true
			b.WriteString("\n" + p.Text + ":\n")
			writeDanger(&b, p.Danger)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func writeDanger(b *strings.Builder, d *Danger) {
	fmt.Fprintf(b, "! %s: %s\n", d.Level, d.Note)
	if d.Safer != "" {
		fmt.Fprintf(b, "  safer: %s\n", d.Safer)
	}
}

// kindLabel is a part's kind as people say it.
func kindLabel(kind string) string {
	switch kind {
	case KindTool:
		return "command"
	case KindCommand:
		return "subcommand"
	case KindArg:
		return "argument"
	case KindEnd:
		return "end flags"
	case KindEnv:
		return "variable"
	case KindWrapper:
		return "runs it"
	case KindTarget:
		return "file"
	}
	return kind
}

// LooksLikeCommand says whether text is a command line codec knows,
// for Smart paste: at most a few lines, the first command one in the
// reference, used with a subcommand, a flag or an operator (so "find
// the bug" stays prose), and few parts codec can't place.
func LooksLikeCommand(text string) bool {
	_, ok := Detect(text)
	return ok
}

// Detect is LooksLikeCommand with the explanation it made, so Smart
// paste doesn't explain the line twice. A line break is no sign of a
// command: two lines of prose that start with a tool's name ("top 10
// tips" then "for go") stay prose.
func Detect(text string) (Explanation, bool) {
	text = strings.TrimSpace(text)
	if text == "" || strings.Count(text, "\n") > 3 || len(text) > 2000 {
		return Explanation{}, false
	}
	ex := Explain(text)
	if len(ex.Segments) == 0 || ex.Segments[0].Tool == "" {
		return Explanation{}, false
	}
	used, unknown := false, 0
	for _, p := range ex.Parts {
		switch {
		case p.Kind == KindCommand, p.Kind == KindFlag, p.Kind == KindOp && p.Text != "\n":
			used = true
		case p.Kind == KindUnknown:
			unknown++
		}
	}
	return ex, used && unknown*4 <= len(ex.Parts)
}
