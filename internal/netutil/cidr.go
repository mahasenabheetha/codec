package netutil

import (
	"errors"
	"fmt"
	"math/big"
	"net/netip"
	"strings"
)

// Range is a CIDR range in detail. Counts are decimal strings: an IPv6
// range can hold more addresses than any integer type.
type Range struct {
	CIDR      string `json:"cidr"` // normalised: the network address and length
	Version   int    `json:"version"`
	Prefix    int    `json:"prefix"`
	Network   string `json:"network"`
	Broadcast string `json:"broadcast,omitempty"` // IPv4 /30 and wider only
	First     string `json:"first"`               // first usable host
	Last      string `json:"last"`                // last usable host
	Mask      string `json:"mask"`
	Wildcard  string `json:"wildcard"`
	Addresses string `json:"addresses"`
	Hosts     string `json:"hosts"` // usable host addresses
	Kind      string `json:"kind"`  // private, public, documentation, …
	// Host is the address given when it isn't the network address
	// ("10.0.0.5/24" names a host in 10.0.0.0/24).
	Host string `json:"host,omitempty"`
}

// ParsePrefix reads a range: "10.0.0.0/24", "2001:db8::/48", an address
// with a netmask ("10.0.0.0 255.255.255.0"), or a single address (/32,
// /128). Host bits are allowed; the range is the network they lie in.
func ParsePrefix(s string) (netip.Prefix, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return netip.Prefix{}, errors.New("enter a range such as 10.0.0.0/16")
	}
	if addr, mask, ok := strings.Cut(s, " "); ok {
		a, err := netip.ParseAddr(addr)
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("%q is not an IP address", addr)
		}
		bits, err := maskBits(strings.TrimSpace(mask))
		if err != nil {
			return netip.Prefix{}, err
		}
		if !a.Is4() {
			return netip.Prefix{}, errors.New("a netmask goes with an IPv4 address; write IPv6 ranges as address/length")
		}
		return netip.PrefixFrom(a, bits), nil
	}
	if strings.Contains(s, "/") {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("%q is not a CIDR range (address/length, e.g. 10.0.0.0/16)", s)
		}
		return p, nil
	}
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("%q is not a CIDR range or an IP address", s)
	}
	return netip.PrefixFrom(a.WithZone(""), a.BitLen()), nil
}

// maskBits is the prefix length of a dotted netmask; the ones must be
// contiguous (255.255.0.255 is not a mask).
func maskBits(mask string) (int, error) {
	m, err := netip.ParseAddr(mask)
	if err != nil || !m.Is4() {
		return 0, fmt.Errorf("%q is not a netmask such as 255.255.255.0", mask)
	}
	n := toInt(m).Uint64()
	bits := 0
	for bits < 32 && n&(1<<(31-bits)) != 0 {
		bits++
	}
	if n != (1<<32-1)^(1<<(32-bits)-1) {
		return 0, fmt.Errorf("%q is not a netmask: its ones aren't contiguous", mask)
	}
	return bits, nil
}

// Details describes the range s.
func Details(s string) (Range, error) {
	p, err := ParsePrefix(s)
	if err != nil {
		return Range{}, err
	}
	return describe(p), nil
}

func describe(p netip.Prefix) Range {
	m := p.Masked()
	total := p.Addr().BitLen()
	hostBits := total - p.Bits()
	first, last := m.Addr(), lastAddr(p)
	r := Range{
		CIDR:      m.String(),
		Version:   4,
		Prefix:    p.Bits(),
		Network:   first.String(),
		First:     first.String(),
		Last:      last.String(),
		Mask:      maskAddr(p.Bits(), total, false).String(),
		Wildcard:  maskAddr(p.Bits(), total, true).String(),
		Addresses: pow2(hostBits).String(),
		Kind:      kindOf(p),
	}
	if p.Addr() != first {
		r.Host = p.Addr().String()
	}
	r.Hosts = r.Addresses
	if p.Addr().Is6() {
		r.Version = 6
		return r
	}
	// IPv4: the network and broadcast addresses aren't hosts, except
	// in a /31 (point-to-point, RFC 3021) and a /32 (one host).
	if hostBits >= 2 {
		r.Broadcast = last.String()
		r.First = first.Next().String()
		r.Last = last.Prev().String()
		r.Hosts = new(big.Int).Sub(pow2(hostBits), big.NewInt(2)).String()
	}
	return r
}

// Contains reports whether the range holds the address.
func Contains(rng, addr string) (bool, error) {
	p, err := ParsePrefix(rng)
	if err != nil {
		return false, err
	}
	a, err := netip.ParseAddr(strings.TrimSpace(addr))
	if err != nil {
		return false, fmt.Errorf("%q is not an IP address", addr)
	}
	return p.Masked().Contains(a.WithZone("")), nil
}

// MaxSubnets bounds how many subnets Split lists.
const MaxSubnets = 1024

// Subnets is a range split into equal parts.
type Subnets struct {
	Count     string   `json:"count"` // how many there are in all
	Subnets   []string `json:"subnets"`
	Truncated bool     `json:"truncated,omitempty"` // only the first MaxSubnets listed
	Each      Range    `json:"each"`                // the first subnet in detail
}

// Split divides the range into subnets of length bits.
func Split(rng string, bits int) (Subnets, error) {
	p, err := ParsePrefix(rng)
	if err != nil {
		return Subnets{}, err
	}
	p = p.Masked()
	total := p.Addr().BitLen()
	if bits < p.Bits() || bits > total {
		return Subnets{}, fmt.Errorf("split /%d into /%d to /%d", p.Bits(), p.Bits(), total)
	}
	count := pow2(bits - p.Bits())
	size := pow2(total - bits)
	n := toInt(p.Addr())
	out := Subnets{Count: count.String()}
	limit := int64(MaxSubnets)
	if count.IsInt64() && count.Int64() < limit {
		limit = count.Int64()
	} else {
		out.Truncated = count.Cmp(big.NewInt(MaxSubnets)) > 0
	}
	for range limit {
		out.Subnets = append(out.Subnets, netip.PrefixFrom(fromInt(n, p.Addr().Is6()), bits).String())
		n.Add(n, size)
	}
	out.Each = describe(netip.PrefixFrom(p.Addr(), bits))
	return out, nil
}

// MaxRanges bounds the list Overlaps compares.
const MaxRanges = 1000

// Listed is one line of a list of ranges.
type Listed struct {
	Input string `json:"input"`
	CIDR  string `json:"cidr,omitempty"`
	Error string `json:"error,omitempty"`
}

// Overlap is two ranges of the list (by index) that share addresses.
// CIDR ranges either nest or don't touch: Relation is "same" or
// "contains" (A holds all of B).
type Overlap struct {
	A        int    `json:"a"`
	B        int    `json:"b"`
	Relation string `json:"relation"`
}

// Overlaps checks a list of ranges (VNets, VPCs, pod and service CIDRs)
// for pairs that share addresses. Blank lines and # comments are
// skipped; a line that isn't a range is reported, not fatal.
func Overlaps(lines []string) ([]Listed, []Overlap, error) {
	var list []Listed
	var prefixes []netip.Prefix
	for _, l := range lines {
		l, _, _ = strings.Cut(l, "#")
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if len(list) == MaxRanges {
			return nil, nil, fmt.Errorf("compare at most %d ranges", MaxRanges)
		}
		p, err := ParsePrefix(l)
		item := Listed{Input: l}
		if err != nil {
			item.Error = err.Error()
		} else {
			p = p.Masked()
			item.CIDR = p.String()
		}
		list = append(list, item)
		prefixes = append(prefixes, p)
	}
	overlaps := []Overlap{}
	for i := range prefixes {
		for j := i + 1; j < len(prefixes); j++ {
			a, b := prefixes[i], prefixes[j]
			if !a.IsValid() || !b.IsValid() || !a.Overlaps(b) {
				continue
			}
			switch {
			case a == b:
				overlaps = append(overlaps, Overlap{i, j, "same"})
			case a.Bits() < b.Bits():
				overlaps = append(overlaps, Overlap{i, j, "contains"})
			default:
				overlaps = append(overlaps, Overlap{j, i, "contains"})
			}
		}
	}
	return list, overlaps, nil
}
