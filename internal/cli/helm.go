package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/helm"
	"github.com/mahasenabheetha/codec/v2/internal/workspace"
)

// Flags shared by the helm subcommands, mirroring `helm template`.
var (
	helmValues      []string
	helmSet         []string
	helmRelease     string
	helmNamespace   string
	helmKubeVersion string
	helmAPIVersions []string
	helmIncludeCRDs bool
	helmProvenance  bool
)

var helmCmd = &cobra.Command{
	Use:   "helm",
	Short: "Render Helm charts and explain their values (no helm install needed)",
	Long: `helm renders charts with the embedded Helm 4 SDK, exactly like
'helm template' of the same version, and shows where every value came
from. Charts are read from disk and never modified; dependencies must be
vendored in charts/ (codec never downloads anything).`,
}

var helmRenderCmd = &cobra.Command{
	Use:   "render <chart-dir>",
	Short: "Render a chart like 'helm template' (manifests to stdout)",
	Example: `  codec helm render ./charts/app -f values-prod.yaml --set image.tag=1.27
  codec helm render . --release web --namespace prod --kube-version 1.31`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := renderChart(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		fmt.Print(res.Manifest)
		return reportHelm(res)
	},
}

var helmValuesCmd = &cobra.Command{
	Use:     "values <chart-dir>",
	Short:   "Print the merged values; --provenance says where each came from",
	Example: `  codec helm values ./charts/app -f values-prod.yaml --provenance`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := renderChart(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		text, lines := helm.ValuesYAML(res.Values)
		if !helmProvenance {
			fmt.Print(text)
			return reportHelm(res)
		}
		// Annotate each key line with the layer that won.
		for i, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
			chain := res.Provenance[lines[i+1]]
			if len(chain) == 0 {
				fmt.Println(line)
				continue
			}
			fmt.Printf("%s  # ← %s\n", line, originText(chain))
		}
		return reportHelm(res)
	},
}

func init() {
	for _, c := range []*cobra.Command{helmRenderCmd, helmValuesCmd} {
		f := c.Flags()
		f.StringArrayVarP(&helmValues, "values", "f", nil, "values file (repeatable; later files win)")
		f.StringArrayVar(&helmSet, "set", nil, "set a value, e.g. image.tag=1.27 (repeatable)")
		f.StringVar(&helmRelease, "release", "", `release name (default "release-name")`)
		f.StringVarP(&helmNamespace, "namespace", "n", "", `namespace (default "default")`)
		f.StringVar(&helmKubeVersion, "kube-version", "", "Kubernetes version for .Capabilities")
		f.StringArrayVarP(&helmAPIVersions, "api-versions", "a", nil, "extra API versions for .Capabilities")
		f.BoolVar(&helmIncludeCRDs, "include-crds", false, "include CRDs in the output")
		helmCmd.AddCommand(c)
	}
	helmValuesCmd.Flags().BoolVar(&helmProvenance, "provenance", false, "annotate each value with the layer that set it")
	rootCmd.AddCommand(helmCmd)
}

// renderChart loads a chart directory (honouring .helmignore) and the
// values files, then renders.
func renderChart(ctx context.Context, dir string) (*helm.Result, error) {
	if _, err := os.Stat(filepath.Join(dir, "Chart.yaml")); err != nil {
		return nil, fmt.Errorf("%s: no Chart.yaml here: %w", dir, helm.ErrNotChart)
	}
	ws, err := workspace.Open(dir)
	if err != nil {
		return nil, err
	}
	defer ws.Close()
	ignoreRules, _ := os.ReadFile(filepath.Join(dir, ".helmignore"))
	skip, err := helm.Ignorer(ignoreRules)
	if err != nil {
		return nil, fmt.Errorf(".helmignore: %w", err)
	}
	tree, err := ws.ReadTree("", skip)
	if err != nil {
		return nil, err
	}
	files := make([]helm.File, len(tree))
	for i, f := range tree {
		files[i] = helm.File{Name: f.Path, Data: f.Data}
	}

	opts := helm.Options{Set: helmSet, Release: helmRelease, Namespace: helmNamespace,
		KubeVersion: helmKubeVersion, APIVersions: helmAPIVersions, IncludeCRDs: helmIncludeCRDs}
	for _, v := range helmValues {
		data, err := os.ReadFile(v)
		if err != nil {
			return nil, err
		}
		opts.Values = append(opts.Values, helm.Layer{Name: filepath.ToSlash(v), Data: data})
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return helm.Render(ctx, files, opts), nil
}

// reportHelm prints diagnostics to stderr: errors always, warnings too;
// the "unused value" infos only matter in the UI. Exit 1 on errors.
func reportHelm(res *helm.Result) error {
	failed := false
	for _, d := range res.Diagnostics {
		if d.Severity == "info" {
			continue
		}
		failed = failed || d.Severity == "error"
		where := d.File
		if d.Line > 0 {
			where += fmt.Sprintf(":%d", d.Line)
			if d.Col > 0 {
				where += fmt.Sprintf(":%d", d.Col)
			}
		}
		fmt.Fprintf(os.Stderr, "%s: %s: %s\n", where, d.Severity, d.Message)
		if d.Hint != "" {
			fmt.Fprintf(os.Stderr, "    hint: %s\n", d.Hint)
		}
	}
	if failed {
		return errors.New("render failed")
	}
	return nil
}

func originText(chain []helm.Origin) string {
	where := func(o helm.Origin) string {
		if o.Line > 0 {
			return fmt.Sprintf("%s:%d", o.Layer, o.Line)
		}
		return o.Layer
	}
	s := where(chain[0])
	if len(chain) > 1 {
		var rest []string
		for _, o := range chain[1:] {
			rest = append(rest, where(o))
		}
		s += " (overrides " + strings.Join(rest, ", ") + ")"
	}
	return s
}
