package ai

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestProviderContract(t *testing.T) {
	cases := []struct {
		name     string
		needsKey bool
		keyEnv   string
		query    func(*testing.T, Provider)
	}{
		{"claude", true, "ANTHROPIC_API_KEY", queryContractClaude},
		{"ollama", false, "", queryContractOllama},
		{"openai", true, "OPENAI_API_KEY", queryContractOpenAI},
	}

	wantNames := []string{"claude", "ollama", "openai"}
	if got := Names(); !reflect.DeepEqual(got, wantNames) {
		t.Fatalf("Names() = %v, want %v", got, wantNames)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider, ok := Lookup(tc.name)
			if !ok {
				t.Fatalf("Lookup(%q) = false", tc.name)
			}
			if provider.Name() != tc.name || provider.NeedsKey() != tc.needsKey || provider.KeyEnv() != tc.keyEnv {
				t.Fatalf("provider = (%q, %t, %q), want (%q, %t, %q)", provider.Name(), provider.NeedsKey(), provider.KeyEnv(), tc.name, tc.needsKey, tc.keyEnv)
			}
			tc.query(t, provider)
		})
	}
}

func queryContractClaude(t *testing.T, provider Provider) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("x-api-key") != "claude-key" || r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Fatalf("unexpected Claude request: method=%s headers=%v", r.Method, r.Header)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		want, _ := json.Marshal(claudeRequest{Model: "claude-model", MaxTokens: 1024, System: SystemPrompt, Messages: []claudeMessage{{Role: "user", Content: "prompt"}}})
		if !bytes.Equal(body, want) {
			t.Fatalf("Claude body = %s, want %s", body, want)
		}
		_, _ = w.Write([]byte(`{"content":[{"text":"claude response"}],"usage":{"input_tokens":2,"output_tokens":3}}`))
	}))
	defer server.Close()
	t.Setenv("ANTHROPIC_BASE_URL", server.URL)

	response, err := provider.Query(context.Background(), Request{Model: "claude-model", System: SystemPrompt, Prompt: "prompt", APIKey: "claude-key"})
	if err != nil || response != (Response{Text: "claude response", Tokens: 5}) {
		t.Fatalf("Query() = %#v, %v", response, err)
	}
}

func queryContractOllama(t *testing.T, provider Provider) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/generate" || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Authorization") != "" {
			t.Fatalf("unexpected Ollama request: method=%s path=%s headers=%v", r.Method, r.URL.Path, r.Header)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		want, _ := json.Marshal(ollamaRequest{Model: "ollama-model", Prompt: "prompt", Stream: false})
		if !bytes.Equal(body, want) {
			t.Fatalf("Ollama body = %s, want %s", body, want)
		}
		_, _ = w.Write([]byte(`{"response":"ollama response","eval_count":8}`))
	}))
	defer server.Close()

	response, err := provider.Query(context.Background(), Request{Model: "ollama-model", System: "ignored", Prompt: "prompt", Extra: map[string]string{"ollama_host": server.URL}})
	if err != nil || response != (Response{Text: "ollama response", Tokens: 8}) {
		t.Fatalf("Query() = %#v, %v", response, err)
	}
}

func queryContractOpenAI(t *testing.T, provider Provider) {
	t.Helper()
	proxy := newContractOpenAIProxy(t)
	t.Setenv("HTTPS_PROXY", proxy.url)
	t.Setenv("https_proxy", proxy.url)
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("http_proxy", "")
	t.Setenv("ALL_PROXY", "")
	t.Setenv("all_proxy", "")
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")
	t.Setenv("SSL_CERT_FILE", proxy.certFile)
	proxyURL, err := url.Parse(proxy.url)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(proxy.certPEM) {
		t.Fatal("add OpenAI proxy certificate")
	}
	oldTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL), TLSClientConfig: &tls.Config{RootCAs: roots}}
	t.Cleanup(func() { http.DefaultClient.Transport = oldTransport })

	response, err := provider.Query(context.Background(), Request{Model: "openai-model", System: SystemPrompt, Prompt: "prompt", APIKey: "openai-key"})
	if err != nil || response != (Response{Text: "openai response", Tokens: 7}) {
		t.Fatalf("Query() = %#v, %v", response, err)
	}
	request := <-proxy.requests
	if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/json" || request.Header.Get("Authorization") != "Bearer openai-key" {
		t.Fatalf("unexpected OpenAI request: method=%s headers=%v", request.Method, request.Header)
	}
	want, _ := json.Marshal(openAIRequest{Model: "openai-model", MaxTokens: 2048, Messages: []openAIMessage{{Role: "system", Content: SystemPrompt}, {Role: "user", Content: "prompt"}}})
	if !bytes.Equal(request.body, want) {
		t.Fatalf("OpenAI body = %s, want %s", request.body, want)
	}
}

func TestProviderContractKeyRequirementsAndUnknownProvider(t *testing.T) {
	for _, tc := range []struct {
		provider string
		keyEnv   string
	}{
		{"claude", "ANTHROPIC_API_KEY"},
		{"openai", "OPENAI_API_KEY"},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			var requests int
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
			defer server.Close()
			t.Setenv("ANTHROPIC_BASE_URL", server.URL)
			_, _, err := Execute(context.Background(), LLMOptions{Provider: tc.provider, Timeout: time.Second}, "prompt")
			if err == nil || !strings.Contains(err.Error(), tc.keyEnv) || requests != 0 {
				t.Fatalf("err=%v requests=%d, want missing %s with no request", err, requests, tc.keyEnv)
			}
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"response":"ok"}`))
	}))
	defer server.Close()
	if _, _, err := Execute(context.Background(), LLMOptions{Provider: "ollama", Extra: map[string]string{"ollama_host": server.URL}, Timeout: time.Second}, "prompt"); err != nil {
		t.Fatalf("Ollama without key: %v", err)
	}

	_, _, err := Execute(context.Background(), LLMOptions{Provider: "unknown", Timeout: time.Second}, "prompt")
	if err == nil || !strings.Contains(err.Error(), strings.Join(Names(), ", ")) {
		t.Fatalf("unknown provider error = %v, want registered names", err)
	}
}

type contractOpenAIProxy struct {
	url      string
	certFile string
	certPEM  []byte
	listener net.Listener
	tls      *tls.Config
	requests chan contractCapturedRequest
}

type contractCapturedRequest struct {
	Method string
	Header http.Header
	body   []byte
}

func newContractOpenAIProxy(t *testing.T) *contractOpenAIProxy {
	t.Helper()
	certPEM, certificate := contractOpenAICertificate(t)
	certFile := filepath.Join(t.TempDir(), "openai-proxy.pem")
	if err := os.WriteFile(certFile, certPEM, 0600); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	proxy := &contractOpenAIProxy{
		url:      "http://" + listener.Addr().String(),
		certFile: certFile,
		certPEM:  certPEM,
		listener: listener,
		tls:      &tls.Config{Certificates: []tls.Certificate{certificate}},
		requests: make(chan contractCapturedRequest, 1),
	}
	go proxy.serve()
	t.Cleanup(func() { _ = listener.Close() })
	return proxy
}

func contractOpenAICertificate(t *testing.T) ([]byte, tls.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		DNSNames:              []string{"api.openai.com"},
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := tls.X509KeyPair(certPEM, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}))
	if err != nil {
		t.Fatal(err)
	}
	return certPEM, certificate
}

func (p *contractOpenAIProxy) serve() {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			return
		}
		go p.handle(conn)
	}
}

func (p *contractOpenAIProxy) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	reader := bufio.NewReader(conn)
	request, err := http.ReadRequest(reader)
	if err != nil || request.Method != http.MethodConnect {
		return
	}
	_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
	tlsConn := tls.Server(&contractBufferedConn{Conn: conn, reader: reader}, p.tls)
	if err := tlsConn.Handshake(); err != nil {
		return
	}
	request, err = http.ReadRequest(bufio.NewReader(tlsConn))
	if err != nil {
		return
	}
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return
	}
	p.requests <- contractCapturedRequest{Method: request.Method, Header: request.Header, body: body}
	response := []byte(`{"choices":[{"message":{"content":"openai response"}}],"usage":{"total_tokens":7}}`)
	_, _ = fmt.Fprintf(tlsConn, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(response), response)
}

type contractBufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *contractBufferedConn) Read(p []byte) (int, error) { return c.reader.Read(p) }
