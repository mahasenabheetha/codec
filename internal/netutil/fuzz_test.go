package netutil

import (
	"net/netip"
	"testing"
)

// FuzzDetails: any input either fails or gives a range whose own CIDR
// reads back to the same range, with the first host inside it.
func FuzzDetails(f *testing.F) {
	for _, s := range []string{"10.0.0.0/24", "192.168.1.77/26", "10.0.0.0 255.255.0.0", "2001:db8::/48", "::/0", "8.8.8.8", "fe80::1%eth0/64", "1.2.3.4 0.0.0.0"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		r, err := Details(s)
		if err != nil {
			return
		}
		again, err := Details(r.CIDR)
		if err != nil || again.CIDR != r.CIDR || again.Addresses != r.Addresses {
			t.Fatalf("%q gave %q, which reads back as %+v (%v)", s, r.CIDR, again, err)
		}
		if ok, err := Contains(r.CIDR, r.First); err != nil || !ok {
			t.Fatalf("%q: first host %s outside %s", s, r.First, r.CIDR)
		}
	})
}

// FuzzParseIP: any input either fails or gives an address whose integer
// and hex forms read back to it.
func FuzzParseIP(f *testing.F) {
	for _, s := range []string{"10.0.0.1", "167772161", "0x0a000001", "0b101", "::ffff:1.2.3.4", "[2001:db8::1]", "fe80::1%eth0", "4294967296"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		ip, err := ParseIP(s)
		if err != nil {
			return
		}
		want := netip.MustParseAddr(ip.Address)
		for _, form := range []string{ip.Hex, ip.Address, ip.Expanded} {
			if form == "" {
				continue
			}
			back, err := ParseIP(form)
			if err != nil || netip.MustParseAddr(back.Address) != want {
				t.Fatalf("%q: %q read back as %+v (%v)", s, form, back, err)
			}
		}
	})
}

// FuzzOverlaps: never panics, and every pair's ranges really overlap.
func FuzzOverlaps(f *testing.F) {
	f.Add("10.0.0.0/16", "10.0.1.0/24", "x")
	f.Add("::/0", "2001:db8::/32", "10.0.0.0/8")
	f.Fuzz(func(t *testing.T, a, b, c string) {
		list, pairs, err := Overlaps([]string{a, b, c})
		if err != nil {
			return
		}
		for _, p := range pairs {
			pa, pb := netip.MustParsePrefix(list[p.A].CIDR), netip.MustParsePrefix(list[p.B].CIDR)
			if !pa.Overlaps(pb) || pa.Bits() > pb.Bits() {
				t.Fatalf("%v: %s and %s", p, pa, pb)
			}
		}
	})
}
