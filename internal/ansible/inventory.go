package ansible

import (
	"regexp"
	"slices"
	"strconv"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Inventory is a YAML inventory: groups (all at the top) holding
// hosts, child groups and variables.
type Inventory struct {
	Groups   []Group   `json:"groups"` // in the order they first appear; "all" first
	Hosts    []Host    `json:"hosts"`
	Problems []Problem `json:"problems"`
}

// Group is one group; a group written in several places is merged.
type Group struct {
	Name     string   `json:"name"`
	Hosts    []string `json:"hosts"`
	Children []string `json:"children"`
	Vars     []KV     `json:"vars"`
	Source   *Source  `json:"source,omitempty"`
}

// Host is one host (or a range like web[01:03], counted).
type Host struct {
	Name   string   `json:"name"`
	Count  int      `json:"count"` // hosts the name stands for (ranges expand)
	Groups []string `json:"groups"`
	Vars   []KV     `json:"vars"`
	Source *Source  `json:"source,omitempty"`
}

// KV is a variable and its value.
type KV struct {
	Name   string  `json:"name"`
	Value  string  `json:"value"`
	Source *Source `json:"source,omitempty"`
}

// ReadInventory reads a YAML inventory.
func ReadInventory(file string, f *yamlkit.File) *Inventory {
	inv := &Inventory{}
	// Indexes, not pointers: appending may move the slices.
	groupAt := map[string]int{}
	hostAt := map[string]int{}
	var group func(name string, key, n *yamlkit.Node)
	group = func(name string, key, n *yamlkit.Node) {
		i, ok := groupAt[name]
		if !ok {
			i = len(inv.Groups)
			groupAt[name] = i
			inv.Groups = append(inv.Groups, Group{Name: name})
		}
		g := &inv.Groups[i]
		if g.Source == nil && key != nil {
			g.Source = ptr(src(file, key))
		}
		for _, pr := range pairs(n.Get("hosts")) {
			j, ok := hostAt[pr.Key.Value]
			if !ok {
				j = len(inv.Hosts)
				hostAt[pr.Key.Value] = j
				inv.Hosts = append(inv.Hosts, Host{Name: pr.Key.Value, Count: hostCount(pr.Key.Value), Source: ptr(src(file, pr.Key))})
			}
			h := &inv.Hosts[j]
			if !slices.Contains(h.Groups, name) {
				h.Groups = append(h.Groups, name)
			}
			h.Vars = append(h.Vars, kvs(file, pr.Value)...)
			if !slices.Contains(g.Hosts, pr.Key.Value) {
				g.Hosts = append(g.Hosts, pr.Key.Value)
			}
		}
		g.Vars = append(g.Vars, kvs(file, n.Get("vars"))...)
		for _, pr := range pairs(n.Get("children")) {
			// g may be stale after the recursive call below: index again.
			if g := &inv.Groups[i]; !slices.Contains(g.Children, pr.Key.Value) {
				g.Children = append(g.Children, pr.Key.Value)
			}
			group(pr.Key.Value, pr.Key, pr.Value)
		}
	}
	if len(f.Docs) > 0 {
		for _, pr := range pairs(f.Docs[0].Root) {
			group(pr.Key.Value, pr.Key, pr.Value)
		}
	}
	// Every group is a child of all, even when not written so.
	if i, ok := groupAt["all"]; ok {
		for _, g := range inv.Groups {
			if g.Name != "all" && !isChild(inv.Groups, g.Name) {
				inv.Groups[i].Children = append(inv.Groups[i].Children, g.Name)
			}
		}
	}
	for i := range inv.Groups {
		g := &inv.Groups[i]
		g.Hosts, g.Children, g.Vars = orEmpty(g.Hosts), orEmpty(g.Children), orEmpty(g.Vars)
	}
	for i := range inv.Hosts {
		inv.Hosts[i].Groups, inv.Hosts[i].Vars = orEmpty(inv.Hosts[i].Groups), orEmpty(inv.Hosts[i].Vars)
	}
	inv.Groups, inv.Hosts, inv.Problems = orEmpty(inv.Groups), orEmpty(inv.Hosts), orEmpty(inv.Problems)
	return inv
}

func isChild(groups []Group, name string) bool {
	return slices.ContainsFunc(groups, func(g Group) bool { return slices.Contains(g.Children, name) })
}

func kvs(file string, n *yamlkit.Node) []KV {
	var out []KV
	for _, pr := range pairs(n) {
		out = append(out, KV{Name: pr.Key.Value, Value: short1(flowText(pr.Value)), Source: ptr(src(file, pr.Key))})
	}
	return out
}

var hostRange = regexp.MustCompile(`\[([0-9]+|[a-z]):([0-9]+|[a-z])(?::([0-9]+))?\]`)

// hostCount is how many hosts a pattern like db-[a:c] or web[01:10:2]
// stands for.
func hostCount(name string) int {
	count := 1
	for _, m := range hostRange.FindAllStringSubmatch(name, -1) {
		lo, hi := rangeValue(m[1]), rangeValue(m[2])
		step := 1
		if m[3] != "" {
			step, _ = strconv.Atoi(m[3])
		}
		if hi >= lo && step > 0 {
			count *= (hi-lo)/step + 1
		}
	}
	return count
}

func rangeValue(s string) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return int(s[0])
}
