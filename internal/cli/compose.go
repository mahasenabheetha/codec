package cli

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/compose"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

var (
	composeEnv     []string
	composeEnvFile string
	composeJSON    bool
)

var composeCmd = &cobra.Command{
	Use:   "compose",
	Short: "Understand Docker Compose projects: merged files, services, variables",
}

var composeServicesCmd = &cobra.Command{
	Use:   "services [compose files…]",
	Short: "List a Compose project's services with ports, volumes and start order",
	Long: `services reads a Compose project as docker compose would: the files
given (in merge order), or compose.yaml and its override in the current
folder; ${VAR} from .env and -e values (your shell's environment is not
read). It lists services, what each starts after, ports, volumes and
variables.

Exit status: 0 fine, 1 error, 2 problems (unknown services, undeclared
networks or volumes, unset required variables…).`,
	Example: "  codec compose services\n  codec compose services compose.yaml compose.prod.yaml -e TAG=1.2",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := composeAnalyze(args)
		if err != nil {
			return err
		}
		if composeJSON {
			if err := emitJSON(p); err != nil {
				return err
			}
		} else {
			printCompose(os.Stdout, p)
		}
		return composeProblems(p)
	},
}

var composeConfigCmd = &cobra.Command{
	Use:   "config [compose files…]",
	Short: "Print the merged configuration of each service and which file set each line",
	Long: `config merges the project's files with the Compose merge rules (ports
and volumes merge, commands replace, !reset and !override apply),
applies extends and ${VAR}, and prints every service with the file and
line each field came from.`,
	Example: "  codec compose config\n  codec compose config compose.yaml compose.override.yaml",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := composeAnalyze(args)
		if err != nil {
			return err
		}
		if composeJSON {
			return emitJSON(p.Services)
		}
		for _, s := range p.Services {
			fmt.Println(s.Name + ":")
			width := 0
			for _, l := range s.Effective {
				width = max(width, len(l.Text))
			}
			for _, l := range s.Effective {
				note := ""
				if l.Source != nil {
					note = fmt.Sprintf("# ← %s:%d", path.Base(l.Source.File), l.Source.Line)
					if l.From != "" {
						note += " (" + l.From + ")"
					}
				}
				fmt.Printf("  %-*s  %s\n", width, l.Text, note)
			}
		}
		return nil
	},
}

func init() {
	for _, c := range []*cobra.Command{composeServicesCmd, composeConfigCmd} {
		c.Flags().StringArrayVarP(&composeEnv, "env", "e", nil, "set a variable, NAME=value (repeatable; wins over .env)")
		c.Flags().StringVar(&composeEnvFile, "env-file", "", `env file instead of .env ("none" for no file)`)
		c.Flags().BoolVar(&composeJSON, "json", false, "print JSON")
	}
	composeCmd.AddCommand(composeServicesCmd, composeConfigCmd)
	rootCmd.AddCommand(composeCmd)
}

// composeAnalyze reads a project from its files, relative to the
// folder holding .git (or the current folder).
func composeAnalyze(args []string) (*compose.Project, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	first := "compose.yaml"
	if len(args) > 0 {
		first = args[0]
	}
	abs, err := filepath.Abs(first)
	if err != nil {
		return nil, err
	}
	root := repoRoot(abs)
	all := listFiles(root)
	rel := func(p string) (string, error) {
		a, err := filepath.Abs(p)
		if err != nil {
			return "", err
		}
		r, err := filepath.Rel(root, a)
		if err != nil || strings.HasPrefix(r, "..") {
			return "", fmt.Errorf("%s is outside %s", p, root)
		}
		return filepath.ToSlash(r), nil
	}
	var files []string
	if len(args) == 0 {
		// Like docker compose: the first compose file here, with its override.
		dir, _ := rel(wd)
		for _, n := range []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"} {
			if f := path.Join(dir, n); slices.Contains(all, f) {
				files = compose.DefaultFiles(f, all)
				break
			}
		}
		if files == nil {
			return nil, fmt.Errorf("no compose.yaml or docker-compose.yml here; name the files")
		}
	}
	for _, a := range args {
		r, err := rel(a)
		if err != nil {
			return nil, err
		}
		files = append(files, r)
	}
	env := map[string]string{}
	for _, e := range composeEnv {
		k, v, ok := strings.Cut(e, "=")
		if !ok {
			return nil, fmt.Errorf("-e %q: expected NAME=value", e)
		}
		env[k] = v
	}
	envFile := composeEnvFile
	if envFile != "" && envFile != "none" {
		if envFile, err = rel(envFile); err != nil {
			return nil, err
		}
	}
	return compose.Analyze(files, compose.Options{Files: all, Env: env, EnvFile: envFile, Load: func(p string) ([]byte, error) {
		return os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
	}}), nil
}

func printCompose(w io.Writer, p *compose.Project) {
	var layers []string
	for _, f := range p.Files {
		if f.Role == "layer" {
			layers = append(layers, f.Path)
		}
	}
	fmt.Fprintf(w, "project %s · %s\n", p.Name, strings.Join(layers, " + "))
	if p.EnvFile != "" {
		fmt.Fprintf(w, "  variables from %s\n", p.EnvFile)
	}
	for _, s := range p.Services {
		line := "\n" + s.Name
		switch {
		case s.Image != "":
			line += "  · " + s.Image
		case s.Build != "":
			line += "  · build " + s.Build
		}
		if len(s.Profiles) > 0 {
			line += "  · profiles " + strings.Join(s.Profiles, ", ")
		}
		fmt.Fprintln(w, line)
		if len(s.DependsOn) > 0 {
			var deps []string
			for _, d := range s.DependsOn {
				dep := d.Service
				if c := strings.TrimPrefix(d.Condition, "service_"); c != "" && c != "started" {
					dep += " (" + c + ")"
				}
				deps = append(deps, dep)
			}
			fmt.Fprintf(w, "  after %s\n", strings.Join(deps, ", "))
		}
		if s.Extends != "" {
			fmt.Fprintf(w, "  extends %s\n", s.Extends)
		}
		for _, pt := range p.Ports {
			if pt.Service == s.Name {
				fmt.Fprintf(w, "  port %s  (%s)\n", portLine(pt), path.Base(pt.Layer))
			}
		}
		for _, m := range p.Mounts {
			if m.Service == s.Name {
				ro := ""
				if m.ReadOnly {
					ro = " (read-only)"
				}
				fmt.Fprintf(w, "  %-6s %s → %s%s\n", m.Type, orFile(m.From, "(anonymous)"), m.Target, ro)
			}
		}
		if len(s.Files) > 1 {
			fmt.Fprintf(w, "  set in %s\n", strings.Join(s.Files, ", "))
		}
	}
	if len(p.Variables) > 0 {
		fmt.Fprintln(w, "\nvariables")
		for _, v := range p.Variables {
			val := v.Value
			switch {
			case v.From == "default":
				val = v.Default + "  (default)"
			case !v.Set:
				val = "(not set)"
			case v.From != "":
				val += "  (" + v.From + ")"
			}
			fmt.Fprintf(w, "  %-22s %s\n", v.Name, val)
		}
	}
}

func portLine(pt compose.Port) string {
	s := pt.Target + "/" + pt.Protocol
	if pt.Published != "" {
		s = pt.Published + " → " + s
	}
	if pt.HostIP != "" {
		s = pt.HostIP + ":" + s
	}
	return s
}

// composeProblems reports problems on stderr; any but info is exit
// status 2.
func composeProblems(p *compose.Project) error {
	bad := 0
	for _, pr := range p.Problems {
		at := ""
		if pr.Source != nil {
			at = fmt.Sprintf("%s:%d: ", pr.Source.File, pr.Source.Line)
		}
		fmt.Fprintf(os.Stderr, "%s%s: %s\n", at, pr.Severity, pr.Message)
		if pr.Hint != "" {
			fmt.Fprintf(os.Stderr, "    %s\n", pr.Hint)
		}
		if pr.Severity != yamlkit.SeverityInfo {
			bad++
		}
	}
	if bad > 0 {
		return exitCode(2)
	}
	return nil
}
