package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWhatsappWebHandler_RedirectsEveryRequest(t *testing.T) {
	h := whatsappWebHandler()

	for _, target := range []string{"/", "/index.html", "/some/asset.js"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://wails.localhost"+target, nil))

		if rec.Code != http.StatusFound {
			t.Errorf("%s: status = %d, want %d", target, rec.Code, http.StatusFound)
		}
		if loc := rec.Header().Get("Location"); loc != whatsappURL {
			t.Errorf("%s: Location = %q, want %q", target, loc, whatsappURL)
		}
	}
}
