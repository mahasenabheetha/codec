// Package netutil is the engine of the Network tool: CIDR ranges (their
// details, subnets and overlaps) and IP addresses in other notations,
// IPv4 and IPv6 alike. It only computes; nothing is looked up on the
// network and nothing is stored.
package netutil

import (
	"math/big"
	"net/netip"
)

// toInt is an address as a number.
func toInt(a netip.Addr) *big.Int {
	return new(big.Int).SetBytes(a.AsSlice())
}

// fromInt is the address with value n in a's family. n must fit.
func fromInt(n *big.Int, v6 bool) netip.Addr {
	if v6 {
		var b [16]byte
		n.FillBytes(b[:])
		return netip.AddrFrom16(b)
	}
	var b [4]byte
	n.FillBytes(b[:])
	return netip.AddrFrom4(b)
}

// pow2 is 2^n.
func pow2(n int) *big.Int {
	return new(big.Int).Lsh(big.NewInt(1), uint(n))
}

// lastAddr is the highest address of p (all host bits set).
func lastAddr(p netip.Prefix) netip.Addr {
	n := toInt(p.Masked().Addr())
	n.Add(n, pow2(p.Addr().BitLen()-p.Bits()))
	n.Sub(n, big.NewInt(1))
	return fromInt(n, p.Addr().Is6())
}

// maskAddr is the netmask of a prefix length as an address
// (255.255.255.0, ffff:ffff:ffff:ffff::); wildcard flips it.
func maskAddr(bits, total int, wildcard bool) netip.Addr {
	n := new(big.Int).Sub(pow2(total), pow2(total-bits)) // ones then zeros
	if wildcard {
		n.Sub(pow2(total-bits), big.NewInt(1))
	}
	return fromInt(n, total == 128)
}

// special address blocks and what they are for (IANA special-purpose
// registries). Ranges inside none of them are public.
var special = []struct {
	prefix netip.Prefix
	kind   string
}{
	{netip.MustParsePrefix("0.0.0.0/8"), "this network"},
	{netip.MustParsePrefix("10.0.0.0/8"), "private"},
	{netip.MustParsePrefix("100.64.0.0/10"), "shared (carrier-grade NAT)"},
	{netip.MustParsePrefix("127.0.0.0/8"), "loopback"},
	{netip.MustParsePrefix("169.254.0.0/16"), "link-local"},
	{netip.MustParsePrefix("172.16.0.0/12"), "private"},
	{netip.MustParsePrefix("192.0.2.0/24"), "documentation"},
	{netip.MustParsePrefix("192.168.0.0/16"), "private"},
	{netip.MustParsePrefix("198.18.0.0/15"), "benchmarking"},
	{netip.MustParsePrefix("198.51.100.0/24"), "documentation"},
	{netip.MustParsePrefix("203.0.113.0/24"), "documentation"},
	{netip.MustParsePrefix("224.0.0.0/4"), "multicast"},
	{netip.MustParsePrefix("255.255.255.255/32"), "broadcast"},
	{netip.MustParsePrefix("240.0.0.0/4"), "reserved"},
	{netip.MustParsePrefix("::/128"), "unspecified"},
	{netip.MustParsePrefix("::1/128"), "loopback"},
	{netip.MustParsePrefix("::ffff:0:0/96"), "IPv4-mapped"},
	{netip.MustParsePrefix("64:ff9b::/96"), "NAT64"},
	{netip.MustParsePrefix("2001:db8::/32"), "documentation"},
	{netip.MustParsePrefix("fc00::/7"), "unique local (private)"},
	{netip.MustParsePrefix("fe80::/10"), "link-local"},
	{netip.MustParsePrefix("ff00::/8"), "multicast"},
}

// kindOf names what a range is for: the special block holding all of
// it, "public" when it touches none, "mixed" when it spans several.
func kindOf(p netip.Prefix) string {
	p = p.Masked()
	touches := false
	for _, s := range special {
		if s.prefix.Addr().Is4() != p.Addr().Is4() {
			continue
		}
		if s.prefix.Bits() <= p.Bits() && s.prefix.Contains(p.Addr()) {
			return s.kind
		}
		if p.Overlaps(s.prefix) {
			touches = true
		}
	}
	if touches {
		return "mixed"
	}
	return "public"
}
