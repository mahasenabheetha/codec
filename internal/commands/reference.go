package commands

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"sync"

	"go.yaml.in/yaml/v3"
)

// The reference: one YAML file per tool in packs/, in codec's own words
// and checked against the tool's --help. Tools, Lookup and the explainer
// read it from here.

// Tool is one command-line tool: its root command and what it is called.
type Tool struct {
	Command `yaml:",inline"`
	// Group says where a tool sits when they are listed: git,
	// kubernetes, containers, linux.
	Group string `yaml:"group" json:"group"`
}

// Command is a tool or one of its subcommands.
type Command struct {
	Name    string   `yaml:"name" json:"name"`
	Aliases []string `yaml:"aliases" json:"aliases,omitempty"` // other names: "co" for checkout, "rm" for remove
	Summary string   `yaml:"summary" json:"summary"`           // what it does, one sentence
	Danger  *Danger  `yaml:"danger" json:"danger,omitempty"`   // what it can break, when it can
	Flags   []Flag   `yaml:"flags" json:"flags,omitempty"`
	Args    []Arg    `yaml:"args" json:"args,omitempty"`
	// Commands are its subcommands ("git stash push", "kubectl rollout
	// restart").
	Commands []*Command `yaml:"commands" json:"commands,omitempty"`
}

// Flag is an option: "-m", "--message".
type Flag struct {
	// Names are all its spellings, short and long. "-<n>" stands for a
	// number written as a flag (head -20, git log -3).
	Names []string `yaml:"names" json:"names"`
	// Value names the value it takes ("message", "namespace"); "" for a
	// flag that takes none.
	Value string `yaml:"value" json:"value,omitempty"`
	// OptionalValue: the value is optional and only ever written after
	// "=" (--dry-run[=client], --color[=when]).
	OptionalValue bool     `yaml:"optionalValue" json:"optionalValue,omitempty"`
	Choices       []Choice `yaml:"choices" json:"choices,omitempty"`
	Repeat        bool     `yaml:"repeat" json:"repeat,omitempty"` // may be given more than once
	Summary       string   `yaml:"summary" json:"summary"`
	Danger        *Danger  `yaml:"danger" json:"danger,omitempty"`
	// Persistent flags are taken by every subcommand too (kubectl's -n,
	// --context). A root flag that isn't persistent is only read before
	// the subcommand (git -C dir commit).
	Persistent bool `yaml:"persistent" json:"persistent,omitempty"`
	// Common flags are shown first in the builder; the rest are folded.
	Common bool `yaml:"common" json:"common,omitempty"`
}

// Choice is one value a flag or argument takes, with what it means.
type Choice struct {
	Value   string `yaml:"value" json:"value"`
	Summary string `yaml:"summary" json:"summary,omitempty"`
}

// UnmarshalYAML takes a choice written as a plain value too ("yaml").
func (c *Choice) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		c.Value = n.Value
		return nil
	}
	type plain Choice
	return n.Decode((*plain)(c))
}

// Arg is a positional argument.
type Arg struct {
	Name     string   `yaml:"name" json:"name"`
	Summary  string   `yaml:"summary" json:"summary"`
	Optional bool     `yaml:"optional" json:"optional,omitempty"`
	Repeat   bool     `yaml:"repeat" json:"repeat,omitempty"` // takes the rest
	Choices  []Choice `yaml:"choices" json:"choices,omitempty"`
}

// Danger says what a command or flag can break.
type Danger struct {
	// Level is "caution" (easy to undo, or only local) or "danger"
	// (history rewritten, data deleted, the cluster changed).
	Level string `yaml:"level" json:"level"`
	Note  string `yaml:"note" json:"note"`
	Safer string `yaml:"safer" json:"safer,omitempty"` // a safer way, as a command
}

//go:embed packs/*.yaml
var packFS embed.FS

var load = sync.OnceValues(func() (map[string]*Tool, error) { return readPacks(packFS) })

// readPacks reads and checks every pack.
func readPacks(fsys fs.FS) (map[string]*Tool, error) {
	files, err := fs.Glob(fsys, "packs/*.yaml")
	if err != nil {
		return nil, err
	}
	tools := map[string]*Tool{}
	for _, f := range files {
		data, err := fs.ReadFile(fsys, f)
		if err != nil {
			return nil, err
		}
		var t Tool
		if err := yaml.Unmarshal(data, &t); err != nil {
			return nil, fmt.Errorf("%s: %w", path.Base(f), err)
		}
		if err := check(&t.Command, t.Name); err != nil {
			return nil, fmt.Errorf("%s: %w", path.Base(f), err)
		}
		for _, n := range append([]string{t.Name}, t.Aliases...) {
			if _, dup := tools[n]; dup {
				return nil, fmt.Errorf("%s: %q is another tool's name too", path.Base(f), n)
			}
			tools[n] = &t
		}
	}
	return tools, nil
}

// check finds the mistakes a pack can make: a nameless command, a flag
// written without its dashes, a name used twice.
func check(c *Command, where string) error {
	if c.Name == "" || c.Summary == "" {
		return fmt.Errorf("%s: a command needs a name and a summary", where)
	}
	seen := map[string]bool{}
	for _, f := range c.Flags {
		if len(f.Names) == 0 || f.Summary == "" {
			return fmt.Errorf("%s: a flag needs names and a summary", where)
		}
		for _, n := range f.Names {
			if !strings.HasPrefix(n, "-") || n == "-" || n == "--" {
				return fmt.Errorf("%s: flag %q must start with - or --", where, n)
			}
			if seen[n] {
				return fmt.Errorf("%s: flag %s twice", where, n)
			}
			seen[n] = true
		}
		if f.OptionalValue && f.Value == "" {
			return fmt.Errorf("%s %s: an optional value needs a name", where, f.Names[0])
		}
		if f.Danger != nil && f.Danger.Level != "caution" && f.Danger.Level != "danger" {
			return fmt.Errorf("%s %s: danger level %q", where, f.Names[0], f.Danger.Level)
		}
	}
	for i, a := range c.Args {
		if a.Name == "" {
			return fmt.Errorf("%s: an argument needs a name", where)
		}
		if a.Repeat && i != len(c.Args)-1 {
			return fmt.Errorf("%s: only the last argument repeats", where)
		}
	}
	if c.Danger != nil && c.Danger.Level != "caution" && c.Danger.Level != "danger" {
		return fmt.Errorf("%s: danger level %q", where, c.Danger.Level)
	}
	names := map[string]bool{}
	for _, s := range c.Commands {
		for _, n := range append([]string{s.Name}, s.Aliases...) {
			if names[n] {
				return fmt.Errorf("%s: subcommand %q twice", where, n)
			}
			names[n] = true
		}
		if err := check(s, where+" "+s.Name); err != nil {
			return err
		}
	}
	return nil
}

// ToolInfo is a tool as a list shows it.
type ToolInfo struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Group   string `json:"group"`
}

// Tools lists the tools codec knows, by group then name.
func Tools() ([]ToolInfo, error) {
	tools, err := load()
	if err != nil {
		return nil, err
	}
	var out []ToolInfo
	seen := map[*Tool]bool{}
	for _, t := range tools {
		if !seen[t] {
			seen[t] = true
			out = append(out, ToolInfo{t.Name, t.Summary, t.Group})
		}
	}
	slices.SortFunc(out, func(a, b ToolInfo) int {
		if a.Group != b.Group {
			return strings.Compare(a.Group, b.Group)
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out, nil
}

// Lookup returns a tool's whole reference, for the builder; nil when
// codec doesn't know the tool.
func Lookup(name string) (*Tool, error) {
	tools, err := load()
	if err != nil {
		return nil, err
	}
	return tools[name], nil
}

// sub finds a subcommand of c by name or alias.
func (c *Command) sub(name string) *Command {
	for _, s := range c.Commands {
		if s.Name == name || slices.Contains(s.Aliases, name) {
			return s
		}
	}
	return nil
}
