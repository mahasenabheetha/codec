package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/config"
	"github.com/mahasenabheetha/codec/v2/internal/scaffold"
)

var (
	newSet    []string
	newFile   string
	cloneTo   string
	cloneFrom string
	cloneDoc  int
	cloneAll  bool
)

var newCmd = &cobra.Command{
	Use:   "new [starter]",
	Short: "Print a new file from a starter template (Kubernetes, Helm, Argo, CI, Compose, Ansible)",
	Long: `new fills a starter in and prints it; it never writes files. Without
a starter it lists them: the built-in ones and your own from the
"templates" folder in codec's settings folder (a folder per starter, or
single files, with <% .name %> placeholders).

A starter with several files (a Helm chart, an Ansible role) prints
each after a "# ==> path <==" line; --file prints one of them alone.`,
	Example: `  codec new
  codec new k8s-app --set name=shop --set ingress=true --set host=shop.example.com
  codec new helm-chart --set name=web --file web/values.yaml`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		all := scaffold.Builtin()
		dir, _ := config.TemplatesDir()
		if dir != "" {
			if _, err := os.Stat(dir); err == nil {
				personal, errs := scaffold.Load(os.DirFS(dir), true)
				for _, e := range errs {
					fmt.Fprintln(os.Stderr, "warning: personal template", e)
				}
				all = append(all, personal...)
			}
		}
		if len(args) == 0 {
			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			for _, s := range all {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", s.ID, s.Category, s.Title)
			}
			tw.Flush()
			if dir != "" {
				fmt.Fprintln(os.Stderr, "\nPersonal templates:", dir)
			}
			return nil
		}
		st := scaffold.Find(all, args[0])
		if st == nil {
			return fmt.Errorf("no starter %q (codec new lists them)", args[0])
		}
		values := map[string]string{}
		for _, kv := range newSet {
			k, v, ok := strings.Cut(kv, "=")
			if !ok {
				return fmt.Errorf("--set %q: want name=value", kv)
			}
			values[k] = v
		}
		files, problems, err := st.Render(values)
		if err != nil {
			return err
		}
		if len(problems) > 0 {
			for _, p := range problems {
				fmt.Fprintf(os.Stderr, "%s: %s\n", p.Field, p.Message)
			}
			fmt.Fprintln(os.Stderr, "\nFields:", fieldList(st))
			return exitCode(1)
		}
		if newFile != "" {
			for _, f := range files {
				if f.Path == newFile {
					return emit(strings.TrimSuffix(f.Content, "\n"))
				}
			}
			return fmt.Errorf("the starter makes no %s", newFile)
		}
		if len(files) == 1 {
			return emit(strings.TrimSuffix(files[0].Content, "\n"))
		}
		var b strings.Builder
		for i, f := range files {
			if i > 0 {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "# ==> %s <==\n%s", f.Path, f.Content)
		}
		return emit(strings.TrimSuffix(b.String(), "\n"))
	},
}

// fieldList describes a starter's fields for an error message.
func fieldList(s *scaffold.Starter) string {
	var out []string
	for _, f := range s.Fields {
		d := f.Name
		if f.Default != "" {
			d += "=" + f.Default
		}
		out = append(out, d)
	}
	return strings.Join(out, " ")
}

var cloneCmd = &cobra.Command{
	Use:   "clone <file> --to <name>",
	Short: "Print a copy of a YAML file (or one document) under a new name",
	Long: `clone renames every whole-word use of the old name (metadata.name, or
the top-level name, unless --from says otherwise): labels, template and
job names, internal references. Comments and layout are kept. It prints
the copy and never writes files.

References to objects that aren't part of the copy, Secrets, images and
addresses are left alone and listed on stderr as suggestions; --all
renames them too. Fields copies usually change (images, hosts, Secret
references) are listed to check.`,
	Example: `  codec clone deploy/shop.yaml --to cart
  codec clone workflows/build-api.yaml --to build-web --all
  codec clone app.yaml --doc 1 --to cart`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		src, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		opts := scaffold.CloneOptions{Doc: cloneDoc, From: cloneFrom, To: cloneTo}
		res, err := scaffold.Clone(src, opts)
		if err != nil {
			return err
		}
		if cloneAll {
			for _, c := range res.Changes {
				if c.Kind == "suggest" {
					opts.Flip = append(opts.Flip, c.ID)
				}
			}
			if res, err = scaffold.Clone(src, opts); err != nil {
				return err
			}
		}
		for _, c := range res.Changes {
			switch {
			case c.Kind == "suggest" && !c.Applied:
				fmt.Fprintf(os.Stderr, "line %d: kept %s: %s (%s)\n", c.Line, c.Path, c.Before, c.Reason)
			case c.Kind == "check":
				fmt.Fprintf(os.Stderr, "line %d: check %s: %s (%s)\n", c.Line, c.Path, c.Before, c.Reason)
			}
		}
		return emit(strings.TrimSuffix(res.Text, "\n"))
	},
}

func init() {
	newCmd.Flags().StringArrayVar(&newSet, "set", nil, "a field value, name=value (repeatable)")
	newCmd.Flags().StringVar(&newFile, "file", "", "print only this generated file")
	cloneCmd.Flags().StringVar(&cloneTo, "to", "", "the new name (required)")
	cloneCmd.Flags().StringVar(&cloneFrom, "from", "", "the name to replace (default: metadata.name, or the top-level name)")
	cloneCmd.Flags().IntVar(&cloneDoc, "doc", -1, "clone only this document (0-based; default: the whole file)")
	cloneCmd.Flags().BoolVar(&cloneAll, "all", false, "also rename references outside the copy, Secrets and images")
	cloneCmd.MarkFlagRequired("to")
	rootCmd.AddCommand(newCmd, cloneCmd)
}
