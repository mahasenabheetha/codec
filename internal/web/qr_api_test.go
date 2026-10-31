package web

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestQRAPI(t *testing.T) {
	s := New(testOptions(false))
	defer s.Close()
	do := func(path, body string) (int, map[string]any) {
		rec := call(s, "POST", path, "127.0.0.1:8765", s.Token(), "", body)
		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	code, out := do("/api/v2/qr/make", `{"kind":"wifi","wifi":{"ssid":"guest","security":"none"},"level":"Q"}`)
	if code != 200 || out["payload"] != "WIFI:T:nopass;S:guest;;" || out["level"] != "Q" || !strings.HasPrefix(out["svg"].(string), "<svg") {
		t.Fatalf("make: %d %v", code, out)
	}
	png, _ := base64.StdEncoding.DecodeString(out["png"].(string))
	code, out = do("/api/v2/qr/read", string(png))
	if code != 200 || out["text"] != "WIFI:T:nopass;S:guest;;" || out["meaning"].(map[string]any)["kind"] != "wifi" {
		t.Errorf("read: %d %v", code, out)
	}
	if code, _ := do("/api/v2/qr/make", `{"kind":"totp","totp":{"account":"a","secret":"!!"}}`); code != 422 {
		t.Errorf("bad totp: %d", code)
	}
	if code, _ := do("/api/v2/qr/read", "not an image"); code != 422 {
		t.Errorf("not an image: %d", code)
	}
}
