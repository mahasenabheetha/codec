package web

import (
	"encoding/base64"
	"errors"
	_ "image/gif" // images a code may be read from
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"

	"github.com/mahasenabheetha/codec/v2/internal/qr"
)

// maxQRImage bounds an image sent to be read.
const maxQRImage = 20 << 20

type qrRequest struct {
	Kind  string  `json:"kind"` // text (default), wifi, totp
	Text  string  `json:"text"`
	WiFi  qr.WiFi `json:"wifi"`
	TOTP  qr.TOTP `json:"totp"`
	Level string  `json:"level"` // L, M, Q, H
	Scale int     `json:"scale"` // PNG pixels a module
}

// POST /api/v2/qr/make: a code for text, a Wi-Fi login or a TOTP setup
// link, as SVG and PNG (base64), with the payload encoded. Nothing is
// kept (decision 19).
func (s *Server) handleQRMake(w http.ResponseWriter, r *http.Request) {
	var req qrRequest
	if !decode(w, r, &req) {
		return
	}
	payload := req.Text
	var err error
	switch req.Kind {
	case "", "text":
	case "wifi":
		payload, err = req.WiFi.Payload()
	case "totp":
		payload, err = req.TOTP.Payload()
	default:
		err = errors.New("kind is text, wifi or totp")
	}
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	c, err := qr.Encode(payload, req.Level)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	png, err := c.PNG(max(req.Scale, 8))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"payload": payload, "svg": c.SVG(), "png": base64.StdEncoding.EncodeToString(png),
		"size": c.Size, "version": c.Version, "level": c.Level,
	})
}

// POST /api/v2/qr/read, body: a PNG, JPEG or GIF (the UI redraws other
// formats as PNG first). The code's text and what it means.
func (s *Server) handleQRRead(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxQRImage))
	if err != nil {
		if errors.As(err, new(*http.MaxBytesError)) {
			writeError(w, http.StatusRequestEntityTooLarge, "the image is over 20 MB")
			return
		}
		writeError(w, http.StatusBadRequest, "reading the image: "+err.Error())
		return
	}
	img, err := qr.Decode(data) // its size checked before the pixels are
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	text, err := qr.Read(img)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"text": text, "meaning": qr.Explain(text)})
}
