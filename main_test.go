package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var buildOnce sync.Once
var builtBinaryPath string
var buildErr error

func buildBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "ts-release-bin-*")
		if err != nil {
			buildErr = err
			return
		}
		bin := filepath.Join(dir, "ts-release-testbin")
		cmd := exec.Command("go", "build", "-o", bin, ".")
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		if err != nil {
			buildErr = fmt.Errorf("go build failed: %w: %s", err, string(out))
			return
		}
		builtBinaryPath = bin
	})
	if buildErr != nil {
		t.Fatalf("build binary: %v", buildErr)
	}
	return builtBinaryPath
}

func runCmd(t *testing.T, bin string, args ...string) (exitCode int, stdout string, stderr string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("command timed out: %v\nstdout: %s\nstderr: %s", ctx.Err(), stdout, stderr)
	}

	if err == nil {
		return 0, stdout, stderr
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("unexpected run error: %v\nstderr: %s", err, stderr)
	}
	return ee.ExitCode(), stdout, stderr
}

func mustJPEGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("jpeg encode: %v", err)
	}
	return buf.Bytes()
}

type mitmProxy struct {
	ln       net.Listener
	caPEM    []byte
	leafCert tls.Certificate
	imgBytes []byte
}

func newMITMProxy(t *testing.T) *mitmProxy {
	t.Helper()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate ca key: %v", err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "ts-release test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create ca cert: %v", err)
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})

	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "wallhaven.cc"},
		DNSNames:     []string{"wallhaven.cc"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse ca cert: %v", err)
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create leaf cert: %v", err)
	}
	leafPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	leafKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(leafKey)})
	leafCert, err := tls.X509KeyPair(leafPEM, leafKeyPEM)
	if err != nil {
		t.Fatalf("load leaf keypair: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen proxy: %v", err)
	}

	p := &mitmProxy{ln: ln, caPEM: caPEM, leafCert: leafCert, imgBytes: mustJPEGBytes(t)}
	go p.serve()
	return p
}

func (p *mitmProxy) close() { _ = p.ln.Close() }

func (p *mitmProxy) serve() {
	for {
		conn, err := p.ln.Accept()
		if err != nil {
			return
		}
		go p.handleConn(conn)
	}
}

func (p *mitmProxy) handleConn(conn net.Conn) {
	defer conn.Close()

	br := bufio.NewReader(conn)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	if req.Method != http.MethodConnect {
		_, _ = io.WriteString(conn, "HTTP/1.1 405 Method Not Allowed\r\nConnection: close\r\n\r\n")
		return
	}

	_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")

	tlsConn := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{p.leafCert}})
	if err := tlsConn.Handshake(); err != nil {
		return
	}
	defer tlsConn.Close()

	reader := bufio.NewReader(tlsConn)
	r, err := http.ReadRequest(reader)
	if err != nil {
		return
	}
	p.respond(tlsConn, r)
}

func (p *mitmProxy) respond(w io.Writer, r *http.Request) {
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/v1/search") {
		body := `{"data":[{"path":"https://wallhaven.cc/img"}]}`
		fmt.Fprintf(w, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
		return
	}
	if path == "/img" {
		fmt.Fprintf(w, "HTTP/1.1 200 OK\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\nConnection: close\r\n\r\n", len(p.imgBytes))
		_, _ = w.Write(p.imgBytes)
		return
	}

	body := "not found"
	fmt.Fprintf(w, "HTTP/1.1 404 Not Found\r\nContent-Type: text/plain\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
}

func TestMain_MissingArgs_UsageAndErrorExit(t *testing.T) {
	bin := buildBinary(t)
	code, _, stderr := runCmd(t, bin)
	if code == 0 {
		t.Fatalf("expected non-zero exit")
	}
	if !strings.Contains(stderr, "Usage: ts-release") {
		t.Fatalf("expected usage in stderr, got: %q", stderr)
	}
}

func TestMain_NonExistingRootFS_UsageAndErrorExit(t *testing.T) {
	bin := buildBinary(t)
	code, _, stderr := runCmd(t, bin, "target", filepath.Join(t.TempDir(), "missing"))
	if code == 0 {
		t.Fatalf("expected non-zero exit")
	}
	if !strings.Contains(stderr, "Usage: ts-release") {
		t.Fatalf("expected usage in stderr, got: %q", stderr)
	}
}

func TestMain_Help_PrintsUsageAndExits(t *testing.T) {
	bin := buildBinary(t)
	code, _, stderr := runCmd(t, bin, "--help")
	if code == 0 {
		t.Fatalf("expected non-zero exit (current behavior)")
	}
	if !strings.Contains(stderr, "Usage: ts-release") {
		t.Fatalf("expected usage in stderr, got: %q", stderr)
	}
}

func TestMain_Success_ValidInput_NoRealNetwork(t *testing.T) {
	bin := buildBinary(t)
	rootFS := t.TempDir()

	proxy := newMITMProxy(t)
	defer proxy.close()

	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, proxy.caPEM, 0o644); err != nil {
		t.Fatalf("write ca file: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "target", rootFS)
	cmd.Env = append(os.Environ(),
		"HTTPS_PROXY=http://"+proxy.ln.Addr().String(),
		"HTTP_PROXY=http://"+proxy.ln.Addr().String(),
		"NO_PROXY=",
		"SSL_CERT_FILE="+caFile,
	)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("command timed out\nstdout: %s\nstderr: %s", outBuf.String(), errBuf.String())
	}
	if err != nil {
		t.Fatalf("expected success, got error: %v\nstdout: %s\nstderr: %s", err, outBuf.String(), errBuf.String())
	}

	paths := []string{
		filepath.Join(rootFS, "boot", "splash.bmp"),
		filepath.Join(rootFS, "usr", "share", "backgrounds", "tssh", "background.jpg"),
		filepath.Join(rootFS, "etc", "tssh.build"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected output file %s to exist: %v", p, err)
		}
	}
}
