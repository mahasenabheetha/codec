package web

import (
	"cmp"
	"net/http"

	"github.com/mahasenabheetha/codec/v2/internal/encode"
)

// maxCount bounds how many secrets or UUIDs one request generates.
const maxCount = 100

// encodeRequest carries every Encode & hash job; each reads its fields.
type encodeRequest struct {
	Input string `json:"input"`
	Op    string `json:"op"` // url: encode, decode, parse; hex: encode, decode; htpasswd: make, check
	// URL
	Whole bool `json:"whole"`
	Plus  bool `json:"plus"`
	// Hex
	Upper bool   `json:"upper"`
	Sep   string `json:"sep"`
	// Hash: HMAC when Key is set (an empty key is a valid HMAC key).
	Key        *string  `json:"key"`
	Algorithms []string `json:"algorithms"`
	// Secrets and UUIDs
	Secret encode.SecretOptions `json:"secret"`
	Count  int                  `json:"count"`
	// htpasswd
	User     string `json:"user"`
	Password string `json:"password"`
	Cost     int    `json:"cost"`
	Line     string `json:"line"`
}

// POST /api/v2/encode/{kind}: url, hex, hash, secret, uuid, htpasswd.
// Input and output are never kept (decision 19). Bad input is a 422
// with the reason.
func (s *Server) handleEncode(w http.ResponseWriter, r *http.Request) {
	var req encodeRequest
	if !decode(w, r, &req) {
		return
	}
	fail := func(err error) { writeError(w, http.StatusUnprocessableEntity, err.Error()) }
	count := min(max(req.Count, 1), maxCount)
	switch kind := r.PathValue("kind"); kind + ":" + req.Op {
	case "url:encode":
		writeJSON(w, http.StatusOK, map[string]any{"output": encode.URLEncode(req.Input, req.Whole, req.Plus)})
	case "url:decode":
		out, err := encode.URLDecode(req.Input, req.Plus)
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"output": out})
	case "url:parse":
		p, err := encode.ParseURL(req.Input)
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"parts": p})
	case "hex:encode":
		writeJSON(w, http.StatusOK, map[string]any{"output": encode.HexEncode([]byte(req.Input), req.Upper, req.Sep)})
	case "hex:decode":
		b, err := encode.HexDecode(req.Input)
		if err != nil {
			fail(err)
			return
		}
		out, text := encode.Printable(b)
		writeJSON(w, http.StatusOK, map[string]any{"output": out, "text": text, "bytes": len(b)})
	case "hash:":
		var key []byte
		if req.Key != nil {
			key = []byte(*req.Key)
		}
		d, err := encode.Hash([]byte(req.Input), key, req.Algorithms...)
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"digests": d, "hmac": key != nil})
	case "secret:":
		out := make([]encode.Secret, 0, count)
		for range count {
			sec, err := encode.NewSecret(req.Secret)
			if err != nil {
				fail(err)
				return
			}
			out = append(out, sec)
		}
		writeJSON(w, http.StatusOK, map[string]any{"secrets": out})
	case "uuid:":
		out := make([]string, count)
		for i := range out {
			out[i] = encode.UUID()
		}
		writeJSON(w, http.StatusOK, map[string]any{"uuids": out})
	case "htpasswd:make":
		line, err := encode.Htpasswd(req.User, req.Password, cmp.Or(req.Cost, encode.DefaultCost))
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"line": line})
	case "htpasswd:check":
		ok, err := encode.HtpasswdCheck(req.Line, req.Password)
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"match": ok})
	default:
		writeError(w, http.StatusBadRequest, "unknown job "+kind+" "+req.Op)
	}
}
