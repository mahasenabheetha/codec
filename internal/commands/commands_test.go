package commands

import (
	"fmt"
	"strings"
	"testing"
)

// words writes tokens compactly: words as they are, operators in <>.
func words(line string) string {
	var b []string
	for _, t := range Read(line) {
		switch t.Kind {
		case Op:
			b = append(b, "<"+strings.ReplaceAll(t.Text, "\n", `\n`)+">")
		case Redirect:
			b = append(b, "{"+t.Text+"}")
		case Comment:
			b = append(b, "#")
		default:
			s := t.Text
			if t.Unclosed {
				s += "…"
			}
			b = append(b, "["+s+"]")
		}
	}
	return strings.Join(b, " ")
}

func TestRead(t *testing.T) {
	tests := []struct{ name, line, want string }{
		{"plain", "git commit -m msg", "[git] [commit] [-m] [msg]"},
		{"quotes", `git commit -m "fix: it's done" -m 'two words'`, `[git] [commit] [-m] [fix: it's done] [-m] [two words]`},
		{"escapes", `echo a\ b "q\"x" \$HOME`, `[echo] [a b] [q"x] [$HOME]`},
		{"ansi-c", `printf $'a\tb\n'`, "[printf] [a\tb\n]"},
		{"joined quotes", `--set=a="b c"d`, `[--set=a=b cd]`},
		{"pipes and chains", "kubectl get po | grep web && echo ok || echo no; date &", "[kubectl] [get] [po] <|> [grep] [web] <&&> [echo] [ok] <||> [echo] [no] <;> [date] <&>"},
		{"no spaces", "a|b&&c", "[a] <|> [b] <&&> [c]"},
		{"redirects", "cmd > out.txt 2>&1 < in 2>>err &>all", "[cmd] {>} [out.txt] {2>&1} {<} [in] {2>>} [err] {&>} [all]"},
		{"number is a word", "head -n 20 file 2", "[head] [-n] [20] [file] [2]"},
		{"substitutions stay whole", `kubectl logs $(kubectl get po -o name | head -1) "${POD:-web}"`, `[kubectl] [logs] [$(kubectl get po -o name | head -1)] [${POD:-web}]`},
		{"backticks", "echo `date +%s`", "[echo] [`date +%s`]"},
		{"comment", "ls -la # list", "[ls] [-la] #"},
		{"hash in a word", "echo a#b", "[echo] [a#b]"},
		{"line continued", "git commit \\\n  -m x", "[git] [commit] [-m] [x]"},
		{"two lines", "cd x\nls", "[cd] [x] <\\n> [ls]"},
		{"unclosed quote", `echo "never closed`, `[echo] [never closed…]`},
		{"unclosed substitution", "echo $(date", "[echo] [$(date…]"},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := words(tc.line); got != tc.want {
				t.Errorf("Read(%q)\n got %s\nwant %s", tc.line, got, tc.want)
			}
		})
	}
}

// show writes an explanation as kind:text=summary parts, one segment a
// line.
func show(line string) string {
	ex := Explain(line)
	var lines []string
	for _, s := range ex.Segments {
		var b []string
		for _, p := range ex.Parts[s.From:s.To] {
			x := p.Kind + ":" + line[p.Start:p.End]
			if p.Danger != nil {
				x += "!" + p.Danger.Level
			}
			b = append(b, x)
		}
		lines = append(lines, strings.Join(s.Path, " ")+" | "+strings.Join(b, " "))
	}
	return strings.Join(lines, "\n")
}

func TestExplain(t *testing.T) {
	tests := []struct{ name, line, want string }{
		{"subcommand and flags", `git commit -am "fix" --amend`,
			"git commit | tool:git command:commit flag:-am value:\"fix\" flag:--amend!caution"},
		{"global flag before the subcommand", "git -C ../repo log --oneline -5",
			"git log | tool:git flag:-C value:../repo command:log flag:--oneline flag:-5"},
		{"a root flag after the subcommand is unknown", "git log -C x",
			"git log | tool:git command:log unknown:-C arg:x"},
		{"persistent flag anywhere", "kubectl get pods -n shop -o yaml",
			"kubectl get | tool:kubectl command:get arg:pods flag:-n value:shop flag:-o value:yaml"},
		{"attached values", "kubectl -nshop logs --tail=50 web",
			"kubectl logs | tool:kubectl flag:-nshop command:logs flag:--tail=50 arg:web"},
		{"nested subcommand", "kubectl rollout restart deployment/web",
			"kubectl rollout restart | tool:kubectl command:rollout command:restart arg:deployment/web"},
		{"alias", "k get po", "kubectl get | tool:k command:get arg:po"},
		{"danger on the command", "kubectl delete pod web",
			"kubectl delete | tool:kubectl command:delete!danger arg:pod arg:web"},
		{"optional value only after =", "kubectl delete pod web --dry-run=client",
			"kubectl delete | tool:kubectl command:delete!danger arg:pod arg:web flag:--dry-run=client"},
		{"force push", "git push -f origin main",
			"git push | tool:git command:push flag:-f!danger arg:origin arg:main"},
		{"after -- everything is an argument", "git commit -- -file",
			"git commit | tool:git command:commit end:-- arg:-file"},
		{"pipeline", "kubectl get po -A | grep Crash",
			"kubectl get | tool:kubectl command:get arg:po flag:-A\n | unknown:grep arg:Crash"},
		{"env, sudo and a redirect", "LANG=C sudo -u bob git log > out.txt 2>&1",
			"git log | env:LANG=C wrapper:sudo wrapper:-u wrapper:bob tool:git command:log redirect:> target:out.txt redirect:2>&1"},
		{"timeout's duration", "timeout 5s kubectl logs -f web",
			"kubectl logs | wrapper:timeout wrapper:5s tool:kubectl command:logs flag:-f arg:web"},
		{"tool by path", "/usr/bin/git log", "git log | tool:/usr/bin/git command:log"},
		{"unknown flag", "git commit --frobnicate",
			"git commit | tool:git command:commit unknown:--frobnicate"},
		{"a subcommand name later is an argument", "git log commit",
			"git log | tool:git command:log arg:commit"},
		{"only a wrapper", "sudo -i", " | wrapper:sudo wrapper:-i"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := show(tc.line); got != tc.want {
				t.Errorf("Explain(%q)\n got %s\nwant %s", tc.line, got, tc.want)
			}
		})
	}
}

func TestExplainSays(t *testing.T) {
	ex := Explain("kubectl get pods -o yaml -n shop")
	var says []string
	for _, p := range ex.Parts {
		says = append(says, p.Summary)
	}
	got := strings.Join(says, " | ")
	for _, want := range []string{
		"the kind of resource, such as pods or deploy, or kind/name: the pods",
		"prints in this format",
		"the format for -o: the full resource as YAML",
		"the namespace for -n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if s := ex.Segments[0]; s.Summary != "Lists resources, or shows one." || s.Danger != nil {
		t.Errorf("segment: %+v", s)
	}
	if d := Explain("git reset --hard HEAD~1").Segments[0].Danger; d == nil || d.Safer != "git stash" {
		t.Errorf("reset --hard danger: %+v", d)
	}
}

// What the builder reads: the flags with their values in every
// spelling, the arguments, and what it keeps as written.
func TestUse(t *testing.T) {
	line := `LANG=C git -C repo commit -am "fix it" --frobnicate -- a.go > out 2>&1`
	u := Explain(line).Segments[0].Use
	var flags []string
	for _, f := range u.Flags {
		flags = append(flags, fmt.Sprintf("%s=%q/%v", f.Ref, f.Value, f.Valued))
	}
	var keep []string
	for _, k := range u.Keep {
		keep = append(keep, line[k.Start:k.End])
	}
	if got, want := strings.Join(flags, " "), `-C="repo"/true -a=""/false -m="fix it"/true`; got != want {
		t.Errorf("flags %s, want %s", got, want)
	}
	if strings.Join(u.Args, ",") != "a.go" || strings.Join(keep, ",") != "--frobnicate,> out,2>&1" || line[:u.ToolStart] != "LANG=C " || u.End != len(line) {
		t.Errorf("args %q keep %q prefix %q end %d", u.Args, keep, line[:u.ToolStart], u.End)
	}
	if u := Explain("kubectl -nshop logs --tail=50 web").Segments[0].Use; fmt.Sprint(u.Flags) != "[{-n shop true 0} {--tail 50 true 1}]" {
		t.Errorf("attached values: %v", u.Flags)
	}
	if u := Explain("grep x").Segments[0].Use; u != nil {
		t.Errorf("unknown command has a use: %+v", u)
	}
}

// Every pack loads and passes the checks.
func TestPacks(t *testing.T) {
	tools, err := Tools()
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) < 2 {
		t.Fatalf("tools: %v", tools)
	}
	for _, ti := range tools {
		if tool, err := Lookup(ti.Name); err != nil || tool == nil || ti.Summary == "" || ti.Group == "" {
			t.Errorf("%s: %v %v", ti.Name, tool, err)
		}
	}
}

func TestCheck(t *testing.T) {
	for _, c := range []Command{
		{Name: "x"},
		{Name: "x", Summary: "s", Flags: []Flag{{Names: []string{"m"}, Summary: "s"}}},
		{Name: "x", Summary: "s", Flags: []Flag{{Names: []string{"-m"}, Summary: "s"}, {Names: []string{"-m"}, Summary: "t"}}},
		{Name: "x", Summary: "s", Flags: []Flag{{Names: []string{"-m"}, Summary: "s", OptionalValue: true}}},
		{Name: "x", Summary: "s", Args: []Arg{{Name: "a", Repeat: true}, {Name: "b"}}},
		{Name: "x", Summary: "s", Danger: &Danger{Level: "scary"}},
	} {
		if err := check(&c, "x"); err == nil {
			t.Errorf("check(%+v) passed", c)
		}
	}
}

func FuzzExplain(f *testing.F) {
	for _, s := range []string{
		`git commit -am "x" && git push -f`, "kubectl -nshop get po -o=yaml | grep x > f 2>&1",
		`echo $'a\'b' "$(x "y")" \`, "sudo -u a env A=1 timeout 1 k logs", "a<<<b>|c;;d|&e", `"unclosed`, "-", "--", "=", "#",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, line string) {
		last := 0
		for _, tok := range Read(line) {
			if tok.Start < last || tok.End <= tok.Start || tok.End > len(line) {
				t.Fatalf("bad token %+v in %q", tok, line)
			}
			last = tok.End
		}
		ex := Explain(line)
		for _, s := range ex.Segments {
			if s.From > s.To || s.To > len(ex.Parts) {
				t.Fatalf("bad segment %+v", s)
			}
		}
		for _, p := range ex.Parts {
			if p.Start < 0 || p.End > len(line) || p.Start >= p.End {
				t.Fatalf("bad part %s %d–%d in %q", p.Kind, p.Start, p.End, line)
			}
		}
		_ = fmt.Sprint(ex)
	})
}

// Regression: the builder wrote every flag after the last subcommand,
// and dropped a root flag (git -C repo log lost -C). A use says whose
// flag each is, so it can be written back there.
func TestUseLevels(t *testing.T) {
	u := Explain("git -C repo -p log -p --oneline").Segments[0].Use
	// -p before log is git's (a pager), after it log's (the patch).
	if fmt.Sprint(u.Flags) != "[{-C repo true 0} {-p  false 0} {-p  false 1} {--oneline  false 1}]" {
		t.Errorf("flags: %v", u.Flags)
	}
}

// A choice written as a prefix (jsonpath=) explains every value it
// starts.
func TestChoicePrefix(t *testing.T) {
	ex := Explain("kubectl get pods -o jsonpath={.items[*].metadata.name}")
	var got string
	for _, p := range ex.Parts {
		if p.Kind == "value" {
			got = p.Summary
		}
	}
	if !strings.Contains(got, "JSONPath") {
		t.Errorf("value: %q", got)
	}
}

// Every recipe explains fully: its tool is known, and no part of it is
// a word the reference doesn't have.
func TestRecipes(t *testing.T) {
	tools, err := Tools()
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, ti := range tools {
		tool, _ := Lookup(ti.Name)
		for _, r := range tool.Recipes {
			n++
			ex := Explain(r.Command)
			if len(ex.Segments) != 1 || ex.Segments[0].Tool != tool.Name {
				t.Errorf("%s: %q isn't one %s command", ti.Name, r.Command, tool.Name)
				continue
			}
			for _, p := range ex.Parts {
				if p.Kind == "unknown" {
					t.Errorf("%s %q: %q is unknown", ti.Name, r.Title, p.Text)
				}
			}
		}
	}
	if n == 0 {
		t.Fatal("no recipes")
	}
}

func TestStem(t *testing.T) {
	for _, g := range [][]string{
		{"change", "changes", "changed", "changing"},
		{"file", "files"},
		{"stop", "stopped", "stopping", "stops"},
		{"commit", "commits", "committed"},
		{"process", "processes"},
		{"delete", "deleted", "deleting"},
		{"log", "logs", "logging"},
		{"branch", "branches"},
	} {
		for _, w := range g[1:] {
			if stem(w) != stem(g[0]) {
				t.Errorf("stem(%q) = %q, stem(%q) = %q", w, stem(w), g[0], stem(g[0]))
			}
		}
	}
}

func TestOneEdit(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"push", "pusj", true}, {"push", "psuh", true}, {"push", "pushh", true}, {"push", "pus", true},
		{"push", "push", false}, {"push", "pull", false}, {"stash", "sthsa", false},
	} {
		if got := oneEdit(c.a, c.b); got != c.want {
			t.Errorf("oneEdit(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

// What a query should find first, by purpose rather than by name.
func TestSearch(t *testing.T) {
	for _, c := range []struct {
		query, tool, first string // first: the best result's command
	}{
		{"undo my last commit", "", "git reset --soft HEAD~1"},
		{"take back the last commit", "", "git reset --soft HEAD~1"},
		{"throw away my changes", "", "git reset --hard"},
		{"put my changes aside", "", "git stash push -m {description}"},
		{"follow logs of a pod", "", "kubectl logs -f --tail=100 {pod} -n {namespace}"},
		{"why did my pod crash", "", "kubectl logs -p {pod} -n {namespace}"},
		{"roll back a deployment", "", "kubectl rollout undo deployment/{name} -n {namespace}"},
		{"restart deployment", "", "kubectl rollout restart deployment/{name} -n {namespace}"},
		{"--force-with-lease", "", "git push --force-with-lease"},
		{"stash pop", "", "git stash pop"},
		{"restart", "git", ""}, // nothing in git restarts
		{"pusj new branch", "", "git push -u origin {branch}"}, // a typo
		{"discard changes to a file", "", "git restore {file}"},
		{"who wrote this line", "", "git blame -w {file}"},
		{"find when a bug started", "", "git bisect start {bad} {good}"},
		{"see old version of a file", "", "git show {commit}:{file}"},
		{"shell into a pod", "", "kubectl exec -it {pod} -n {namespace} -- sh"},
		{"pod stuck pending", "", "kubectl describe pod {pod} -n {namespace}"},
		{"switch namespace", "", "kubectl config set-context --current --namespace={namespace}"},
		{"access service locally", "", "kubectl port-forward svc/{service} 8080:80 -n {namespace}"},
		{"node maintenance", "", "kubectl drain {node} --ignore-daemonsets"},
		{"free disk space docker", "", "docker system prune -a"},
		{"build for arm", "", "docker buildx build --platform linux/amd64,linux/arm64 -t {image} --push ."},
		{"reset database compose", "", "docker compose down -v"},
		{"login to registry", "docker", "docker login {server} -u {user} --password-stdin"},
		{"undo helm upgrade", "", "helm rollback {release} -n {namespace}"},
		{"see chart default values", "", "helm show values {chart}"},
		{"what values did i install with", "", "helm get values {release} -n {namespace}"},
	} {
		got, err := Search(c.query, c.tool, 10)
		if err != nil {
			t.Fatal(err)
		}
		if c.first == "" {
			if len(got) != 0 {
				t.Errorf("%q in %s: %v", c.query, c.tool, got[0])
			}
			continue
		}
		if len(got) == 0 || strings.TrimSpace(got[0].Command) != c.first {
			var cmds []string
			for _, r := range got {
				cmds = append(cmds, fmt.Sprintf("%s (%.2f %v)", r.Command, r.Score, r.Why))
			}
			t.Errorf("%q: want %s first, got\n  %s", c.query, c.first, strings.Join(cmds, "\n  "))
		}
	}
}

// A vague query gets several answers, each saying why.
func TestSearchVague(t *testing.T) {
	got, _ := Search("undo", "", 10)
	if len(got) < 3 {
		t.Fatalf("undo: %d results", len(got))
	}
	for _, r := range got {
		if len(r.Why) == 0 {
			t.Errorf("%s: no why", r.Title)
		}
	}
	// An empty query lists the recipes; naming a tool keeps to it.
	all, _ := Search("", "", 0)
	git, _ := Search("", "git", 0)
	named, _ := Search("git", "", 0)
	if len(all) <= len(git) || len(git) == 0 || len(named) != len(git) {
		t.Errorf("browse: %d all, %d git, %d for the query git", len(all), len(git), len(named))
	}
}

// Regression: "--dry-run" in a recipe's command made it the answer to
// "what is running".
func TestSearchIgnoresFlagWords(t *testing.T) {
	got, _ := Search("see what is running", "", 3)
	if len(got) == 0 || !strings.HasPrefix(got[0].Command, "kubectl get pods") {
		t.Errorf("first: %+v", got)
	}
}

// Naming the area ("kubernetes logs") keeps git's log below kubectl's.
func TestSearchGroupHint(t *testing.T) {
	got, _ := Search("kubernetes logs", "", 10)
	if len(got) < 2 || got[0].Tool != "kubectl" || got[1].Tool != "kubectl" {
		t.Errorf("first two: %+v", got)
	}
}

func FuzzSearch(f *testing.F) {
	for _, s := range []string{"undo my last commit", "--force", "-", "pusj", "ééé ß", "a", "kubernetes", "{x}"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, q string) {
		got, err := Search(q, "", 10)
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i < len(got); i++ {
			if got[i].Score > got[i-1].Score {
				t.Fatalf("%q: not in order", q)
			}
		}
	})
}
