package qr

import (
	"encoding/base32"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// WiFi is a network a phone joins by scanning: WIFI:T:WPA;S:…;P:…;;
type WiFi struct {
	SSID     string `json:"ssid"`
	Password string `json:"password"`
	Security string `json:"security"` // WPA (also WPA2/WPA3), WEP, nopass
	Hidden   bool   `json:"hidden"`
}

// wifiEscape escapes the characters the Wi-Fi format reserves.
var wifiEscape = strings.NewReplacer(`\`, `\\`, `;`, `\;`, `,`, `\,`, `:`, `\:`, `"`, `\"`)

// Payload is the text to encode for w.
func (w WiFi) Payload() (string, error) {
	if w.SSID == "" {
		return "", errors.New("the network name (SSID) is empty")
	}
	sec := strings.ToUpper(w.Security)
	switch sec {
	case "", "WPA", "WPA2", "WPA3":
		sec = "WPA"
	case "WEP":
	case "NOPASS", "NONE", "OPEN":
		sec = "nopass"
	default:
		return "", fmt.Errorf("security is WPA, WEP or none, not %q", w.Security)
	}
	if sec != "nopass" && w.Password == "" {
		return "", errors.New("a secured network needs its password")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "WIFI:T:%s;S:%s;", sec, wifiEscape.Replace(w.SSID))
	if sec != "nopass" {
		fmt.Fprintf(&b, "P:%s;", wifiEscape.Replace(w.Password))
	}
	if w.Hidden {
		b.WriteString("H:true;")
	}
	b.WriteString(";")
	return b.String(), nil
}

// TOTP is an authenticator setup link: otpauth://totp/Issuer:account?…
type TOTP struct {
	Issuer    string `json:"issuer"`
	Account   string `json:"account"`
	Secret    string `json:"secret"`    // base32
	Digits    int    `json:"digits"`    // 6 (default) or 8
	Period    int    `json:"period"`    // seconds, 30 by default
	Algorithm string `json:"algorithm"` // SHA1 (default), SHA256, SHA512
}

// Payload is the otpauth URI for t, the secret checked as base32.
func (t TOTP) Payload() (string, error) {
	if t.Account == "" {
		return "", errors.New("the account (often an email) is empty")
	}
	secret := strings.ToUpper(strings.NewReplacer(" ", "", "-", "").Replace(t.Secret))
	secret = strings.TrimRight(secret, "=")
	if secret == "" {
		return "", errors.New("the secret is empty")
	}
	if _, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret); err != nil {
		return "", errors.New("the secret is not base32 (letters A–Z and digits 2–7)")
	}
	label := url.PathEscape(t.Account)
	if t.Issuer != "" {
		label = url.PathEscape(t.Issuer) + ":" + label
	}
	q := url.Values{"secret": {secret}}
	if t.Issuer != "" {
		q.Set("issuer", t.Issuer)
	}
	if t.Digits != 0 && t.Digits != 6 {
		if t.Digits != 8 {
			return "", errors.New("digits are 6 or 8")
		}
		q.Set("digits", "8")
	}
	if t.Period != 0 && t.Period != 30 {
		if t.Period < 1 || t.Period > 300 {
			return "", errors.New("the period is 1 to 300 seconds")
		}
		q.Set("period", strconv.Itoa(t.Period))
	}
	switch alg := strings.ToUpper(t.Algorithm); alg {
	case "", "SHA1":
	case "SHA256", "SHA512":
		q.Set("algorithm", alg)
	default:
		return "", fmt.Errorf("algorithm is SHA1, SHA256 or SHA512, not %q", t.Algorithm)
	}
	return "otpauth://totp/" + label + "?" + q.Encode(), nil
}

// Field is one named part of what a code holds.
type Field struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Meaning says what text read from a code is: a link, a Wi-Fi login, an
// authenticator setup, an email, a phone number, or plain text, with
// its parts.
type Meaning struct {
	Kind   string  `json:"kind"` // url, wifi, otpauth, email, phone, sms, geo, text
	Fields []Field `json:"fields,omitempty"`
}

// Explain reads the common payloads.
func Explain(text string) Meaning {
	low := strings.ToLower(text)
	switch {
	case strings.HasPrefix(low, "wifi:"):
		return Meaning{Kind: "wifi", Fields: wifiFields(text[5:])}
	case strings.HasPrefix(low, "otpauth://"):
		u, err := url.Parse(text)
		if err != nil {
			break
		}
		m := Meaning{Kind: "otpauth", Fields: []Field{{"Type", u.Host}}}
		label, _ := url.PathUnescape(strings.TrimPrefix(u.Path, "/"))
		if issuer, account, ok := strings.Cut(label, ":"); ok {
			m.Fields = append(m.Fields, Field{"Issuer", issuer}, Field{"Account", account})
		} else {
			m.Fields = append(m.Fields, Field{"Account", label})
		}
		q := u.Query()
		for _, k := range []string{"issuer", "secret", "digits", "period", "algorithm", "counter"} {
			if v := q.Get(k); v != "" && !(k == "issuer" && hasField(m.Fields, "Issuer", v)) {
				m.Fields = append(m.Fields, Field{strings.ToUpper(k[:1]) + k[1:], v})
			}
		}
		return m
	case strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://"):
		if u, err := url.Parse(text); err == nil && u.Host != "" {
			return Meaning{Kind: "url", Fields: []Field{{"Host", u.Host}}}
		}
	case strings.HasPrefix(low, "mailto:"):
		return Meaning{Kind: "email", Fields: []Field{{"To", strings.SplitN(text[7:], "?", 2)[0]}}}
	case strings.HasPrefix(low, "tel:"):
		return Meaning{Kind: "phone", Fields: []Field{{"Number", text[4:]}}}
	case strings.HasPrefix(low, "smsto:") || strings.HasPrefix(low, "sms:"):
		return Meaning{Kind: "sms", Fields: []Field{{"Number", strings.SplitN(text[strings.Index(text, ":")+1:], ":", 2)[0]}}}
	case strings.HasPrefix(low, "geo:"):
		return Meaning{Kind: "geo", Fields: []Field{{"Place", text[4:]}}}
	}
	return Meaning{Kind: "text"}
}

func hasField(fs []Field, name, value string) bool {
	for _, f := range fs {
		if f.Name == name && f.Value == value {
			return true
		}
	}
	return false
}

// wifiFields splits T:WPA;S:name;P:pass;H:true;; honouring \ escapes.
func wifiFields(s string) []Field {
	names := map[string]string{"T": "Security", "S": "Network", "P": "Password", "H": "Hidden"}
	var out []Field
	var cur strings.Builder
	flush := func() {
		part := cur.String()
		cur.Reset()
		k, v, ok := strings.Cut(part, ":")
		if !ok || v == "" {
			return
		}
		if n, known := names[k]; known {
			out = append(out, Field{n, v})
		}
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			if i+1 < len(s) {
				i++
				cur.WriteByte(s[i])
			}
		case ';':
			flush()
		default:
			cur.WriteByte(s[i])
		}
	}
	flush()
	return out
}
