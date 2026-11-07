// Package quote writes a value so a target reads it back exactly: a
// shell, a data format, a language's string, a regular expression
// (decision 104). It also reads a quoted value back, and quotes through
// layers (a command for sh -c, inside kubectl exec, inside ssh). Pure:
// no files, no network.
package quote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"github.com/mahasenabheetha/codec/v2/internal/commands"
)

// Target is somewhere a value goes.
type Target struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Group string `json:"group"` // shells, data, build, code
}

// Targets lists them, in the order a menu shows them.
var Targets = []Target{
	{"bash", "bash, zsh", "shells"},
	{"sh", "sh (POSIX)", "shells"},
	{"powershell", "PowerShell", "shells"},
	{"cmd", "cmd.exe", "shells"},
	{"json", "JSON", "data"},
	{"yaml", "YAML", "data"},
	{"env", ".env file", "data"},
	{"helm-set", "Helm --set", "data"},
	{"makefile", "Makefile", "build"},
	{"dockerfile", "Dockerfile", "build"},
	{"sql", "SQL", "code"},
	{"regex", "Regular expression", "code"},
	{"go", "Go", "code"},
	{"xml", "XML", "code"},
	{"html", "HTML", "code"},
}

// Form is one way to write the value for a target.
type Form struct {
	Label string `json:"label"` // "single quotes", "block scalar"
	Value string `json:"value"`
	Why   string `json:"why"` // what this way does with the value
}

// Result is the value written for a target: the forms, the best first,
// and notes on how the target would otherwise misread it.
type Result struct {
	Forms []Form   `json:"forms"`
	Notes []string `json:"notes"`
}

// Quote writes value for the target.
func Quote(target, value string) (Result, error) {
	f, ok := quoters[target]
	if !ok {
		return Result{}, fmt.Errorf("no target %q", target)
	}
	r := f(value)
	if r.Notes == nil {
		r.Notes = []string{}
	}
	return r, nil
}

// Unquote reads a value written for the target back to what it stands
// for: the other way round.
func Unquote(target, text string) (string, error) {
	f, ok := unquoters[target]
	if !ok {
		return "", fmt.Errorf("no target %q", target)
	}
	return f(text)
}

// Layer is one wrapping: the value so far is quoted for Target and
// written after Before, such as "kubectl exec -it web -- sh -c ".
type Layer struct {
	Target string `json:"target"`
	Before string `json:"before"`
}

// Step is the value after one layer.
type Step struct {
	Target string `json:"target"`
	Value  string `json:"value"`
}

// Layers quotes value through each layer in turn, innermost first: each
// step's whole line is the next one's value.
func Layers(value string, layers []Layer) ([]Step, error) {
	out := []Step{}
	for _, l := range layers {
		r, err := Quote(l.Target, value)
		if err != nil {
			return nil, err
		}
		value = l.Before + r.Forms[0].Value
		out = append(out, Step{l.Target, value})
	}
	return out, nil
}

var quoters = map[string]func(string) Result{
	"bash":       func(v string) Result { return shell(v, true) },
	"sh":         func(v string) Result { return shell(v, false) },
	"powershell": powershell,
	"cmd":        cmd,
	"json":       jsonString,
	"yaml":       yamlValue,
	"env":        env,
	"helm-set":   helmSet,
	"makefile":   makefile,
	"dockerfile": dockerfile,
	"sql":        sql,
	"regex":      regexValue,
	"go":         goString,
	"xml":        xmlValue,
	"html":       htmlValue,
}

// Shells

// shellPlain is a word no POSIX shell changes.
var shellPlain = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

func shell(v string, bash bool) Result {
	var r Result
	single := "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
	singleWhy := "single quotes keep everything as it is, $ and \\ included"
	if strings.Contains(v, "'") {
		singleWhy += `; a ' can't be inside them, so each one is written '\'' (end the quotes, an escaped ', start them again)`
	}
	double := `"` + regexp.MustCompile("([\"$`\\\\])").ReplaceAllString(v, `\$1`) + `"`
	doubleWhy := "double quotes with \\ before \" $ ` and \\; use them to mix in $VARIABLES of your own"
	if strings.Contains(v, "!") && bash {
		doubleWhy += "; ! still runs history expansion in an interactive bash, so prefer single quotes"
	}
	switch {
	case v == "":
		r.Forms = append(r.Forms, Form{"empty", "''", "an empty argument must be quoted, or it disappears"})
	case shellPlain.MatchString(v):
		r.Forms = append(r.Forms, Form{"as it is", v, "nothing in it is special to the shell"})
		r.Forms = append(r.Forms, Form{"single quotes", single, singleWhy})
	case bash && hasControl(v):
		r.Forms = append(r.Forms, Form{"ANSI-C quotes", ansiC(v), "$'…' writes newlines, tabs and other control characters as \\n, \\t, \\xHH, on one line"})
		r.Forms = append(r.Forms, Form{"single quotes", single, singleWhy + "; control characters stay in it as they are"})
	default:
		r.Forms = append(r.Forms, Form{"single quotes", single, singleWhy})
		r.Forms = append(r.Forms, Form{"double quotes", double, doubleWhy})
	}
	if !bash && hasControl(v) {
		r.Notes = append(r.Notes, "sh has no $'…' everywhere (dash only from 0.5.12): a newline stays a real newline inside the single quotes")
	}
	if strings.HasPrefix(v, "-") {
		r.Notes = append(r.Notes, "it starts with -: quoting doesn't stop a command reading it as an option; put -- before it")
	}
	if strings.ContainsAny(v, "*?[") && !shellPlain.MatchString(v) {
		r.Notes = append(r.Notes, "unquoted, * ? and [ would match file names")
	}
	return r
}

func hasControl(v string) bool {
	for _, c := range v {
		if c < 0x20 || c == 0x7f {
			return true
		}
	}
	return false
}

func ansiC(v string) string {
	var b strings.Builder
	b.WriteString("$'")
	for _, c := range v {
		switch c {
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		default:
			if c < 0x20 || c == 0x7f {
				fmt.Fprintf(&b, `\x%02x`, c)
			} else {
				b.WriteRune(c)
			}
		}
	}
	b.WriteString("'")
	return b.String()
}

func powershell(v string) Result {
	var r Result
	single := "'" + strings.ReplaceAll(v, "'", "''") + "'"
	if hasControl(v) {
		var b strings.Builder
		b.WriteString(`"`)
		for _, c := range v {
			switch c {
			case '\n':
				b.WriteString("`n")
			case '\t':
				b.WriteString("`t")
			case '\r':
				b.WriteString("`r")
			case '`', '$', '"':
				b.WriteString("`" + string(c))
			default:
				if c < 0x20 {
					fmt.Fprintf(&b, "`u{%x}", c) // PowerShell 6 and later
				} else {
					b.WriteRune(c)
				}
			}
		}
		b.WriteString(`"`)
		r.Forms = append(r.Forms, Form{"double quotes", b.String(), "double quotes, with `n, `t, `r and `u{…} (PowerShell 6 and later) for control characters, and ` before $ ` and \""})
		r.Forms = append(r.Forms, Form{"single quotes", single, "single quotes keep everything as it is, the control characters too; a ' inside is written ''"})
		return r
	}
	why := "single quotes keep everything as it is, $ and ` included"
	if strings.Contains(v, "'") {
		why += "; a ' inside is written ''"
	}
	r.Forms = append(r.Forms, Form{"single quotes", single, why})
	double := `"` + regexp.MustCompile("([`$\"])").ReplaceAllString(v, "`$1") + `"`
	r.Forms = append(r.Forms, Form{"double quotes", double, "double quotes with ` before $ ` and \"; use them to mix in $variables of your own"})
	if strings.ContainsAny(v, "‘’“”") {
		r.Notes = append(r.Notes, "PowerShell reads curly quotes (‘ ’ “ ”) as quotes too")
	}
	r.Notes = append(r.Notes, "a native program (git, kubectl) gets the value from PowerShell 7.3 and later as it is; Windows PowerShell 5.1 drops \" inside arguments to native programs")
	return r
}

func cmd(v string) Result {
	var r Result
	// MSVC's argument rules, which most programs use: a " inside is \",
	// and backslashes before a " are doubled.
	var b strings.Builder
	b.WriteString(`"`)
	slashes := 0
	for _, c := range v {
		switch c {
		case '\\':
			slashes++
			b.WriteRune(c)
			continue
		case '"':
			b.WriteString(strings.Repeat(`\`, slashes) + `\"`)
		default:
			b.WriteRune(c)
		}
		slashes = 0
	}
	b.WriteString(strings.Repeat(`\`, slashes) + `"`)
	r.Forms = append(r.Forms, Form{"double quotes", b.String(), "double quotes keep spaces and & | < > ^ together; a \" inside is \\\" for programs that read their arguments the usual (MSVC) way"})
	if strings.Contains(v, "%") {
		r.Forms = append(r.Forms, Form{"in a .bat file", strings.ReplaceAll(b.String(), "%", "%%"), "in a batch file % is written %%"})
		r.Notes = append(r.Notes, "%NAME% is expanded even inside double quotes; at the prompt there is no sure way to stop it")
	}
	if strings.Contains(v, "!") {
		r.Notes = append(r.Notes, "with delayed expansion on, ! is special too")
	}
	if hasControl(v) {
		r.Notes = append(r.Notes, "cmd can't put a newline in an argument")
	}
	return r
}

// Data

func jsonString(v string) Result {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return Result{Forms: []Form{{"string", strings.TrimRight(buf.String(), "\n"), "a JSON string: \\ before \" and \\, control characters as \\n, \\t or \\u00XX"}}}
}

// yamlSpecial are plain scalars that some YAML reader takes for
// something other than a string.
var (
	yamlBool    = regexp.MustCompile(`^(?i:y|yes|n|no|true|false|on|off)$`)
	yamlNull    = regexp.MustCompile(`^(?:~|null|Null|NULL)$`)
	yamlNumber  = regexp.MustCompile(`^[-+]?(?:\d[\d_]*(?:\.\d*)?(?:[eE][-+]?\d+)?|\.\d+|0x[0-9a-fA-F]+|0o?[0-7]+|\.inf|\.nan|\d+(?::[0-5]?\d)+)$`)
	yamlPlainOK = regexp.MustCompile(`^[^-?:,\[\]{}#&*!|>'"%@` + "`" + `\s][^\n]*$`)
)

func yamlValue(v string) Result {
	var r Result
	single := "'" + strings.ReplaceAll(v, "'", "''") + "'"
	dq, _ := json.Marshal(v) // a JSON string is a YAML double-quoted scalar
	double := string(dq)
	plainOK := v != "" && yamlPlainOK.MatchString(v) && !strings.Contains(v, ": ") && !strings.Contains(v, " #") &&
		!strings.HasSuffix(v, " ") && !strings.HasSuffix(v, ":")
	switch {
	case yamlBool.MatchString(v):
		r.Notes = append(r.Notes, fmt.Sprintf("plain, %s is read as a boolean: YAML 1.1 readers (PyYAML, Ansible, older Kubernetes tools) take yes/no/on/off/y/n too", v))
		plainOK = false
	case yamlNull.MatchString(v):
		r.Notes = append(r.Notes, "plain, it is read as null")
		plainOK = false
	case yamlNumber.MatchString(v):
		r.Notes = append(r.Notes, "plain, it is read as a number ("+yamlNumberWhy(v)+")")
		plainOK = false
	}
	if strings.Contains(v, "\n") {
		chomp := "|-"
		why := "a literal block keeps the lines as they are; |- leaves out the last newline"
		if strings.HasSuffix(v, "\n") {
			chomp, why = "|", "a literal block keeps the lines as they are, with the last newline"
		}
		var b strings.Builder
		b.WriteString(chomp + "\n")
		for _, line := range strings.Split(strings.TrimSuffix(v, "\n"), "\n") {
			b.WriteString("  " + line + "\n")
		}
		block := b.String() // | keeps the last newline only if the block ends with one
		if chomp == "|-" {
			block = strings.TrimRight(block, "\n")
		}
		if strings.HasPrefix(v, " ") || strings.HasPrefix(v, "\t") {
			block = strings.Replace(block, chomp, chomp+"2", 1)
			why += "; the 2 says how far it is indented, since the first line starts with a space"
		}
		r.Forms = append(r.Forms, Form{"literal block", block, why})
		r.Forms = append(r.Forms, Form{"double quotes", double, "double quotes with \\n for the line breaks, on one line"})
		return r
	}
	if plainOK {
		r.Forms = append(r.Forms, Form{"plain", v, "nothing in it makes YAML read it as anything but this string"})
	}
	if strings.ContainsAny(v, "\\") || !strings.ContainsAny(v, "'") || hasControl(v) {
		r.Forms = append(r.Forms, Form{"single quotes", single, "single quotes keep \\ and everything else as it is; a ' inside is written ''"})
		r.Forms = append(r.Forms, Form{"double quotes", double, "double quotes with \\ escapes, as JSON writes them"})
	} else {
		r.Forms = append(r.Forms, Form{"double quotes", double, "double quotes with \\ escapes, as JSON writes them"})
		r.Forms = append(r.Forms, Form{"single quotes", single, "single quotes keep everything as it is; a ' inside is written ''"})
	}
	if hasControl(v) {
		r.Forms[0], r.Forms[1] = r.Forms[1], r.Forms[0] // only double quotes can write \t
	}
	if !plainOK && v != "" && len(r.Notes) == 0 {
		r.Notes = append(r.Notes, "plain, it would not be read as this string: it starts with or has a character YAML gives a meaning (: - # & * ! | > ' \" % @ ` [ ] { } ,)")
	}
	return r
}

func yamlNumberWhy(v string) string {
	switch {
	case strings.Contains(v, ":"):
		return "YAML 1.1 reads 1:30 as 90, base 60"
	case strings.HasPrefix(v, "0") && len(v) > 1 && !strings.ContainsAny(v, ".eE"):
		return "a leading 0 makes it octal in YAML 1.1"
	case strings.Contains(v, ".") && strings.HasSuffix(v, "0"):
		return "1.10 becomes 1.1, a version number loses its zero"
	}
	return "it loses being text: a value that must stay a string needs quotes"
}

func env(v string) Result {
	var r Result
	plain := regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]*$`).MatchString(v)
	dq := `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "$", `\$`).Replace(v) + `"`
	if plain {
		r.Forms = append(r.Forms, Form{"as it is", v, "nothing in it needs quotes"})
	}
	r.Forms = append(r.Forms, Form{"double quotes", dq, "double quotes with \\ before \" \\ and $, and \\n for line breaks, as Compose and the dotenv libraries read them"})
	if !strings.Contains(v, "'") && !strings.Contains(v, "\n") {
		r.Forms = append(r.Forms, Form{"single quotes", "'" + v + "'", "single quotes: no escapes and no ${…} expansion in Compose"})
	}
	if !plain {
		r.Notes = append(r.Notes, "docker run --env-file takes everything after = as it is, quotes included: there, write the value bare")
	}
	if strings.Contains(v, "#") {
		r.Notes = append(r.Notes, "unquoted, # after a space starts a comment in most readers")
	}
	return r
}

func helmSet(v string) Result {
	var r Result
	esc := strings.NewReplacer(`\`, `\\`, `,`, `\,`).Replace(v)
	typed := yamlBool.MatchString(v) || yamlNull.MatchString(v) || yamlNumber.MatchString(v)
	flag := "--set"
	if typed {
		flag = "--set-string"
		r.Notes = append(r.Notes, "--set would make "+v+" a number, a boolean or null; --set-string keeps it text")
	}
	if strings.HasPrefix(v, "{") && strings.HasSuffix(v, "}") {
		r.Notes = append(r.Notes, "{…} is read as a list; --set-string doesn't help, use --set-json or a values file")
	}
	arg := shell(flag+" key="+esc, true).Forms[0].Value
	r.Forms = append(r.Forms, Form{"value", esc, "a , ends a value in --set, so it is written \\,, and \\ as \\\\"})
	r.Forms = append(r.Forms, Form{"whole argument, for bash", arg, "the flag with key= (change key to yours), quoted for the shell"})
	if strings.Contains(v, "\n") || len(v) > 200 {
		r.Notes = append(r.Notes, "--set-file key=path reads a long or multi-line value from a file")
	}
	return r
}

// Build files

func makefile(v string) Result {
	var r Result
	dollar := strings.ReplaceAll(v, "$", "$$")
	sh := shell(v, false).Forms[0].Value
	r.Forms = append(r.Forms, Form{"in a recipe", strings.ReplaceAll(sh, "$", "$$"), "quoted for the shell the recipe runs in, then each $ written $$ so make passes it on"})
	r.Forms = append(r.Forms, Form{"in a variable", strings.ReplaceAll(dollar, "#", `\#`), "make expands $ and starts a comment at #: written $$ and \\#"})
	if strings.Contains(v, "\n") {
		r.Notes = append(r.Notes, "each line of a recipe runs in its own shell; a value with line breaks needs define … endef")
	}
	return r
}

func dockerfile(v string) Result {
	var r Result
	js := jsonString(v).Forms[0].Value
	sh := shell(v, false).Forms[0].Value
	r.Forms = append(r.Forms, Form{"exec form", "[" + js + "]", "a JSON array, one string per argument: no shell runs, so $VAR is not expanded and nothing else is special"})
	r.Forms = append(r.Forms, Form{"shell form", sh, "RUN and CMD without brackets run in /bin/sh -c, so the value is quoted for sh"})
	r.Forms = append(r.Forms, Form{"ENV or LABEL value", `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`).Replace(v) + `"`, "double quotes with \\ before \" \\ and $ (the Dockerfile expands $VAR itself)"})
	if strings.Contains(v, "\n") {
		r.Notes = append(r.Notes, "a line break in an instruction needs \\ at the end of the line, or a heredoc (RUN <<EOF)")
	}
	return r
}

// Code

func sql(v string) Result {
	r := Result{Forms: []Form{
		{"string", "'" + strings.ReplaceAll(v, "'", "''") + "'", "a SQL string: a ' inside is written ''"},
		{"identifier (standard, PostgreSQL)", `"` + strings.ReplaceAll(v, `"`, `""`) + `"`, "a name of a table or column: \" inside is written \"\""},
		{"identifier (MySQL)", "`" + strings.ReplaceAll(v, "`", "``") + "`", "a name in MySQL and MariaDB"},
		{"identifier (SQL Server)", "[" + strings.ReplaceAll(v, "]", "]]") + "]", "a name in SQL Server"},
	}}
	r.Notes = append(r.Notes, "in code, pass values as parameters (? or $1), never pasted into the SQL")
	if strings.Contains(v, `\`) {
		r.Notes = append(r.Notes, "MySQL without NO_BACKSLASH_ESCAPES reads \\ as an escape in strings: write it \\\\ there")
	}
	return r
}

func regexValue(v string) Result {
	ere := regexp.MustCompile(`([.\[\](){}*+?|^$\\])`).ReplaceAllString(v, `\$1`)
	bre := regexp.MustCompile(`([.\[\]*^$\\])`).ReplaceAllString(v, `\$1`)
	repl := regexp.MustCompile(`([&\\/])`).ReplaceAllString(v, `\$1`)
	return Result{Forms: []Form{
		{"RE2, PCRE, JavaScript, Python", regexp.QuoteMeta(v), "matches the text as it is: \\ before every character a regular expression gives a meaning"},
		{"POSIX extended (grep -E, awk)", ere, "\\ before . [ ] ( ) { } * + ? | ^ $ and \\"},
		{"POSIX basic (grep, sed)", bre, "\\ before . [ ] * ^ $ and \\; ( ) { } + ? are already plain there"},
		{"sed replacement", repl, "the s/…/HERE/ side: \\ before & \\ and /"},
	}, Notes: []string{"to match the whole text only, put ^ before and $ after"}}
}

func goString(v string) Result {
	var r Result
	if utf8.ValidString(v) && !strings.Contains(v, "`") && !strings.Contains(v, "\r") && printable(v) {
		r.Forms = append(r.Forms, Form{"raw string", "`" + v + "`", "a raw string keeps everything as it is, line breaks too; it can't hold a `"})
	}
	r.Forms = append(r.Forms, Form{"interpreted string", strconv.Quote(v), "\\ before \" and \\, \\n and \\t for control characters"})
	return r
}

func printable(v string) bool {
	for _, c := range v {
		if c != '\n' && c != '\t' && !unicode.IsPrint(c) {
			return false
		}
	}
	return true
}

func xmlValue(v string) Result {
	text := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(v)
	attr := `"` + strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "\n", "&#10;", "\t", "&#9;").Replace(v) + `"`
	cdata := "<![CDATA[" + strings.ReplaceAll(v, "]]>", "]]]]><![CDATA[>") + "]]>"
	return Result{Forms: []Form{
		{"text", text, "between tags: & < and > as &amp; &lt; &gt;"},
		{"attribute", attr, "an attribute in double quotes: \" as &quot;, line breaks as &#10; (else they become spaces)"},
		{"CDATA", cdata, "a CDATA section keeps everything as it is; ]]> inside is split across two"},
	}}
}

func htmlValue(v string) Result {
	return Result{Forms: []Form{
		{"text or attribute", html.EscapeString(v), "& < > \" and ' as entities, safe between tags and in quoted attributes"},
	}, Notes: []string{"inside <script>, a URL or CSS, entities aren't enough: each needs its own escaping"}}
}

// Reading back

var unquoters = map[string]func(string) (string, error){
	"bash":       shellUnquote,
	"sh":         shellUnquote,
	"powershell": powershellUnquote,
	"cmd":        cmdUnquote,
	"json": func(t string) (string, error) {
		var s string
		err := json.Unmarshal([]byte(strings.TrimSpace(t)), &s)
		return s, err
	},
	"yaml": func(t string) (string, error) {
		var n yaml.Node
		if err := yaml.Unmarshal([]byte(t), &n); err != nil {
			return "", err
		}
		if len(n.Content) == 0 || n.Content[0].Kind != yaml.ScalarNode {
			return "", fmt.Errorf("that is not one YAML value")
		}
		return n.Content[0].Value, nil
	},
	"env": func(t string) (string, error) {
		t = strings.TrimSpace(t)
		if len(t) >= 2 && t[0] == '\'' && t[len(t)-1] == '\'' {
			return t[1 : len(t)-1], nil
		}
		if len(t) >= 2 && t[0] == '"' && t[len(t)-1] == '"' {
			return strings.NewReplacer(`\\`, `\`, `\"`, `"`, `\n`, "\n", `\$`, "$").Replace(t[1 : len(t)-1]), nil
		}
		return t, nil
	},
	"helm-set": func(t string) (string, error) {
		return strings.NewReplacer(`\\`, `\`, `\,`, `,`).Replace(t), nil
	},
	"makefile": func(t string) (string, error) {
		return strings.NewReplacer("$$", "$", `\#`, "#").Replace(t), nil
	},
	"dockerfile": func(t string) (string, error) {
		t = strings.TrimSpace(t)
		if strings.HasPrefix(t, "[") {
			var a []string
			if err := json.Unmarshal([]byte(t), &a); err != nil {
				return "", err
			}
			return strings.Join(a, " "), nil
		}
		return shellUnquote(t)
	},
	"sql": func(t string) (string, error) {
		t = strings.TrimSpace(t)
		for _, q := range []struct{ open, close, twice string }{{"'", "'", "''"}, {`"`, `"`, `""`}, {"`", "`", "``"}, {"[", "]", "]]"}} {
			if len(t) >= 2 && strings.HasPrefix(t, q.open) && strings.HasSuffix(t, q.close) {
				return strings.ReplaceAll(t[1:len(t)-1], q.twice, q.close), nil
			}
		}
		return "", fmt.Errorf("a SQL string starts and ends with '")
	},
	"regex": func(t string) (string, error) {
		return regexp.MustCompile(`\\(.)`).ReplaceAllString(t, "$1"), nil
	},
	"go": func(t string) (string, error) { return strconv.Unquote(strings.TrimSpace(t)) },
	"xml": func(t string) (string, error) {
		if strings.HasPrefix(t, "<![CDATA[") && strings.HasSuffix(t, "]]>") {
			return strings.ReplaceAll(t[9:len(t)-3], "]]]]><![CDATA[>", "]]>"), nil
		}
		// The attribute form: quoted, with no " inside but as &quot;.
		if len(t) >= 2 && t[0] == '"' && t[len(t)-1] == '"' && !strings.Contains(t[1:len(t)-1], `"`) {
			t = t[1 : len(t)-1]
		}
		return html.UnescapeString(t), nil
	},
	"html": func(t string) (string, error) { return html.UnescapeString(t), nil },
}

// shellUnquote reads one shell word, with the command reader's own
// rules for '…', "…", $'…' and \.
func shellUnquote(t string) (string, error) {
	toks := commands.Read(strings.TrimSpace(t))
	if len(toks) != 1 || toks[0].Kind != commands.Word {
		return "", fmt.Errorf("that is %d words; quote it as one", len(toks))
	}
	if toks[0].Unclosed {
		return "", fmt.Errorf("a quote isn't closed")
	}
	return toks[0].Text, nil
}

func powershellUnquote(t string) (string, error) {
	t = strings.TrimSpace(t)
	switch {
	case len(t) >= 2 && t[0] == '\'' && t[len(t)-1] == '\'':
		return strings.ReplaceAll(t[1:len(t)-1], "''", "'"), nil
	case len(t) >= 2 && t[0] == '"' && t[len(t)-1] == '"':
		var b strings.Builder
		in := t[1 : len(t)-1]
		for i := 0; i < len(in); i++ {
			if in[i] == '`' && i+1 < len(in) {
				i++
				switch in[i] {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				case 'r':
					b.WriteByte('\r')
				case '0':
					b.WriteByte(0)
				case 'u':
					// `u{HEX}, a character by its code
					if end := strings.IndexByte(in[i:], '}'); i+1 < len(in) && in[i+1] == '{' && end > 0 {
						if n, err := strconv.ParseUint(in[i+2:i+end], 16, 32); err == nil {
							b.WriteRune(rune(n))
							i += end
							continue
						}
					}
					b.WriteByte('u')
				default:
					b.WriteByte(in[i])
				}
				continue
			}
			b.WriteByte(in[i])
		}
		return b.String(), nil
	}
	return t, nil
}

func cmdUnquote(t string) (string, error) {
	t = strings.TrimSpace(t)
	// MSVC's rules: 2n backslashes before " are n and a quote that opens
	// or closes (dropped); 2n+1 are n and a literal ".
	var b strings.Builder
	for i := 0; i < len(t); i++ {
		if t[i] == '"' {
			continue // a quote that opens or closes
		}
		if t[i] != '\\' {
			b.WriteByte(t[i])
			continue
		}
		n := 0
		for i < len(t) && t[i] == '\\' {
			n++
			i++
		}
		if i < len(t) && t[i] == '"' {
			b.WriteString(strings.Repeat(`\`, n/2))
			if n%2 == 1 {
				b.WriteByte('"')
			}
			continue
		}
		b.WriteString(strings.Repeat(`\`, n))
		i--
	}
	return b.String(), nil
}
