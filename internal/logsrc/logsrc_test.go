package logsrc

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// show writes refs as "text→path:line:col" for compact comparison.
func show(text string, refs []Ref) string {
	var b []string
	for _, r := range refs {
		b = append(b, fmt.Sprintf("%s→%s:%d:%d", text[r.Start:r.End], r.Path, r.Line, r.Col))
	}
	return strings.Join(b, " | ")
}

func TestFind(t *testing.T) {
	tests := []struct {
		name, text, want string
	}{
		{"go panic frame", "\t/build/src/internal/shop/cart.go:88 +0x1d4", "/build/src/internal/shop/cart.go:88→/build/src/internal/shop/cart.go:88:0"},
		{"go vet with column", "internal/x.go:12:5: unreachable code", "internal/x.go:12:5→internal/x.go:12:5"},
		{"go test", "    cart_test.go:41: got 3, want 4", "cart_test.go:41→cart_test.go:41:0"},
		{"java frame gets its package", "\tat com.acme.shop.Cart$Item.add(Cart.java:42)", "Cart.java:42→com/acme/shop/Cart.java:42:0"},
		{"java constructor", "at com.acme.Shop.<init>(Shop.java:7)", "Shop.java:7→com/acme/Shop.java:7:0"},
		{"java without a package", "at Main.main(Main.java:3)", "Main.java:3→Main.java:3:0"},
		{"java native method is no file", "at java.lang.Thread.run(Native Method)", ""},
		{"kotlin", "at io.app.MainKt.main(Main.kt:10)", "Main.kt:10→io/app/Main.kt:10:0"},
		{"python", `  File "/app/shop/cart.py", line 12, in add`, `File "/app/shop/cart.py", line 12→/app/shop/cart.py:12:0`},
		{".NET stack", `   at Shop.Cart.Add() in C:\src\Shop\Cart.cs:line 42`, `C:\src\Shop\Cart.cs:line 42→C:/src/Shop/Cart.cs:42:0`},
		{"msbuild", `Cart.cs(42,7): error CS0103: The name 'x' does not exist`, "Cart.cs(42,7)→Cart.cs:42:7"},
		{"tsc", "src/app.ts(10,5): error TS2322: Type 'string'", "src/app.ts(10,5)→src/app.ts:10:5"},
		{"tsc pretty", "src/app.ts:10:5 - error TS2322", "src/app.ts:10:5→src/app.ts:10:5"},
		{"node frame", "    at add (/app/src/cart.js:10:5)", "/app/src/cart.js:10:5→/app/src/cart.js:10:5"},
		{"node file url", "at file:///app/src/cart.mjs:3:9", "///app/src/cart.mjs:3:9→///app/src/cart.mjs:3:9"},
		{"node after a port", "at http://localhost:3000/static/js/main.js:10:5", "/static/js/main.js:10:5→/static/js/main.js:10:5"},
		{"terraform", "Error: Unsupported argument\n\n  on main.tf line 12, in resource \"x\" \"y\":", "main.tf line 12→main.tf:12:0"},
		{"terraform module", "  on modules/net/vpc.tf line 3:", "modules/net/vpc.tf line 3→modules/net/vpc.tf:3:0"},
		{"helm", "Error: template: shop/templates/deploy.yaml:23:14: executing", "shop/templates/deploy.yaml:23:14→shop/templates/deploy.yaml:23:14"},
		{"ansible task path", "task path: /home/u/repo/roles/app/tasks/main.yml:12", "/home/u/repo/roles/app/tasks/main.yml:12→/home/u/repo/roles/app/tasks/main.yml:12:0"},
		{"maven", "[ERROR] /ws/shop/src/main/java/com/acme/Cart.java:[42,7] cannot find symbol", "/ws/shop/src/main/java/com/acme/Cart.java:[42,7]→/ws/shop/src/main/java/com/acme/Cart.java:42:7"},
		{"rust", "  --> src/main.rs:10:5", "src/main.rs:10:5→src/main.rs:10:5"},
		{"dockerfile", "Dockerfile:12\n--------------------", "Dockerfile:12→Dockerfile:12:0"},
		{"windows path", `C:\agent\_work\1\s\cmd\main.go:7`, `C:\agent\_work\1\s\cmd\main.go:7→C:/agent/_work/1/s/cmd/main.go:7:0`},
		{"relative ./", "./cmd/main.go:7", "./cmd/main.go:7→cmd/main.go:7:0"},
		{"eslint stylish", "/app/src/cart.js\n  10:5  error  'x' is not defined  no-undef\n  12:1  warning  Unexpected console  no-console\n", "10:5→/app/src/cart.js:10:5 | 12:1→/app/src/cart.js:12:1"},
		{"eslint unix", "/app/src/cart.js:10:5: 'x' is not defined [Error/no-undef]", "/app/src/cart.js:10:5→/app/src/cart.js:10:5"},
		{"two in one line", "a.go:1 then b.go:2", "a.go:1→a.go:1:0 | b.go:2→b.go:2:0"},

		// Not files.
		{"time", "12:30:45 started", ""},
		{"host and port", "dial tcp api.example.com:443: i/o timeout", ""},
		{"localhost", "listening on localhost:8080", ""},
		{"ip", "connect 10.0.0.1:5432 refused", ""},
		{"version", "image v1.2.3:45", ""},
		{"unknown extension", "report.xyz:12", ""},
		{"line zero", "x.go:0", ""},
		{"go call is no msbuild", "fmt.Println(3)", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := show(tc.text, Find(tc.text)); got != tc.want {
				t.Errorf("Find(%q)\n got %s\nwant %s", tc.text, got, tc.want)
			}
		})
	}
}

func TestFindBounded(t *testing.T) {
	text := strings.Repeat("a.go:1 ", MaxRefs+50)
	if n := len(Find(text)); n != MaxRefs {
		t.Errorf("got %d refs, want %d", n, MaxRefs)
	}
}

func TestResolve(t *testing.T) {
	files := []string{
		"cmd/codec/main.go",
		"internal/shop/cart.go",
		"internal/shop/cart_test.go",
		"testdata/fixtures/internal/shop/cart.go",
		"src/main/java/com/acme/shop/Cart.java",
		"Dockerfile",
		"build/Dockerfile",
		"pkg/panic.go",
		"src/App.ts",
	}
	ix := NewIndex(files)
	tests := []struct {
		name, path, want string
	}{
		{"relative path", "cmd/codec/main.go", "cmd/codec/main.go exact"},
		{"build path above the repository", "/build/src/cmd/codec/main.go", "cmd/codec/main.go exact"},
		{"windows agent path", "C:/agent/_work/1/s/cmd/codec/main.go", "cmd/codec/main.go exact"},
		{"a copy deeper down is listed after", "internal/shop/cart.go", "internal/shop/cart.go strong, testdata/fixtures/internal/shop/cart.go strong"},
		{"the longer tail wins", "fixtures/internal/shop/cart.go", "testdata/fixtures/internal/shop/cart.go exact"},
		{"java package path", "com/acme/shop/Cart.java", "src/main/java/com/acme/shop/Cart.java exact"},
		{"bare name shared by two", "Dockerfile", "Dockerfile strong, build/Dockerfile strong"},
		{"only the name agrees", "/usr/local/go/src/runtime/panic.go", "pkg/panic.go possible"},
		{"case is ignored", "SRC/app.ts", "src/App.ts exact"},
		{"not in the folder", "x/missing.go", ""},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, m := range ix.Resolve(tc.path) {
				got = append(got, m.File+" "+string(m.Quality))
			}
			if s := strings.Join(got, ", "); s != tc.want {
				t.Errorf("Resolve(%q) = %s, want %s", tc.path, s, tc.want)
			}
		})
	}
}

func FuzzFind(f *testing.F) {
	for _, s := range []string{
		"\tat com.acme.Cart.add(Cart.java:42)", `File "x.py", line 3`, `in C:\x\Y.cs:line 4`, "X.cs(1,2): error",
		"on main.tf line 1", "a.go:1:2", "/x.js\n  1:2  error  no\n", "http://h:1/x.js:1:2", "::::", "(((1,2)",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, text string) {
		last := -1
		for _, r := range Find(text) {
			if r.Start < 0 || r.End > len(text) || r.Start >= r.End || r.Start < last {
				t.Fatalf("bad span %d–%d in %q", r.Start, r.End, text)
			}
			if r.Line <= 0 || r.Path == "" {
				t.Fatalf("bad ref %+v in %q", r, text)
			}
			last = r.End
		}
		if utf8.ValidString(text) {
			NewIndex([]string{text, "a/" + text}).Resolve(text)
		}
	})
}
