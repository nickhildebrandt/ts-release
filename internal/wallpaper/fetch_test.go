package wallpaper

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type rewriteTransport struct {
	base       http.RoundTripper
	rewriteURL *url.URL
}

// RoundTrip rewrites requests to wallhaven.cc to the local test server without changing path/query.
// This enables real http.Get calls in the code while avoiding external network access.
func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "wallhaven.cc" {
		clone := req.Clone(req.Context())
		clone.URL.Scheme = t.rewriteURL.Scheme
		clone.URL.Host = t.rewriteURL.Host
		// Keep path/query as-is.
		return t.base.RoundTrip(clone)
	}
	return t.base.RoundTrip(req)
}

// mustPNGBytes produces a small valid PNG byte slice for mocked HTTP responses.
// The test fails fast if PNG encoding unexpectedly fails.
func mustPNGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}

// withHTTPRedirectToServer temporarily replaces http.DefaultTransport to redirect wallhaven.cc to an httptest.Server.
// The original transport is restored via t.Cleanup to avoid cross-test interference.
func withHTTPRedirectToServer(t *testing.T, serverURL string) {
	t.Helper()
	u, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	old := http.DefaultTransport
	if old == nil {
		old = http.DefaultTransport
	}
	http.DefaultTransport = &rewriteTransport{base: old, rewriteURL: u}

	t.Cleanup(func() {
		http.DefaultTransport = old
	})
}

// TestFetchBackground_Success_MockedHTTP verifies the happy path: search returns an image URL and the image is decoded.
// The test fails if rewriting, JSON handling, or image decoding does not behave as expected.
func TestFetchBackground_Success_MockedHTTP(t *testing.T) {
	pngBytes := mustPNGBytes(t)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/search"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"path":"` + server.URL + `/img"}]}`))
			return
		case r.URL.Path == "/img":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngBytes)
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	withHTTPRedirectToServer(t, server.URL)

	img, err := FetchBackground(1920, 1080)
	if err != nil {
		t.Fatalf("FetchBackground error: %v", err)
	}
	if img == nil {
		t.Fatalf("expected non-nil image")
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		t.Fatalf("expected decoded image with positive bounds, got %v", b)
	}
}

// TestFetchBackground_NoResults_Error expects an error when the search API returns no image data.
// It also checks that no image is returned and the error message describes the case.
func TestFetchBackground_NoResults_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	withHTTPRedirectToServer(t, server.URL)

	img, err := FetchBackground(1920, 1080)
	if err == nil {
		t.Fatalf("expected error")
	}
	if img != nil {
		t.Fatalf("expected nil image on error")
	}
	if !strings.Contains(err.Error(), "no usable image") {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}

// TestFetchBackground_MalformedJSON_Error expects an error when the search API returns malformed JSON.
// This ensures decode errors are propagated cleanly.
func TestFetchBackground_MalformedJSON_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{this is not json`))
	}))
	defer server.Close()

	withHTTPRedirectToServer(t, server.URL)

	_, err := FetchBackground(1920, 1080)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "decode search failed") {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}

// TestFetchBackground_ImageDecodeFails_Error expects an error when the image URL returns non-decodable image bytes.
// This verifies that image.Decode failures are reported correctly in the fetch path.
func TestFetchBackground_ImageDecodeFails_Error(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/search"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"path":"` + server.URL + `/img"}]}`))
			return
		case r.URL.Path == "/img":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("not-an-image"))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	withHTTPRedirectToServer(t, server.URL)

	_, err := FetchBackground(1920, 1080)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "decode failed") {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}

// TestFetchBackground_InvalidSize_Error expects an error for invalid target dimensions.
// This prevents pointless requests and documents the validation behavior.
func TestFetchBackground_InvalidSize_Error(t *testing.T) {
	_, err := FetchBackground(0, 1080)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "invalid target size") {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}
