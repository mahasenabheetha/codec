package helm

import (
	"regexp"
	"strconv"
	"strings"
)

// Helm reports template problems as text; these patterns pull out the
// file and position so the UI can jump there.
var (
	// execution error at (app/templates/x.yaml:3:4): image.tag is required
	reExecAt = regexp.MustCompile(`execution error at \(([^:()]+):(\d+):(\d+)\): (.*)`)
	// template: app/templates/x.yaml:12:5: executing "..." at <.Values.a>: nil pointer ...
	reTemplate = regexp.MustCompile(`template: ([^:\s]+):(\d+):(\d+): (.*)`)
	// parse error at (app/templates/x.yaml:7): unexpected "}" in operand
	reParse = regexp.MustCompile(`parse error at \(([^:()]+):(\d+)\): (.*)`)
	// YAML parse error on app/templates/x.yaml: error converting YAML to JSON: yaml: line 5: ...
	reYAML = regexp.MustCompile(`YAML parse error on ([^:\s]+): (.*)`)
)

// renderError turns a Helm render failure into a diagnostic on the
// chart file it names. Paths come as "<chart>/templates/…"; the chart
// name prefix is dropped to get the chart-relative path.
func renderError(err error) Diagnostic {
	msg := err.Error()
	d := Diagnostic{Severity: "error", Code: "render", Message: msg}
	rel := func(p string) string {
		if i := strings.IndexByte(p, '/'); i >= 0 {
			return p[i+1:]
		}
		return p
	}
	switch {
	case reExecAt.MatchString(msg):
		m := reExecAt.FindStringSubmatch(msg)
		d.File, d.Line, d.Col, d.Message = rel(m[1]), atoi(m[2]), atoi(m[3]), m[4]
		d.Code = "template-fail"
	case reTemplate.MatchString(msg):
		m := reTemplate.FindStringSubmatch(msg)
		d.File, d.Line, d.Col, d.Message = rel(m[1]), atoi(m[2]), atoi(m[3]), m[4]
		d.Code = "template-exec"
		if strings.Contains(m[4], "nil pointer evaluating") {
			d.Hint = "A value on this path is missing. Guard it with `if`/`with`, or set it in a values file."
		}
	case reParse.MatchString(msg):
		m := reParse.FindStringSubmatch(msg)
		d.File, d.Line, d.Message = rel(m[1]), atoi(m[2]), "Template syntax error: "+m[3]
		d.Code = "template-parse"
	case reYAML.MatchString(msg):
		m := reYAML.FindStringSubmatch(msg)
		d.File, d.Message = rel(m[1]), "The rendered output of this template is not valid YAML: "+m[2]
		d.Code = "rendered-yaml"
		d.Hint = "Line numbers refer to the rendered output. Check indentation around nindent/toYaml."
	}
	return d
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

var reLookup = regexp.MustCompile(`\blookup\s+"`)

// lookupNotes flags templates that call lookup: like `helm template`,
// codec has no cluster, so lookup returns an empty result.
func lookupNotes(files []File) []Diagnostic {
	var out []Diagnostic
	for _, f := range files {
		if !isTemplate(f.Name) {
			continue
		}
		for i, line := range strings.Split(string(f.Data), "\n") {
			if loc := reLookup.FindStringIndex(line); loc != nil {
				out = append(out, Diagnostic{
					Severity: "info", Code: "lookup", File: f.Name, Line: i + 1, Col: loc[0] + 1,
					Message: "lookup returns an empty result here: templates render without a cluster, as with helm template",
				})
				break
			}
		}
	}
	return out
}
