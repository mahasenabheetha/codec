package logsrc

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestPhrases(t *testing.T) {
	tests := []struct {
		name, message string
		want          []string
	}{
		{"numbers are slots", "order 7 failed after 3 retries", []string{"failed after"}},
		{"ids and quoted values", `user "bob" not found in tenant 4f9c2a1b`, []string{"not found in tenant", "found in tenant", "not found in"}},
		{"a word that is a variable", "user bob logged in from 10.0.0.1", []string{"user bob logged in from", "bob logged in from", "user bob logged in"}},
		{"two words left are too common", "bob failed for 10.0.0.1", []string{"bob failed for"}},
		{"key=value keeps the key", "retrying request attempt=3 backoff=2s", []string{"retrying request attempt"}},
		{"paths and urls", "cannot read /etc/app/config.yaml from https://x.io/a", []string{"cannot read", "from"}[:1]},
		{"too short or common", "failed: 42", nil},
		{"one long word", "Unauthorized", []string{"Unauthorized"}},
		{"duplicates once", "cache miss, cache miss", []string{"cache miss, cache miss", "cache miss, cache", "miss, cache miss"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Phrases(tc.message); !slices.Equal(got, tc.want) {
				t.Errorf("Phrases(%q)\n got %q\nwant %q", tc.message, got, tc.want)
			}
		})
	}
}

func TestLayers(t *testing.T) {
	tests := []struct {
		name string
		q    LineQuery
		want []string
	}{
		{"plain message", LineQuery{Message: "payment declined by provider"}, []string{"payment declined by provider"}},
		{"go error chain", LineQuery{Message: "checkout failed", Error: "add item to cart: reserve stock: cart is full"},
			[]string{"checkout failed", "add item to cart", "reserve stock", "cart is full"}},
		{"a label is not a layer", LineQuery{Message: "ValueError: invalid literal for int"}, []string{"ValueError: invalid literal for int"}},
		{"nothing to search", LineQuery{Message: "42"}, nil},
		{"error repeated in the message", LineQuery{Message: "cart is full", Error: "cart is full"}, []string{"cart is full"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Layers(tc.q); !slices.Equal(got, tc.want) {
				t.Errorf("Layers(%+v)\n got %q\nwant %q", tc.q, got, tc.want)
			}
		})
	}
}

func TestFindLine(t *testing.T) {
	src := map[string]string{
		"internal/shop/cart.go": `package shop

func (c *Cart) Add(id int) error {
	if c.full() {
		return fmt.Errorf("cart is full")
	}
	slog.Info("order failed after retries", "order_id", id, "attempts", n)
	return nil
}
`,
		"internal/shop/cart_test.go": "\tt.Errorf(\"order failed after retries: got %d\", n)\n",
		"docs/errors.md":             "If the order failed after retries, check the provider.\n",
		"web/app.ts":                 "console.error(`reserve stock for ${sku}`)\n",
		"run.log":                    "order 7 failed after 3 retries\n",
		"node_modules/x/i.js":        "log('order failed after retries')\n",
	}
	files := make([]string, 0, len(src))
	for p := range src {
		files = append(files, p)
	}
	read := func(p string) ([]byte, error) {
		if s, ok := src[p]; ok {
			return []byte(s), nil
		}
		return nil, errors.New("no file")
	}
	show := func(l Layer) string {
		var b []string
		for _, h := range l.Hits {
			b = append(b, h.Path+":"+strconv.Itoa(h.Line)+" "+string(h.Quality))
		}
		return strings.Join(b, ", ")
	}

	layers, _, err := FindLine(context.Background(), LineQuery{Message: "order failed after retries", Fields: []string{"order_id", "level"}}, files, read)
	if err != nil || len(layers) != 1 {
		t.Fatalf("FindLine: %v %v", layers, err)
	}
	// The logging call first, then the test (a strong look-alike), then
	// the docs; logs and dependencies never.
	if got, want := show(layers[0]), "internal/shop/cart.go:7 exact, internal/shop/cart_test.go:1 strong, docs/errors.md:1 possible"; got != want {
		t.Errorf("hits\n got %s\nwant %s", got, want)
	}
	if why := layers[0].Hits[0].Why; !slices.Contains(why, "in a string") || !slices.Contains(why, "next to a logging call") || !slices.Contains(why, "the entry's field names nearby") {
		t.Errorf("why: %q", why)
	}

	// A wrapped error: each layer found on its own.
	layers, _, _ = FindLine(context.Background(), LineQuery{Message: "checkout failed", Error: "reserve stock: cart is full"}, files, read)
	got := []string{}
	for _, l := range layers {
		got = append(got, l.Text+" → "+show(l))
	}
	want := []string{"checkout failed → ", "reserve stock → web/app.ts:1 exact", "cart is full → internal/shop/cart.go:5 exact"}
	if !slices.Equal(got, want) {
		t.Errorf("chain\n got %q\nwant %q", got, want)
	}
}

// A lower-cased line can be longer than the line: the phrase is found in
// the line itself (regression: an index out of range in inString).
func TestFindLineLongerLower(t *testing.T) {
	src := map[string]string{
		"a.go": strings.Repeat("\xe9", 60) + ` log.Info("order failed for customer")` + "\n",
		"b.go": strings.Repeat("Ⱥ", 80) + ` log.Info("order failed for customer")` + "\n",
	}
	read := func(p string) ([]byte, error) { return []byte(src[p]), nil }
	layers, _, err := FindLine(context.Background(), LineQuery{Message: "order failed for customer"}, []string{"a.go", "b.go"}, read)
	if err != nil || len(layers) != 1 || len(layers[0].Hits) != 2 || !slices.Contains(layers[0].Hits[0].Why, "in a string") {
		t.Fatalf("FindLine: %+v %v", layers, err)
	}
}

// A search stopped by its deadline keeps what it found.
func TestFindLineCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	layers, _, err := FindLine(ctx, LineQuery{Message: "cart is full"}, []string{"a.go"}, func(string) ([]byte, error) { return []byte(`"cart is full"`), nil })
	if !errors.Is(err, context.Canceled) || len(layers) != 1 {
		t.Errorf("FindLine: %v %v", layers, err)
	}
}

func TestInString(t *testing.T) {
	line := `log.Info("cart is full", "id", 3) // cart is full`
	if !inString(line, strings.Index(line, "cart")) || inString(line, strings.LastIndex(line, "cart")) {
		t.Error("inString")
	}
}

func FuzzPhrases(f *testing.F) {
	for _, s := range []string{"order 7 failed", `user "bob" x`, "a: b: c", "k=v k2=", "''''", "x://"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, msg string) {
		for _, p := range Phrases(msg) {
			if p == "" || strings.Trim(p, edges) != p {
				t.Fatalf("bad phrase %q of %q", p, msg)
			}
		}
		Layers(LineQuery{Message: msg, Error: msg})
	})
}
