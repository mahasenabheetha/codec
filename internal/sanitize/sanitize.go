// Package sanitize takes what shouldn't leave the machine out of a text
// before it is shared (decision 104): secrets, emails, IP addresses,
// host names, UUIDs, Azure resource names, user names in paths, and
// words of your own. Each value becomes the same stand-in wherever it is
// (192.0.2.1, user-1@example.com, host-1.example.internal), so the text
// still reads; or everything is masked. Pure: no files, no network.
//
// It is a safety net, not a guarantee: read what you share.
package sanitize

import (
	"cmp"
	"fmt"
	"net/netip"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// The kinds of value, in the order a list shows them.
const (
	KindSecret = "secret"
	KindEmail  = "email"
	KindIP     = "ip"
	KindHost   = "host"
	KindUUID   = "uuid"
	KindAzure  = "azure"
	KindUser   = "user"
	KindWord   = "word"
)

// Options say what to take out and how.
type Options struct {
	Secrets bool `json:"secrets"`
	Emails  bool `json:"emails"`
	IPs     bool `json:"ips"`
	// KeepPrivate leaves private addresses (10.x, 192.168.x, fd00::…)
	// as they are: they say little outside the network.
	KeepPrivate bool `json:"keepPrivate"`
	Hosts       bool `json:"hosts"`
	UUIDs       bool `json:"uuids"`
	Azure       bool `json:"azure"`
	Users       bool `json:"users"`
	// Words are your own: a customer, a project, a server's name.
	Words []string `json:"words"`
	// Mask writes ••• for every value instead of a stand-in.
	Mask bool `json:"mask"`
	// Keep are values found that stay as they are (unticked in a list).
	Keep []string `json:"keep"`
}

// All is every kind on, stand-ins, nothing kept.
func All() Options {
	return Options{Secrets: true, Emails: true, IPs: true, Hosts: true, UUIDs: true, Azure: true, Users: true}
}

// Finding is one value taken out, with what it became.
type Finding struct {
	Kind        string `json:"kind"`
	Value       string `json:"value"`
	Replacement string `json:"replacement"`
	Count       int    `json:"count"`
	Kept        bool   `json:"kept"` // left as it is, by Options.Keep
}

// Result is the sanitized text and what was taken out of it.
type Result struct {
	Text     string    `json:"text"`
	Findings []Finding `json:"findings"`
}

// hit is a value found at [start, end).
type hit struct {
	start, end int
	kind       string
	prio       int // lower wins where two overlap
}

// Sanitize takes out of text what o asks for.
func Sanitize(text string, o Options) Result {
	var hits []hit
	add := func(kind string, prio int, start, end int) {
		if end > start {
			hits = append(hits, hit{start, end, kind, prio})
		}
	}
	// Your own words first: you asked for them.
	for _, w := range o.Words {
		if w = strings.TrimSpace(w); w == "" {
			continue
		}
		re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(w))
		for _, m := range re.FindAllStringIndex(text, -1) {
			if wordEdge(text, m[0], m[1]) {
				add(KindWord, 0, m[0], m[1])
			}
		}
	}
	if o.Secrets {
		secrets(text, func(s, e int) { add(KindSecret, 1, s, e) })
	}
	if o.Emails {
		emails(text, func(s, e int) {
			if !documentation(text[s:e]) {
				add(KindEmail, 2, s, e)
			}
		})
	}
	if o.Azure && strings.Contains(text, "/") {
		for _, re := range []*regexp.Regexp{azureGroup, azureResource} {
			for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
				add(KindAzure, 3, m[2], m[3])
			}
		}
	}
	if o.UUIDs {
		uuids(text, func(s, e int) {
			if v := text[s:e]; strings.Trim(v, "0-") != "" && !strings.HasPrefix(v, "00000000-0000-0000-0000-") {
				add(KindUUID, 4, s, e)
			}
		})
	}
	if o.IPs {
		ip := func(s, e int) {
			if ipToHide(text[s:e], o.KeepPrivate) {
				add(KindIP, 5, s, e)
			}
		}
		ipv4s(text, ip)
		ipv6s(text, ip)
	}
	if o.Hosts {
		hosts(text, func(s, e int) {
			if v := strings.ToLower(text[s:e]); !publicHost(v) && !documentation(v) {
				add(KindHost, 6, s, e)
			}
		})
	}
	if o.Users && (strings.Contains(text, "/home/") || strings.Contains(text, "/Users/") || strings.Contains(strings.ToLower(text), "users\\")) {
		for _, m := range userPathRe.FindAllStringSubmatchIndex(text, -1) {
			if !commonUser[strings.ToLower(text[m[2]:m[3]])] {
				add(KindUser, 7, m[2], m[3])
			}
		}
	}

	// The first to start wins; where two start together, the one that
	// matters more (a secret over a host), then the longer.
	slices.SortFunc(hits, func(a, b hit) int {
		return cmp.Or(cmp.Compare(a.start, b.start), cmp.Compare(a.prio, b.prio), cmp.Compare(b.end, a.end))
	})
	keep := map[string]bool{}
	for _, k := range o.Keep {
		keep[k] = true
	}
	st := newStandins()
	var b strings.Builder
	var findings []Finding
	index := map[[2]string]int{}
	at := 0
	for _, h := range hits {
		if h.start < at {
			continue // inside one already taken
		}
		v := text[h.start:h.end]
		key := [2]string{h.kind, v}
		if h.kind != KindSecret {
			key[1] = strings.ToLower(v) // ACME and acme are one word, one host
		}
		i, seen := index[key]
		if !seen {
			f := Finding{Kind: h.kind, Value: v, Kept: keep[v]}
			switch {
			case f.Kept:
				f.Replacement = v
			case o.Mask:
				f.Replacement = "•••"
			default:
				f.Replacement = st.next(h.kind, v)
			}
			i = len(findings)
			index[key] = i
			findings = append(findings, f)
		}
		findings[i].Count++
		b.WriteString(text[at:h.start])
		b.WriteString(findings[i].Replacement)
		at = h.end
	}
	b.WriteString(text[at:])
	if findings == nil {
		findings = []Finding{}
	}
	return Result{Text: b.String(), Findings: findings}
}

// Secrets counts the secrets in text, for a hint that it has some.
func Secrets(text string) int {
	n := 0
	secrets(text, func(int, int) { n++ })
	return n
}

// wordEdge: a word of your own counts where it isn't part of a longer
// word (acme, not acmeville), unless it starts or ends with a symbol.
func wordEdge(s string, start, end int) bool {
	isW := func(r byte) bool {
		return r == '_' || r < 0x80 && (unicode.IsLetter(rune(r)) || unicode.IsDigit(rune(r)))
	}
	if start > 0 && isW(s[start-1]) && isW(s[start]) {
		return false
	}
	return !(end < len(s) && isW(s[end]) && isW(s[end-1]))
}

// Stand-ins

type standins struct {
	n    map[string]int // per kind
	seen map[string]string
}

func newStandins() *standins { return &standins{n: map[string]int{}, seen: map[string]string{}} }

// next is v's stand-in: the same for the same value, numbered per kind.
func (s *standins) next(kind, v string) string {
	key := kind + "\x00" + strings.ToLower(v)
	if r, ok := s.seen[key]; ok {
		return r
	}
	s.n[kind]++
	n := s.n[kind]
	var r string
	switch kind {
	case KindSecret:
		r = fmt.Sprintf("[secret-%d]", n)
	case KindEmail:
		r = fmt.Sprintf("user-%d@example.com", n)
	case KindIP:
		if strings.Contains(v, ":") {
			r = fmt.Sprintf("2001:db8::%x", n)
		} else {
			r = testNet(n)
		}
	case KindHost:
		r = fmt.Sprintf("host-%d.example.internal", n)
	case KindUUID:
		r = fmt.Sprintf("00000000-0000-0000-0000-%012d", n)
	case KindAzure:
		r = fmt.Sprintf("name-%d", n)
	case KindUser:
		r = fmt.Sprintf("user-%d", n)
	default:
		r = fmt.Sprintf("word-%d", n)
	}
	s.seen[key] = r
	return r
}

// testNet is the n-th address of the ranges kept for documentation
// (RFC 5737): 192.0.2.x, then 198.51.100.x, then 203.0.113.x.
func testNet(n int) string {
	nets := []string{"192.0.2", "198.51.100", "203.0.113"}
	i := (n - 1) / 254
	return fmt.Sprintf("%s.%d", nets[i%len(nets)], (n-1)%254+1)
}

// Recognisers

var (
	// Azure resource ids: the group and the resource's own name.
	azureGroup    = regexp.MustCompile(`(?i)/resourceGroups/([^/\s"'?#]+)`)
	azureResource = regexp.MustCompile(`(?i)/providers/[A-Za-z0-9.]+/[A-Za-z0-9]+/([^/\s"'?#]+)`)
	// /home/NAME, /Users/NAME, C:\Users\NAME (also with \\ as JSON writes it).
	userPathRe = regexp.MustCompile(`(?i)(?:/home/|/Users/|[A-Za-z]:\\{1,2}Users\\{1,2})([^/\\\s"':;]+)`)
)

// commonUser are the folders under Users and home that aren't a person.
var commonUser = map[string]bool{"public": true, "default": true, "default user": true, "all users": true, "shared": true, "runner": true, "user": true, "$user": true, "%username%": true}

// publicHost: names that say nothing about you, left as they are.
var publicHosts = map[string]bool{
	"github.com": true, "gitlab.com": true, "bitbucket.org": true, "docker.io": true, "registry-1.docker.io": true, "ghcr.io": true, "quay.io": true,
	"k8s.io": true, "registry.k8s.io": true, "kubernetes.io": true, "helm.sh": true, "npmjs.org": true, "registry.npmjs.org": true, "pypi.org": true,
	"golang.org": true, "go.dev": true, "proxy.golang.org": true, "google.com": true, "microsoft.com": true, "azure.com": true, "mcr.microsoft.com": true,
	"amazonaws.com": true, "cloudflare.com": true, "ubuntu.com": true, "debian.org": true, "alpinelinux.org": true, "githubusercontent.com": true,
	"raw.githubusercontent.com": true, "objects.githubusercontent.com": true, "w3.org": true, "schema.org": true, "json-schema.org": true,
	"cluster.local": true, "svc.cluster.local": true,
	// Azure's own suffixes: an account or vault name before them is yours.
	"core.windows.net": true, "blob.core.windows.net": true, "database.windows.net": true, "servicebus.windows.net": true,
	"azurewebsites.net": true, "vault.azure.net": true, "azurecr.io": true, "azmk8s.io": true, "login.microsoftonline.com": true,
}

func publicHost(h string) bool { return publicHosts[h] }

// documentation: the names and addresses kept for examples (RFC 2606,
// 5737, 3849), which a stand-in itself is.
func documentation(v string) bool {
	v = strings.ToLower(v)
	for _, s := range []string{"example.com", "example.org", "example.net", "example.internal", ".example", ".invalid", ".test", ".localhost"} {
		if strings.HasSuffix(v, s) {
			return true
		}
	}
	return false
}

// ipToHide says whether an address is one to take out: a real address,
// not loopback, unspecified, broadcast or a documentation range, and not
// private when private ones are kept.
func ipToHide(s string, keepPrivate bool) bool {
	a, err := netip.ParseAddr(s)
	if err != nil {
		return false
	}
	if a.Is4() && s != a.String() {
		return false // a leading zero: 01.02.03.04 is a version, not an address
	}
	if a.IsLoopback() || a.IsUnspecified() || a == netip.MustParseAddr("255.255.255.255") {
		return false
	}
	for _, p := range docNets {
		if p.Contains(a) {
			return false
		}
	}
	if a.Is6() && !strings.Contains(s, "::") && strings.Count(s, ":") < 7 {
		return false
	}
	return !(keepPrivate && (a.IsPrivate() || a.IsLinkLocalUnicast()))
}

var docNets = []netip.Prefix{
	netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("2001:db8::/32"),
}
