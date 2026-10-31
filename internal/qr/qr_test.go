package qr

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"
)

func roundTrip(t *testing.T, text, level string) string {
	t.Helper()
	c, err := Encode(text, level)
	if err != nil {
		t.Fatalf("%q: %v", text, err)
	}
	b, err := c.PNG(4)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Read(img)
	if err != nil {
		t.Fatalf("%q at %s: %v", text, level, err)
	}
	return got
}

func TestRoundTrip(t *testing.T) {
	for _, c := range []struct{ text, level string }{
		{"https://example.com/a?b=1", "M"},
		{"1234567890", "L"},
		{"HELLO WORLD", "Q"},
		{"WIFI:T:WPA;S:home\\;net;P:p@ss;;", "H"},
		{"Ünïcödé text, ok", "M"},
		{strings.Repeat("x", 1000), "L"},
	} {
		if got := roundTrip(t, c.text, c.level); got != c.text {
			t.Errorf("read %q, want %q", got, c.text)
		}
	}
}

func TestEncodeShapes(t *testing.T) {
	c, err := Encode("hi", "")
	if err != nil || c.Size != 21 || c.Version != 1 || c.Level != "M" {
		t.Fatalf("%+v %v", c, err)
	}
	svg := c.SVG()
	if !strings.HasPrefix(svg, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 29 29"`) || !strings.Contains(svg, `d="M4 4h7v1h-7z`) {
		t.Errorf("svg: %.200s", svg)
	}
	if lines := strings.Count(c.Terminal(), "\n"); lines != 15 { // 29 rows, two a line
		t.Errorf("terminal lines %d", lines)
	}
	if _, err := Encode("", "M"); err == nil {
		t.Error("empty accepted")
	}
	if _, err := Encode("x", "Z"); err == nil {
		t.Error("level Z accepted")
	}
	if _, err := Encode(strings.Repeat("x", 2900), "H"); err == nil {
		t.Error("2900 bytes at H accepted")
	}
	if _, err := Read(image.NewGray(image.Rect(0, 0, 50, 50))); err == nil {
		t.Error("a blank image read")
	}
}

func TestInverted(t *testing.T) {
	c, _ := Encode("dark mode", "M")
	b, _ := c.PNG(4)
	img, _ := png.Decode(bytes.NewReader(b))
	got, err := Read(invert{img})
	if err != nil || got != "dark mode" {
		t.Errorf("inverted: %q %v", got, err)
	}
}

func TestPayloads(t *testing.T) {
	w, err := WiFi{SSID: `Café;5G`, Password: `a:b,"c"\`, Security: "wpa2", Hidden: true}.Payload()
	if err != nil || w != `WIFI:T:WPA;S:Café\;5G;P:a\:b\,\"c\"\\;H:true;;` {
		t.Errorf("wifi: %q %v", w, err)
	}
	m := Explain(w)
	want := []Field{{"Security", "WPA"}, {"Network", "Café;5G"}, {"Password", `a:b,"c"\`}, {"Hidden", "true"}}
	if m.Kind != "wifi" || len(m.Fields) != 4 || m.Fields[1] != want[1] || m.Fields[2] != want[2] {
		t.Errorf("wifi explained: %+v", m)
	}
	if open, _ := (WiFi{SSID: "guest", Security: "none"}).Payload(); open != "WIFI:T:nopass;S:guest;;" {
		t.Errorf("open: %q", open)
	}
	for _, bad := range []WiFi{{}, {SSID: "x"}, {SSID: "x", Password: "p", Security: "wpa9"}} {
		if _, err := bad.Payload(); err == nil {
			t.Errorf("%+v accepted", bad)
		}
	}

	u, err := TOTP{Issuer: "Example Co", Account: "ann@example.com", Secret: "jbsw y3dp ehpk 3pxp", Digits: 8, Period: 60, Algorithm: "sha256"}.Payload()
	if err != nil || u != "otpauth://totp/Example%20Co:ann@example.com?algorithm=SHA256&digits=8&issuer=Example+Co&period=60&secret=JBSWY3DPEHPK3PXP" {
		t.Errorf("totp: %q %v", u, err)
	}
	m = Explain(u)
	if m.Kind != "otpauth" || !hasField(m.Fields, "Issuer", "Example Co") || !hasField(m.Fields, "Account", "ann@example.com") || !hasField(m.Fields, "Secret", "JBSWY3DPEHPK3PXP") {
		t.Errorf("totp explained: %+v", m)
	}
	for _, bad := range []TOTP{{Account: "a"}, {Account: "a", Secret: "not base32!"}, {Account: "a", Secret: "JBSWY3DP", Digits: 7}} {
		if _, err := bad.Payload(); err == nil {
			t.Errorf("%+v accepted", bad)
		}
	}
	for text, kind := range map[string]string{"https://example.com/x": "url", "mailto:a@example.com?subject=hi": "email", "tel:+15550100": "phone", "geo:59.33,18.06": "geo", "hello": "text"} {
		if k := Explain(text).Kind; k != kind {
			t.Errorf("%q: %s, want %s", text, k, kind)
		}
	}
}
