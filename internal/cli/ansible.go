package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/ansible"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

var (
	ansRoot string
	ansJSON bool
	ansPath string
)

var ansibleCmd = &cobra.Command{
	Use:   "ansible",
	Short: "Understand Ansible playbooks, roles and YAML inventories",
}

var ansiblePlaysCmd = &cobra.Command{
	Use:   "plays <playbook>",
	Short: "List a playbook's plays and tasks in the order Ansible runs them",
	Long: `plays reads a playbook and prints each play's steps in execution order:
fact gathering, pre_tasks, roles (their dependencies first), tasks and
post_tasks, with handlers flushed after each part. Roles and
import/include_tasks are followed into their files in the repository
(--root, by default the folder holding .git).

Exit status: 0 fine, 1 error, 2 problems (missing roles or files,
unknown handlers).`,
	Example: "  codec ansible plays playbooks/site.yml\n  codec ansible plays site.yml --json",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pb, err := ansibleAnalyze(args[0])
		if err != nil {
			return err
		}
		if ansJSON {
			if err := emitJSON(pb); err != nil {
				return err
			}
		} else {
			printPlaybook(os.Stdout, pb)
		}
		return ansibleProblems(pb.Problems)
	},
}

var ansibleVarsCmd = &cobra.Command{
	Use:     "vars <playbook> [name]",
	Short:   "Show where a playbook's variables are defined, highest precedence first",
	Example: "  codec ansible vars playbooks/site.yml\n  codec ansible vars playbooks/site.yml app_port",
	Args:    cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		pb, err := ansibleAnalyze(args[0])
		if err != nil {
			return err
		}
		defs := pb.Variables
		if len(args) == 2 {
			defs = pb.Definitions(args[1])
			if len(defs) == 0 {
				return fmt.Errorf("%s isn't defined in the files codec read (it may come from the inventory, -e, a fact or a registered result)", args[1])
			}
		}
		if ansJSON {
			return emitJSON(defs)
		}
		last := ""
		for _, d := range defs {
			if d.Name != last {
				fmt.Println(d.Name)
				last = d.Name
			}
			scope := ""
			if d.Scope != "" {
				scope = " (" + d.Scope + ")"
			}
			fmt.Printf("  %-26s %s  %s\n", d.Kind+scope, d.Value, where(d.Source))
		}
		return nil
	},
}

var ansibleTaskCmd = &cobra.Command{
	Use:   "task <task name from a log>",
	Short: "Find the definition of a task named in an Ansible log",
	Long: `task finds where a task from a log is defined. Give the name as the log
shows it ("TASK [role : name]" → "role : name"), and --path with the
log's "task path:" when it has one. Names written with {{ variables }}
match whatever values the run had.`,
	Example: `  codec ansible task "nginx : restart nginx"
  codec ansible task "Deploy app 2.0" --path /builds/team/repo/playbooks/site.yml:9`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root := ansRoot
		if root == "" {
			root = "."
		}
		root, err := filepath.Abs(root)
		if err != nil {
			return err
		}
		ms := ansible.FindTask(args[0], ansPath, listFiles(root), func(p string) ([]byte, error) {
			return os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		})
		if ansJSON {
			return emitJSON(ms)
		}
		if len(ms) == 0 {
			return fmt.Errorf("no task named %q under %s", args[0], root)
		}
		for _, m := range ms {
			fmt.Printf("%s:%d  %s  (%s)\n", m.File, m.Line, m.Name, m.Why)
		}
		return nil
	},
}

var ansibleInventoryCmd = &cobra.Command{
	Use:     "inventory <inventory.yml>",
	Short:   "Show a YAML inventory as a tree of groups and hosts",
	Example: "  codec ansible inventory inventory/hosts.yml",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		inv := ansible.ReadInventory(filepath.ToSlash(args[0]), yamlkit.Parse(data))
		if ansJSON {
			return emitJSON(inv)
		}
		printInventory(os.Stdout, inv)
		return nil
	},
}

func init() {
	for _, c := range []*cobra.Command{ansiblePlaysCmd, ansibleVarsCmd, ansibleTaskCmd} {
		c.Flags().StringVar(&ansRoot, "root", "", "repository root for roles and included files (default: the folder holding .git; for task, the current folder)")
	}
	for _, c := range []*cobra.Command{ansiblePlaysCmd, ansibleVarsCmd, ansibleTaskCmd, ansibleInventoryCmd} {
		c.Flags().BoolVar(&ansJSON, "json", false, "print JSON")
	}
	ansibleTaskCmd.Flags().StringVar(&ansPath, "path", "", `the log's "task path:" (file:line)`)
	ansibleCmd.AddCommand(ansiblePlaysCmd, ansibleVarsCmd, ansibleTaskCmd, ansibleInventoryCmd)
	rootCmd.AddCommand(ansibleCmd)
}

// ansibleAnalyze reads a playbook with the repository around it.
func ansibleAnalyze(file string) (*ansible.Playbook, error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	root := ansRoot
	if root == "" {
		root = repoRoot(abs)
	}
	if root, err = filepath.Abs(root); err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return nil, fmt.Errorf("%s is outside the repository root %s (set --root)", file, root)
	}
	rel = filepath.ToSlash(rel)
	opts := ansible.Options{Files: listFiles(root), Load: func(p string) ([]byte, error) {
		return os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
	}}
	return ansible.Analyze(rel, data, yamlkit.Parse(data), opts), nil
}

func where(s *ansible.Source) string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("%s:%d", s.File, s.Line)
}

func printPlaybook(w io.Writer, pb *ansible.Playbook) {
	var steps func(ts []ansible.Task, indent string)
	steps = func(ts []ansible.Task, indent string) {
		for _, t := range ts {
			line := indent + t.Name
			switch t.Kind {
			case "section":
				line = indent + t.Name + ":"
			case "facts", "flush":
				line = indent + "(" + strings.ToLower(t.Name) + ")"
			case "role":
				line = indent + "role " + t.Name
			default:
				if m := strings.TrimPrefix(strings.TrimPrefix(t.Module, "ansible.builtin."), "ansible.legacy."); m != "" && !strings.HasPrefix(t.Name, m) {
					line += "  · " + m
				}
			}
			if t.When != "" {
				line += "  · when " + t.When
			}
			if t.Dynamic {
				line += "  · dynamic"
			}
			if t.Missing {
				line += "  · NOT FOUND"
			}
			if len(t.Notify) > 0 {
				line += "  → notifies " + strings.Join(t.Notify, ", ")
			}
			fmt.Fprintln(w, line)
			steps(t.Children, indent+"  ")
		}
	}
	if pb.Kind == "tasks" {
		head := "task file " + pb.File
		if pb.Role != "" {
			head += " (role " + pb.Role + ")"
		}
		fmt.Fprintln(w, head)
		steps(pb.Tasks, "  ")
	}
	for _, pl := range pb.Plays {
		head := "\nplay " + pl.Name
		if pl.Hosts != "" && pl.Hosts != pl.Name {
			head += "  · hosts " + pl.Hosts
		}
		if pl.Imported != "" {
			head += "  · from " + pl.Imported
		}
		fmt.Fprintln(w, head)
		steps(pl.Steps, "  ")
		if len(pl.Handlers) > 0 {
			fmt.Fprintln(w, "  handlers:")
			steps(pl.Handlers, "    ")
		}
	}
	if len(pb.Roles) > 0 {
		fmt.Fprintln(w, "\nroles")
		for _, r := range pb.Roles {
			switch {
			case r.External:
				fmt.Fprintf(w, "  %-28s %s\n", r.Name, r.Origin())
			case !r.Found:
				fmt.Fprintf(w, "  %-28s NOT FOUND\n", r.Name)
			default:
				line := fmt.Sprintf("  %-28s %s  (%s)", r.Name, r.Path, strings.Join(r.Parts, ", "))
				if len(r.Dependencies) > 0 {
					line += "  depends on " + strings.Join(r.Dependencies, ", ")
				}
				fmt.Fprintln(w, line)
			}
		}
	}
}

// ansibleProblems reports problems on stderr; any but info is exit
// status 2.
func ansibleProblems(ps []ansible.Problem) error {
	bad := 0
	for _, pr := range ps {
		at := ""
		if s := where(pr.Source); s != "" {
			at = s + ": "
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

func printInventory(w io.Writer, inv *ansible.Inventory) {
	groups := map[string]ansible.Group{}
	for _, g := range inv.Groups {
		groups[g.Name] = g
	}
	counts := map[string]int{}
	for _, h := range inv.Hosts {
		counts[h.Name] = h.Count
	}
	var group func(name, indent string, seen map[string]bool)
	group = func(name, indent string, seen map[string]bool) {
		g := groups[name]
		line := indent + name
		if len(g.Vars) > 0 {
			var vs []string
			for _, v := range g.Vars {
				vs = append(vs, v.Name+"="+v.Value)
			}
			line += "  · " + strings.Join(vs, ", ")
		}
		fmt.Fprintln(w, line)
		if seen[name] {
			return
		}
		seen[name] = true
		for _, h := range g.Hosts {
			hl := indent + "  - " + h
			if counts[h] > 1 {
				hl += fmt.Sprintf("  (%d hosts)", counts[h])
			}
			fmt.Fprintln(w, hl)
		}
		for _, c := range g.Children {
			group(c, indent+"  ", seen)
		}
		delete(seen, name)
	}
	roots := []string{}
	for _, g := range inv.Groups {
		if g.Name == "all" || !isChildGroup(inv.Groups, g.Name) {
			roots = append(roots, g.Name)
		}
	}
	for _, r := range roots {
		group(r, "", map[string]bool{})
	}
}

func isChildGroup(gs []ansible.Group, name string) bool {
	for _, g := range gs {
		for _, c := range g.Children {
			if c == name {
				return true
			}
		}
	}
	return false
}
