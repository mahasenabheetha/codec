package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/kube"
	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

var k8sJSON bool

var k8sCmd = &cobra.Command{
	Use:   "k8s",
	Short: "Understand Kubernetes manifests: images, references, neat copies",
}

var k8sImagesCmd = &cobra.Command{
	Use:     "images [file|folder…]",
	Short:   "List container images and who runs them",
	Example: "  codec k8s images charts/ deploy/\n  helm template ./chart | codec k8s images",
	RunE: func(cmd *cobra.Command, args []string) error {
		objs, err := k8sObjects(cmd.Context(), args)
		if err != nil {
			return err
		}
		inv := kube.Summarize(objs, kube.Relate(objs))
		if k8sJSON {
			return emitJSON(inv.Images)
		}
		for _, im := range inv.Images {
			note := ""
			if !im.Pinned {
				note = "  (not pinned)"
			}
			fmt.Printf("%s%s\n", im.Ref, note)
			for _, u := range im.UsedBy {
				fmt.Printf("    %s\n", u)
			}
		}
		return nil
	},
}

var k8sRefsCmd = &cobra.Command{
	Use:   "refs [file|folder…]",
	Short: "Show how objects connect, and references that point nowhere",
	Long: `refs works out the relationships between the objects in the given
files or folders (or stdin): Service → pods (selector vs labels),
Ingress → Service:port, workloads → ConfigMaps, Secrets, PVCs and
ServiceAccounts, RBAC bindings, HPA targets. References to objects not
in the scope are reported as warnings — they may live elsewhere.

Exit status: 0 no warnings, 1 error, 2 warnings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		objs, err := k8sObjects(cmd.Context(), args)
		if err != nil {
			return err
		}
		g := kube.Relate(objs)
		if k8sJSON {
			return emitJSON(g)
		}
		names := map[string]string{}
		for _, n := range g.Nodes {
			label := n.Kind + " " + n.Name
			if n.Missing {
				label += " (not in scope)"
			}
			names[n.ID] = label
		}
		last := ""
		for _, e := range g.Edges {
			if e.From != last {
				fmt.Println(names[e.From])
				last = e.From
			}
			label := ""
			if e.Label != "" {
				label = " (" + e.Label + ")"
			}
			fmt.Printf("  %s → %s%s\n", e.Kind, names[e.To], label)
		}
		warnings := 0
		for _, f := range g.Findings {
			if f.Severity == "warning" {
				warnings++
			}
			where := f.Source.File
			if where == "" {
				where = "<stdin>"
			}
			fmt.Fprintf(os.Stderr, "%s:%d: %s: %s\n", where, f.Source.Line, f.Severity, f.Message)
			if f.Hint != "" {
				fmt.Fprintf(os.Stderr, "    %s\n", f.Hint)
			}
		}
		if warnings > 0 {
			return exitCode(2)
		}
		return nil
	},
}

var k8sNeatCmd = &cobra.Command{
	Use:   "neat [file]",
	Short: "Strip cluster noise (status, managedFields, uid, timestamps…) from manifests",
	Long: `neat prints manifests without what the cluster adds: status,
managedFields, uid, resourceVersion, generation, timestamps, owner
references and the last-applied annotation. Comments and key order are
kept. Reads the file, or stdin:

  kubectl get deploy api -o yaml | codec k8s neat`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, _, err := readSource(args)
		if err != nil {
			return err
		}
		out, err := kube.Neat(data)
		if err != nil {
			return err
		}
		fmt.Print(string(out))
		return nil
	},
}

var kustomizeCmd = &cobra.Command{
	Use:   "kustomize",
	Short: "Build Kustomize overlays in-process (no kubectl or kustomize needed)",
}

var kustomizeBuildCmd = &cobra.Command{
	Use:   "build <dir>",
	Short: "Build a kustomization like 'kubectl kustomize <dir>'",
	Long: `build renders the kustomization in dir with the embedded Kustomize
library, like 'kubectl kustomize'. Remote bases (git/http URLs) and
helmCharts aren't supported: codec never downloads or runs helm.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		abs, err := filepath.Abs(args[0])
		if err != nil {
			return err
		}
		dir := slashKey(abs)
		files, err := kube.KustomizeFiles(osReader{}, dir)
		if err != nil {
			return err
		}
		out, err := kube.Kustomize(files, dir)
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	},
}

func init() {
	for _, c := range []*cobra.Command{k8sImagesCmd, k8sRefsCmd} {
		c.Flags().BoolVar(&k8sJSON, "json", false, "print JSON")
	}
	k8sCmd.AddCommand(k8sImagesCmd, k8sRefsCmd, k8sNeatCmd)
	kustomizeCmd.AddCommand(kustomizeBuildCmd)
	rootCmd.AddCommand(k8sCmd, kustomizeCmd)
}

// k8sObjects reads objects from files and folders, or stdin. Raw Helm
// templates are skipped (render them first).
func k8sObjects(ctx context.Context, args []string) ([]kube.Object, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(args) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
		return kube.Collect("", yamlkit.Parse(data)), nil
	}
	targets, err := yamlTargets(ctx, args)
	if err != nil {
		return nil, err
	}
	var objs []kube.Object
	for _, t := range targets {
		data, err := t.read()
		if err != nil {
			return nil, err
		}
		f := yamlkit.Parse(data)
		switch provider.Default.Best(&provider.File{Path: t.path, YAML: f}).ID() {
		case "helm-template", "helm-values", "helm-chart":
			continue
		}
		objs = append(objs, kube.Collect(t.show, f)...)
	}
	return objs, nil
}

// osReader reads absolute paths given as slash keys (see slashKey).
type osReader struct{}

func (osReader) ReadFile(p string) ([]byte, error) { return os.ReadFile(fromKey(p)) }

func (osReader) IsDir(p string) bool {
	fi, err := os.Stat(fromKey(p))
	return err == nil && fi.IsDir()
}

// slashKey turns an absolute path into a relative-looking slash path
// ("C/work/app" or "home/me/app"), the form kustomize's in-memory
// filesystem is fed with. The drive colon goes: a "C:" inside a memfs
// path sends kyaml's path splitting on Windows into endless recursion.
func slashKey(abs string) string {
	s := strings.TrimPrefix(filepath.ToSlash(abs), "/")
	if vol := filepath.VolumeName(abs); len(vol) == 2 && vol[1] == ':' {
		s = vol[:1] + s[2:]
	}
	return s
}

func fromKey(p string) string {
	if runtime.GOOS == "windows" {
		if i := strings.IndexByte(p, '/'); i == 1 {
			p = p[:1] + ":" + p[1:] // C/work → C:/work
		}
		return filepath.FromSlash(p)
	}
	return "/" + p
}
