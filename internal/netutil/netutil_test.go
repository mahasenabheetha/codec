package netutil

import (
	"strings"
	"testing"
)

func TestDetails(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want Range
	}{
		{"10.0.0.0/24", Range{CIDR: "10.0.0.0/24", Version: 4, Prefix: 24, Network: "10.0.0.0", Broadcast: "10.0.0.255", First: "10.0.0.1", Last: "10.0.0.254", Mask: "255.255.255.0", Wildcard: "0.0.0.255", Addresses: "256", Hosts: "254", Kind: "private"}},
		{"192.168.1.77/26", Range{CIDR: "192.168.1.64/26", Version: 4, Prefix: 26, Network: "192.168.1.64", Broadcast: "192.168.1.127", First: "192.168.1.65", Last: "192.168.1.126", Mask: "255.255.255.192", Wildcard: "0.0.0.63", Addresses: "64", Hosts: "62", Kind: "private", Host: "192.168.1.77"}},
		{"172.16.0.0 255.240.0.0", Range{CIDR: "172.16.0.0/12", Version: 4, Prefix: 12, Network: "172.16.0.0", Broadcast: "172.31.255.255", First: "172.16.0.1", Last: "172.31.255.254", Mask: "255.240.0.0", Wildcard: "0.15.255.255", Addresses: "1048576", Hosts: "1048574", Kind: "private"}},
		{"203.0.113.8/31", Range{CIDR: "203.0.113.8/31", Version: 4, Prefix: 31, Network: "203.0.113.8", First: "203.0.113.8", Last: "203.0.113.9", Mask: "255.255.255.254", Wildcard: "0.0.0.1", Addresses: "2", Hosts: "2", Kind: "documentation"}},
		{"8.8.8.8", Range{CIDR: "8.8.8.8/32", Version: 4, Prefix: 32, Network: "8.8.8.8", First: "8.8.8.8", Last: "8.8.8.8", Mask: "255.255.255.255", Wildcard: "0.0.0.0", Addresses: "1", Hosts: "1", Kind: "public"}},
		{"0.0.0.0/0", Range{CIDR: "0.0.0.0/0", Version: 4, Prefix: 0, Network: "0.0.0.0", Broadcast: "255.255.255.255", First: "0.0.0.1", Last: "255.255.255.254", Mask: "0.0.0.0", Wildcard: "255.255.255.255", Addresses: "4294967296", Hosts: "4294967294", Kind: "mixed"}},
		{"100.64.0.0/10", Range{CIDR: "100.64.0.0/10", Version: 4, Prefix: 10, Network: "100.64.0.0", Broadcast: "100.127.255.255", First: "100.64.0.1", Last: "100.127.255.254", Mask: "255.192.0.0", Wildcard: "0.63.255.255", Addresses: "4194304", Hosts: "4194302", Kind: "shared (carrier-grade NAT)"}},
		{"2001:db8:abcd::/48", Range{CIDR: "2001:db8:abcd::/48", Version: 6, Prefix: 48, Network: "2001:db8:abcd::", First: "2001:db8:abcd::", Last: "2001:db8:abcd:ffff:ffff:ffff:ffff:ffff", Mask: "ffff:ffff:ffff::", Wildcard: "::ffff:ffff:ffff:ffff:ffff", Addresses: "1208925819614629174706176", Hosts: "1208925819614629174706176", Kind: "documentation"}},
		{"fd00::1/64", Range{CIDR: "fd00::/64", Version: 6, Prefix: 64, Network: "fd00::", First: "fd00::", Last: "fd00::ffff:ffff:ffff:ffff", Mask: "ffff:ffff:ffff:ffff::", Wildcard: "::ffff:ffff:ffff:ffff", Addresses: "18446744073709551616", Hosts: "18446744073709551616", Kind: "unique local (private)", Host: "fd00::1"}},
	} {
		got, err := Details(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("%s:\n got %+v (%v)\nwant %+v", tt.in, got, err, tt.want)
		}
	}
	for _, bad := range []string{"", "10.0.0.0/33", "10.0.0", "10.0.0.0 255.0.255.0", "::1 255.0.0.0", "nonsense/8"} {
		if _, err := Details(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestContains(t *testing.T) {
	for _, tt := range []struct {
		rng, ip string
		want    bool
	}{
		{"10.0.0.0/8", "10.200.3.4", true},
		{"10.0.0.0/8", "11.0.0.1", false},
		{"2001:db8::/32", "2001:db8:1::5", true},
		{"10.0.0.0/8", "::ffff:10.0.0.1", false}, // another family
	} {
		if got, err := Contains(tt.rng, tt.ip); err != nil || got != tt.want {
			t.Errorf("%s in %s: %v (%v)", tt.ip, tt.rng, got, err)
		}
	}
	if _, err := Contains("10.0.0.0/8", "10.0.0"); err == nil {
		t.Error("bad address accepted")
	}
}

func TestSplit(t *testing.T) {
	s, err := Split("10.0.0.0/24", 26)
	if err != nil || s.Count != "4" || strings.Join(s.Subnets, " ") != "10.0.0.0/26 10.0.0.64/26 10.0.0.128/26 10.0.0.192/26" || s.Each.Hosts != "62" || s.Truncated {
		t.Errorf("split /24 into /26: %+v (%v)", s, err)
	}
	s, _ = Split("10.0.0.0/8", 24)
	if s.Count != "65536" || len(s.Subnets) != MaxSubnets || !s.Truncated || s.Subnets[1] != "10.0.1.0/24" {
		t.Errorf("split /8 into /24: count %s, %d listed, truncated %v", s.Count, len(s.Subnets), s.Truncated)
	}
	s, _ = Split("2001:db8::/32", 48)
	if s.Count != "65536" || s.Subnets[1] != "2001:db8:1::/48" {
		t.Errorf("v6 split: %s %v", s.Count, s.Subnets[:2])
	}
	if s, _ := Split("10.0.0.0/24", 24); s.Count != "1" || len(s.Subnets) != 1 {
		t.Errorf("split into itself: %+v", s)
	}
	for _, bits := range []int{23, 33} {
		if _, err := Split("10.0.0.0/24", bits); err == nil {
			t.Errorf("split into /%d accepted", bits)
		}
	}
}

func TestOverlaps(t *testing.T) {
	list, pairs, err := Overlaps([]string{
		"10.0.0.0/16   # hub",
		"",
		"10.0.4.0/22",
		"10.1.0.0/16",
		"10.0.0.0/16",
		"not a range",
		"192.168.0.0/24",
		"10.0.0.0/8",
		"2001:db8::/32",
	})
	if err != nil || len(list) != 8 || list[4].Error == "" || list[0].CIDR != "10.0.0.0/16" || list[0].Note != "hub" {
		t.Fatalf("list %+v (%v)", list, err)
	}
	want := map[Overlap]bool{
		{0, 1, "contains"}: true, // 10.0.0.0/16 holds 10.0.4.0/22
		{0, 3, "same"}:     true,
		{3, 1, "contains"}: true,
		{6, 0, "contains"}: true, // 10/8 holds the rest of 10.x
		{6, 1, "contains"}: true,
		{6, 2, "contains"}: true,
		{6, 3, "contains"}: true,
	}
	if len(pairs) != len(want) {
		t.Errorf("pairs %+v", pairs)
	}
	for _, p := range pairs {
		if !want[p] {
			t.Errorf("unexpected %+v", p)
		}
	}
	if _, _, err := Overlaps(make([]string, 0)); err != nil {
		t.Error(err)
	}
	many := make([]string, MaxRanges+1)
	for i := range many {
		many[i] = "10.0.0.0/24"
	}
	if _, _, err := Overlaps(many); err == nil {
		t.Error("over the limit accepted")
	}
}

func TestParseIP(t *testing.T) {
	ip, err := ParseIP("10.0.0.1")
	want := IP{Address: "10.0.0.1", Version: 4, Integer: "167772161", Hex: "0x0a000001", Binary: "00001010.00000000.00000000.00000001", Mapped: "::ffff:10.0.0.1", Reverse: "1.0.0.10.in-addr.arpa", Kind: "private"}
	if err != nil || ip != want {
		t.Errorf("10.0.0.1:\n got %+v\nwant %+v", ip, want)
	}
	for _, in := range []string{"167772161", "0x0a000001", "0x0A00_0001", "0b00001010000000000000000000000001"} {
		if ip, err := ParseIP(in); err != nil || ip.Address != "10.0.0.1" {
			t.Errorf("%s: %+v (%v)", in, ip, err)
		}
	}
	ip, _ = ParseIP("[2001:DB8::1]")
	if ip.Address != "2001:db8::1" || ip.Expanded != "2001:0db8:0000:0000:0000:0000:0000:0001" || ip.Version != 6 || ip.Kind != "documentation" ||
		ip.Reverse != "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa" || !strings.HasPrefix(ip.Binary, "0010000000000001:0000110110111000:") {
		t.Errorf("v6: %+v", ip)
	}
	if ip, _ := ParseIP("::ffff:192.168.1.1"); ip.Mapped != "192.168.1.1" || ip.Version != 6 {
		t.Errorf("mapped: %+v", ip)
	}
	if ip, _ := ParseIP("0x00000000000000000000000000000001"); ip.Address != "::1" {
		t.Errorf("32 hex digits are IPv6: %+v", ip)
	}
	if ip, _ := ParseIP("4294967296"); ip.Address != "::1:0:0" {
		t.Errorf("2^32 is IPv6: %+v", ip)
	}
	if ip, _ := ParseIP("fe80::1%eth0"); ip.Address != "fe80::1" || ip.Kind != "link-local" {
		t.Errorf("zone: %+v", ip)
	}
	for _, bad := range []string{"", "1.2.3", "-1", "0x" + strings.Repeat("f", 33), "340282366920938463463374607431768211456", "hello"} {
		if _, err := ParseIP(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
