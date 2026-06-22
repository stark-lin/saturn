// This file tests auditable HTTP request source extraction.
package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCaptureRequestSourceNormalizesLoopback(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "[::1]:8080"
	response := httptest.NewRecorder()

	CaptureRequestSource(nil, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		source := RequestSourceFromContext(r.Context())
		if source.IP != "127.0.0.1" {
			t.Fatalf("source IP = %q, want normalized loopback", source.IP)
		}
	})).ServeHTTP(response, request)
}

func TestCaptureRequestSourceIgnoresForwardedIPFromUntrustedRemote(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "198.51.100.4:8080"
	request.Header.Set("X-Forwarded-For", "203.0.113.8")

	CaptureRequestSource([]string{"10.0.0.0/8"}, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		source := RequestSourceFromContext(r.Context())
		if source.IP != "198.51.100.4" {
			t.Fatalf("source IP = %q, want direct remote IP", source.IP)
		}
	})).ServeHTTP(httptest.NewRecorder(), request)
}

func TestCaptureRequestSourceUsesForwardedIPFromTrustedRemote(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.5:8080"
	request.Header.Set("X-Forwarded-For", "203.0.113.8")
	request.Header.Set("User-Agent", "saturn-test")

	CaptureRequestSource([]string{"10.0.0.0/8"}, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		source := RequestSourceFromContext(r.Context())
		if source.IP != "203.0.113.8" || source.UserAgent != "saturn-test" {
			t.Fatalf("source = %#v, want forwarded IP and user agent", source)
		}
	})).ServeHTTP(httptest.NewRecorder(), request)
}
