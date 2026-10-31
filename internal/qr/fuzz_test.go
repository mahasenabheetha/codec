package qr

import "testing"

// FuzzWiFi: any network name and password survive escaping: building
// the payload and explaining it gives them back, and Explain never
// panics on what a code may hold.
func FuzzWiFi(f *testing.F) {
	f.Add("home", "pass")
	f.Add(`a;b\c`, `x:y,"z"`)
	f.Fuzz(func(t *testing.T, ssid, pass string) {
		Explain(ssid)
		p, err := WiFi{SSID: ssid, Password: pass}.Payload()
		if err != nil {
			return
		}
		got := map[string]string{}
		for _, fl := range Explain(p).Fields {
			got[fl.Name] = fl.Value
		}
		if got["Network"] != ssid || got["Password"] != pass {
			t.Fatalf("%q / %q came back as %q / %q from %q", ssid, pass, got["Network"], got["Password"], p)
		}
	})
}
