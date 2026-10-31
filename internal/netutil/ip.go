package netutil

import (
	"fmt"
	"math/big"
	"net/netip"
	"strings"
)

// IP is an address in every notation the Network tool shows.
type IP struct {
	Address  string `json:"address"` // canonical (IPv6 shortened)
	Version  int    `json:"version"`
	Expanded string `json:"expanded,omitempty"` // IPv6, every group in full
	Integer  string `json:"integer"`            // decimal
	Hex      string `json:"hex"`
	Binary   string `json:"binary"` // by octet (IPv4) or 16-bit group (IPv6)
	// Mapped is an IPv4 address as IPv4-mapped IPv6 (::ffff:10.0.0.1),
	// or the IPv4 address inside a mapped one.
	Mapped  string `json:"mapped,omitempty"`
	Reverse string `json:"reverse"` // the PTR name for reverse DNS
	Kind    string `json:"kind"`
}

// ParseIP reads an address as text (10.0.0.1, 2001:db8::1, [::1]), as a
// decimal integer, or as hex (0x0a000001) or binary (0b…) digits. A
// number up to 2^32-1 is IPv4, a larger one IPv6.
func ParseIP(s string) (IP, error) {
	a, err := readAddr(s)
	if err != nil {
		return IP{}, err
	}
	return describeIP(a), nil
}

func readAddr(s string) (netip.Addr, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return netip.Addr{}, fmt.Errorf("enter an address such as 10.0.0.1")
	}
	if a, err := netip.ParseAddr(strings.TrimSuffix(strings.TrimPrefix(s, "["), "]")); err == nil {
		return a, nil
	}
	// Hex and binary say the family by how many digits are written
	// (0x00000000000000000000000000000001 is ::1); a decimal number by
	// its size.
	n, ok, v6 := new(big.Int), false, false
	switch low := strings.ToLower(strings.ReplaceAll(s, "_", "")); {
	case strings.HasPrefix(low, "0x"):
		digits := low[2:]
		_, ok = n.SetString(digits, 16)
		ok = ok && len(digits) <= 32
		v6 = len(digits) > 8
	case strings.HasPrefix(low, "0b"):
		digits := low[2:]
		_, ok = n.SetString(digits, 2)
		ok = ok && len(digits) <= 128
		v6 = len(digits) > 32
	default:
		_, ok = n.SetString(low, 10)
		v6 = ok && n.BitLen() > 32
	}
	if !ok || n.Sign() < 0 || n.BitLen() > 128 {
		return netip.Addr{}, fmt.Errorf("%q is not an IP address or an address number", s)
	}
	return fromInt(n, v6), nil
}

func describeIP(a netip.Addr) IP {
	a = a.WithZone("")
	b := a.AsSlice()
	n := toInt(a)
	ip := IP{
		Address: a.String(),
		Version: 4,
		Integer: n.String(),
		Hex:     fmt.Sprintf("0x%0*x", len(b)*2, n),
		Kind:    kindOf(netip.PrefixFrom(a, a.BitLen())),
	}
	var groups, rev []string
	if a.Is4() {
		for _, o := range b {
			groups = append(groups, fmt.Sprintf("%08b", o))
		}
		for i := len(b) - 1; i >= 0; i-- {
			rev = append(rev, fmt.Sprint(b[i]))
		}
		ip.Binary = strings.Join(groups, ".")
		ip.Mapped = netip.AddrFrom16(a.As16()).String()
		ip.Reverse = strings.Join(rev, ".") + ".in-addr.arpa"
		return ip
	}
	ip.Version = 6
	ip.Expanded = a.StringExpanded()
	for i := 0; i < 16; i += 2 {
		groups = append(groups, fmt.Sprintf("%08b%08b", b[i], b[i+1]))
	}
	h := fmt.Sprintf("%032x", n)
	for i := len(h) - 1; i >= 0; i-- {
		rev = append(rev, h[i:i+1])
	}
	ip.Binary = strings.Join(groups, ":")
	ip.Reverse = strings.Join(rev, ".") + ".ip6.arpa"
	if a.Is4In6() {
		ip.Mapped = a.Unmap().String()
	}
	return ip
}
