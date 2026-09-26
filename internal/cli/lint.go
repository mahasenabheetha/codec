package cli

import (
	"cmp"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/check"
	"github.com/mahasenabheetha/codec/v2/internal/config"
	"github.com/mahasenabheetha/codec/v2/internal/lint"
	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/version"
	"github.com/mahasenabheetha/codec/v2/internal/workspace"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

var (
	lintK8s       string
	lintOffline   bool
	lintNoSchemas bool
	lintJSON      bool
)

var yamlLintCmd = &cobra.Command{
	Use:   "lint [path…]",
	Short: "Check YAML for mistakes: style, Kubernetes risks, removed APIs, schemas",
	Long: `lint checks YAML files and folders (default: the current folder,
honouring .gitignore). It reports syntax errors, style traps (yes/no
booleans, octal-looking numbers, indentation), risky Kubernetes settings,
removed or deprecated Kubernetes APIs for the target version, and
violations of the published schema for the file's type (Kubernetes and
CRDs, GitHub Actions, GitLab CI, Azure Pipelines, Compose).

Schemas are downloaded on first use and cached; --offline uses only the
cache and your custom schema folder. Rule levels come from your codec
settings (the web UI's Lint settings).

Exit status: 0 no warnings or errors, 1 usage error, 2 findings.`,
	Example: `  codec yaml lint
  codec yaml lint charts/ deploy.yaml --k8s-version 1.31
  codec yaml lint . --offline --json`,
	RunE: runLint,
}

func init() {
	f := yamlLintCmd.Flags()
	f.StringVar(&lintK8s, "k8s-version", "", fmt.Sprintf("target Kubernetes version (default %s): %s", lint.DefaultK8sVersion, strings.Join(lint.K8sVersions, ", ")))
	f.BoolVar(&lintOffline, "offline", false, "don't download schemas; use the cache and the custom schema folder")
	f.BoolVar(&lintNoSchemas, "no-schemas", false, "skip schema validation")
	f.BoolVar(&lintJSON, "json", false, "print findings as JSON")
	yamlCmd.AddCommand(yamlLintCmd)
}

type lintResult struct {
	Path        string                  `json:"path"`
	Type        string                  `json:"type"`
	Diagnostics []yamlkit.Diagnostic    `json:"diagnostics"`
	Schemas     []provider.SchemaStatus `json:"schemas,omitempty"`
}

func runLint(cmd *cobra.Command, args []string) error {
	st := config.Lint{}
	if cfg, err := config.Open(); err == nil {
		st = cfg.Get().Lint
	}
	if lintK8s != "" {
		if !slices.Contains(lint.K8sVersions, lintK8s) {
			return fmt.Errorf("--k8s-version %s: choose one of %s", lintK8s, strings.Join(lint.K8sVersions, ", "))
		}
		st.K8sVersion = lintK8s
	}
	st.Offline = st.Offline || lintOffline
	st.NoSchemas = st.NoSchemas || lintNoSchemas
	checker := check.New(provider.Default, st, version.Get().Version)

	if len(args) == 0 {
		args = []string{"."}
	}
	files, err := yamlTargets(cmd.Context(), args)
	if err != nil {
		return err
	}
	results := make([]lintResult, len(files))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8) // parse and validate in parallel, in bounded numbers
	for i, t := range files {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = lintOne(cmd.Context(), checker, t)
		})
	}
	wg.Wait()

	counts := map[yamlkit.Severity]int{}
	for _, r := range results {
		for _, d := range r.Diagnostics {
			counts[d.Severity]++
		}
	}
	if lintJSON {
		if err := emitJSON(results); err != nil {
			return err
		}
	} else {
		for _, r := range results {
			for _, d := range r.Diagnostics {
				fmt.Printf("%s:%d:%d: %s: %s [%s]\n", r.Path, d.Range.Start.Line, d.Range.Start.Col, d.Severity, d.Message, cmp.Or(d.Code, "syntax"))
				if d.Hint != "" {
					fmt.Printf("    fix: %s\n", d.Hint)
				}
			}
			for _, s := range r.Schemas {
				if s.State != "ok" {
					fmt.Fprintf(os.Stderr, "note: %s: %s\n", r.Path, s.Message)
				}
			}
		}
		fmt.Fprintf(os.Stderr, "%d files: %d errors, %d warnings, %d infos\n",
			len(results), counts[yamlkit.SeverityError], counts[yamlkit.SeverityWarning], counts[yamlkit.SeverityInfo])
	}
	if counts[yamlkit.SeverityError]+counts[yamlkit.SeverityWarning] > 0 {
		return exitCode(2)
	}
	return nil
}

// yamlTarget is one file: its path as shown, and how to read it.
type yamlTarget struct {
	show string
	path string // slash path for type detection (relative to its folder)
	read func() ([]byte, error)
}

// yamlTargets expands folders into their YAML files (honouring
// .gitignore, like the workspace view) and keeps named files as given.
func yamlTargets(ctx context.Context, args []string) ([]yamlTarget, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var out []yamlTarget
	for _, a := range args {
		fi, err := os.Stat(a)
		if err != nil {
			return nil, err
		}
		if !fi.IsDir() {
			out = append(out, yamlTarget{show: a, path: filepath.ToSlash(a), read: func() ([]byte, error) { return os.ReadFile(a) }})
			continue
		}
		ws, err := workspace.Open(a)
		if err != nil {
			return nil, err
		}
		files, _, err := ws.Files(ctx)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if f.Lang != "yaml" {
				continue
			}
			show := filepath.ToSlash(filepath.Join(a, filepath.FromSlash(f.Path)))
			show = strings.TrimPrefix(show, "./")
			out = append(out, yamlTarget{show: show, path: f.Path, read: func() ([]byte, error) {
				c, err := ws.Read(f.Path)
				if err != nil {
					return nil, err
				}
				data := []byte(c.Text)
				if c.CRLF {
					data = []byte(strings.ReplaceAll(c.Text, "\n", "\r\n"))
				}
				return data, nil
			}})
		}
	}
	return out, nil
}

func lintOne(ctx context.Context, c *check.Checker, t yamlTarget) lintResult {
	r := lintResult{Path: t.show, Diagnostics: []yamlkit.Diagnostic{}}
	data, err := t.read()
	if err != nil {
		msg := err.Error()
		if pe, ok := err.(*fs.PathError); ok {
			msg = pe.Err.Error()
		}
		r.Diagnostics = append(r.Diagnostics, yamlkit.Diagnostic{Severity: yamlkit.SeverityError, Code: "read", Message: "Can't read the file: " + msg})
		return r
	}
	f := &provider.File{Path: t.path, Content: data, YAML: yamlkit.Parse(data)}
	a := c.Analyze(ctx, f, 45*time.Second)
	r.Type, r.Diagnostics = a.Type, a.Diagnostics
	for _, d := range a.Docs {
		if d.Schema != nil {
			r.Schemas = append(r.Schemas, *d.Schema)
		}
	}
	return r
}
