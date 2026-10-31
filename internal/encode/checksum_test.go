package encode

import (
	"strings"
	"testing"
	"time"
)

func TestChecksum(t *testing.T) {
	d, n, err := Checksum(strings.NewReader("abc"))
	if err != nil || n != 3 || len(d) != len(algorithms) {
		t.Fatalf("%v %d %v", d, n, err)
	}
	if d[0].Algorithm != "SHA-256" || d[0].Hex != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Errorf("sha256: %+v", d[0])
	}
	only, _, err := Checksum(strings.NewReader("abc"), "md5")
	if err != nil || len(only) != 1 || only[0].Hex != "900150983cd24fb0d6963f7d28e17f72" {
		t.Errorf("md5 only: %+v %v", only, err)
	}
	if _, _, err := Checksum(strings.NewReader(""), "crc32"); err == nil {
		t.Error("crc32 accepted")
	}
}

func TestParseExpected(t *testing.T) {
	sha := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	cases := []struct {
		in, alg, name string
	}{
		{sha, "SHA-256", ""},
		{strings.ToUpper(sha), "SHA-256", ""},
		{"ba:78:16:bf:8f:01:cf:ea:41:41:40:de:5d:ae:22:23:b0:03:61:a3:96:17:7a:9c:b4:10:ff:61:f2:00:15:ad", "SHA-256", ""},
		{"ungWv48Bz+pBQUDeXa4iI7ADYaOWF3qctBD/YfIAFa0=", "SHA-256", ""},
		{sha + "  codec_3.7.0_linux_amd64.tar.gz", "SHA-256", "codec_3.7.0_linux_amd64.tar.gz"},
		{sha + " *codec.exe", "SHA-256", "codec.exe"},
		{"SHA256 (codec.zip) = " + sha, "SHA-256", "codec.zip"},
		{"SHA2-256(codec.zip)= " + sha, "SHA-256", "codec.zip"},
		{"900150983cd24fb0d6963f7d28e17f72", "MD5", ""},
	}
	for _, c := range cases {
		es, err := ParseExpected(c.in)
		if err != nil || len(es) != 1 || es[0].Algorithm != c.alg || es[0].Name != c.name || es[0].Hex != strings.ToLower(es[0].Hex) {
			t.Errorf("%q: %+v %v", c.in, es, err)
		}
	}
	list := "# release\n" + sha + "  a.tar.gz\n\nd41d8cd98f00b204e9800998ecf8427e  b.zip\n" // b.zip: MD5 of nothing
	es, err := ParseExpected(list)
	if err != nil || len(es) != 2 || es[1].Name != "b.zip" || es[1].Line != 4 {
		t.Errorf("list: %+v %v", es, err)
	}
	for _, bad := range []string{"", "hello world", "abcd", "MD5 (x) = " + sha} {
		if _, err := ParseExpected(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	d, _, _ := Checksum(strings.NewReader("abc"))
	if found, equal := es[0].Match(d); !found || !equal {
		t.Error("no match")
	}
	if found, equal := es[1].Match(d); !found || equal {
		t.Error("md5 of b.zip matched abc")
	}
}

func TestUUIDs(t *testing.T) {
	for _, v := range []int{0, 4, 7} {
		us, err := NewUUIDs(UUIDOptions{Version: v}, 3)
		if err != nil || len(us) != 3 || us[0] == us[1] {
			t.Fatalf("v%d: %v %v", v, us, err)
		}
		info, err := ParseUUID(us[0])
		want := max(v, 4)
		if err != nil || info.Version != want || info.Variant != "RFC 9562" {
			t.Errorf("v%d parsed as %+v %v", v, info, err)
		}
	}
	// RFC 9562 appendix A: v5 of www.example.com in the DNS namespace.
	us, err := NewUUIDs(UUIDOptions{Version: 5, Namespace: "dns", Name: "www.example.com"}, 10)
	if err != nil || len(us) != 1 || us[0] != "2ed6657d-e927-568b-95e1-2665a8aea6a2" {
		t.Errorf("v5: %v %v", us, err)
	}
	us, err = NewUUIDs(UUIDOptions{Version: 3, Namespace: "6ba7b810-9dad-11d1-80b4-00c04fd430c8", Name: "www.example.com"}, 1)
	if err != nil || us[0] != "5df41881-3aed-3515-88a7-2f4a814cf09e" {
		t.Errorf("v3: %v %v", us, err)
	}
	if _, err := NewUUIDs(UUIDOptions{Version: 5, Namespace: "nope"}, 1); err == nil {
		t.Error("bad namespace accepted")
	}
	if _, err := NewUUIDs(UUIDOptions{Version: 2}, 1); err == nil {
		t.Error("version 2 accepted")
	}
	if us, _ := NewUUIDs(UUIDOptions{}, 5000); len(us) != MaxUUIDs {
		t.Errorf("bulk %d", len(us))
	}
}

func TestParseUUID(t *testing.T) {
	// RFC 9562 appendix A.6: a version 7 example, 2022-02-22T19:22:22Z.
	info, err := ParseUUID("{017F22E2-79B0-7CC3-98C4-DC0C0C07398F}")
	if err != nil || info.UUID != "017f22e2-79b0-7cc3-98c4-dc0c0c07398f" || info.Version != 7 || !strings.HasPrefix(info.Time, "2022-02-22T19:22:22") {
		t.Errorf("v7: %+v %v", info, err)
	}
	if _, err := time.Parse(time.RFC3339Nano, info.Time); err != nil {
		t.Error(err)
	}
	info, err = ParseUUID("urn:uuid:c232ab00-9414-11ec-b3c8-9f6bdeced846")
	if err != nil || info.Version != 1 || !strings.HasPrefix(info.Time, "2022-02-22T19:22:22") {
		t.Errorf("v1: %+v %v", info, err)
	}
	info, _ = ParseUUID("00000000000000000000000000000000")
	if !info.Nil || info.Kind != "the nil UUID" {
		t.Errorf("nil: %+v", info)
	}
	// The ULID spec's example: 01ARZ3NDEKTSV4RRFFQ69G5FAV, made at
	// 2016-07-30T23:54:10.259Z.
	info, err = ParseUUID("01arz3ndektsv4rrffq69g5fav")
	if err != nil || info.ULID != "01ARZ3NDEKTSV4RRFFQ69G5FAV" || info.Time != "2016-07-30T23:54:10.259Z" || info.UUID != "01563e3a-b5d3-d676-4c61-efb99302bd5b" {
		t.Errorf("ulid: %+v %v", info, err)
	}
	for _, bad := range []string{"81ARZ3NDEKTSV4RRFFQ69G5FAV", "01ARZ3NDEKTSV4RRFFQ69G5FAI", "01ARZ3NDEK"} {
		if _, _, ok := ParseULID(bad); ok {
			t.Errorf("%q read as a ULID", bad)
		}
	}
	if _, err := ParseUUID("not-a-uuid"); err == nil {
		t.Error("garbage parsed")
	}
}
