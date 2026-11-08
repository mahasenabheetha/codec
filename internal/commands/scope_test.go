package commands

import "testing"

// Every Linux tool has a category, one of those the lists show.
func TestLinuxCategories(t *testing.T) {
	tools, err := Tools()
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{"files": true, "text": true, "archives": true, "permissions": true, "system": true, "network": true}
	for _, ti := range tools {
		if ti.Group == "linux" && !known[ti.Category] {
			t.Errorf("%s: category %q", ti.Name, ti.Category)
		}
		if ti.Group != "linux" && ti.Category != "" {
			t.Errorf("%s: a category outside linux", ti.Name)
		}
	}
}

// A search keeps to a tool, a group or a category.
func TestSearchScope(t *testing.T) {
	tools, _ := Tools()
	info := map[string]ToolInfo{}
	for _, ti := range tools {
		info[ti.Name] = ti
	}
	for _, c := range []struct {
		query, filter string
		ok            func(ToolInfo) bool
	}{
		{"", "group:linux", func(ti ToolInfo) bool { return ti.Group == "linux" }},
		{"", "linux", func(ti ToolInfo) bool { return ti.Group == "linux" }}, // no tool named linux
		{"", "category:network", func(ti ToolInfo) bool { return ti.Category == "network" }},
		{"copy files to a server", "category:network", func(ti ToolInfo) bool { return ti.Category == "network" }},
		{"", "git", func(ti ToolInfo) bool { return ti.Name == "git" }}, // the tool, though git is a group too
	} {
		got, err := Search(c.query, c.filter, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) == 0 {
			t.Errorf("%q in %s: nothing", c.query, c.filter)
		}
		for _, r := range got {
			if !c.ok(info[r.Tool]) {
				t.Errorf("%q in %s: %s (%s)", c.query, c.filter, r.Tool, r.Title)
			}
		}
	}
}
