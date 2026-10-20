package main

import (
	"bufio"
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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/terraspin/terraspin/internal/ai"
)

func TestCLIOpenAIAndOllamaPayloadsExcludeAttributeValues(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "terraspin")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = filepath.Join(repoRoot, "cmd", "terraspin")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	t.Run("ollama", func(t *testing.T) {
		bodies := make(chan []byte, 2)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
				return
			}
			bodies <- body
			_, _ = w.Write([]byte(`{"response":"{\"summary\":\"LLM narrative\",\"risk_assessment\":\"low\",\"recommendations\":[],\"rollback_strategy\":\"none\"}"}`))
		}))
		defer server.Close()
		workdir, planA, planB := payloadPlans(t)
		for _, args := range [][]string{{"--llm", "ollama", "--ollama-host", server.URL, planA}, {"diff", "--llm", "ollama", "--ollama-host", server.URL, planA, planB}} {
			runPayloadCommand(t, binary, workdir, os.Environ(), args)
		}
		assertProviderPayloads(t, "ollama", bodies)
	})

	t.Run("openai", func(t *testing.T) {
		proxy := newOpenAIProxy(t)
		workdir, planA, planB := payloadPlans(t)
		if err := os.WriteFile(filepath.Join(workdir, ".terraspin.yml"), []byte("version: 1\nllm:\n  provider: openai\n  timeout: 1s\n  max_retries: 0\n  fallback_to_rules: false\n"), 0644); err != nil {
			t.Fatal(err)
		}
		env := withoutEnvironment(os.Environ(), "OPENAI_API_KEY", "SSL_CERT_FILE", "HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy", "NO_PROXY", "no_proxy")
		env = append(env, "OPENAI_API_KEY=payload-placeholder", "HTTPS_PROXY="+proxy.url, "SSL_CERT_FILE="+proxy.certFile, "NO_PROXY=")
		for _, args := range [][]string{{planA}, {"diff", planA, planB}} {
			runPayloadCommand(t, binary, workdir, env, args)
		}
		assertProviderPayloads(t, "openai", proxy.bodies)
	})
}

func TestCLIPayloadsSanitizeHostileAddresses(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "terraspin")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = filepath.Join(repoRoot, "cmd", "terraspin")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	bodies := make(chan []byte, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		bodies <- body
		_, _ = w.Write([]byte(`{"response":"{\\"summary\\":\\"LLM narrative\\",\\"risk_assessment\\":\\"low\\",\\"recommendations\\":[],\\"rollback_strategy\\":\\"none\\"}"}`))
	}))
	defer server.Close()

	workdir := t.TempDir()
	hostilePlan := filepath.Join(repoRoot, "testdata", "security", "hostile_address_plan.json")
	emptyPlan := filepath.Join(repoRoot, "testdata", "plan_empty.json")
	for _, args := range [][]string{{"--llm", "ollama", "--ollama-host", server.URL, hostilePlan}, {"diff", "--llm", "ollama", "--ollama-host", server.URL, hostilePlan, emptyPlan}} {
		runPayloadCommand(t, binary, workdir, os.Environ(), args)
	}

	for _, command := range []string{"analyze", "diff"} {
		select {
		case body := <-bodies:
			var payload struct {
				Prompt string `json:"prompt"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(payload.Prompt, "HOSTILE_LLM_INJECTION") {
				t.Fatalf("%s payload contains hostile address: %s", command, payload.Prompt)
			}
			if !strings.Contains(payload.Prompt, "<invalid-address>") {
				t.Fatalf("%s payload does not neutralize hostile address: %s", command, payload.Prompt)
			}
			if !strings.Contains(payload.Prompt, `aws_s3_bucket.logs["prod-logs"]`) {
				t.Fatalf("%s payload lost valid address: %s", command, payload.Prompt)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s payload was not captured", command)
		}
	}
}

func payloadPlans(t *testing.T) (string, string, string) {
	t.Helper()
	workdir := t.TempDir()
	planA := filepath.Join(workdir, "a.json")
	planB := filepath.Join(workdir, "b.json")
	for path, values := range map[string][2]string{
		planA: {"SYNTHETIC_SENSITIVE_A", "SYNTHETIC_UNMARKED_A"},
		planB: {"SYNTHETIC_SENSITIVE_B", "SYNTHETIC_UNMARKED_B"},
	} {
		if err := os.WriteFile(path, syntheticPayloadPlan(t, values[0], values[1]), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return workdir, planA, planB
}

func runPayloadCommand(t *testing.T, binary, workdir string, env, args []string) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = workdir
	cmd.Env = env
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run %v: %v\n%s", args, err, output)
	}
}

func assertProviderPayloads(t *testing.T, provider string, bodies <-chan []byte) {
	t.Helper()
	for _, command := range []string{"analyze", "diff"} {
		select {
		case body := <-bodies:
			if strings.Contains(string(body), "SYNTHETIC_") {
				t.Fatalf("%s %s payload contains an attribute value: %s", provider, command, body)
			}
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatal(err)
			}
			if provider == "ollama" {
				if _, ok := payload["system"]; ok {
					t.Fatalf("%s %s unexpectedly has a system field", provider, command)
				}
			} else {
				messages, ok := payload["messages"].([]any)
				if !ok || len(messages) < 2 {
					t.Fatalf("%s %s messages=%v, want system and user messages", provider, command, payload["messages"])
				}
				first, _ := messages[0].(map[string]any)
				if first["role"] != "system" || first["content"] != ai.SystemPrompt {
					t.Fatalf("%s %s system message changed or missing", provider, command)
				}
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s %s payload was not captured", provider, command)
		}
	}
}

func withoutEnvironment(env []string, names ...string) []string {
	filtered := make([]string, 0, len(env))
	for _, value := range env {
		name, _, _ := strings.Cut(value, "=")
		keep := true
		for _, excluded := range names {
			if name == excluded {
				keep = false
				break
			}
		}
		if keep {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

type openAIProxy struct {
	url      string
	certFile string
	bodies   chan []byte
	listener net.Listener
	tls      *tls.Config
}

func newOpenAIProxy(t *testing.T) *openAIProxy {
	t.Helper()
	certPEM, certificate := openAICertificate(t)
	certFile := filepath.Join(t.TempDir(), "openai-proxy.pem")
	if err := os.WriteFile(certFile, certPEM, 0600); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	proxy := &openAIProxy{
		url:      "http://" + listener.Addr().String(),
		certFile: certFile,
		bodies:   make(chan []byte, 2),
		listener: listener,
		tls:      &tls.Config{Certificates: []tls.Certificate{certificate}},
	}
	go proxy.serve()
	t.Cleanup(func() { _ = listener.Close() })
	return proxy
}

func openAICertificate(t *testing.T) ([]byte, tls.Certificate) {
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
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return certPEM, certificate
}

func (p *openAIProxy) serve() {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			return
		}
		go p.handle(conn)
	}
}

func (p *openAIProxy) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	reader := bufio.NewReader(conn)
	request, err := http.ReadRequest(reader)
	if err != nil || request.Method != http.MethodConnect {
		return
	}
	_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
	tlsConn := tls.Server(&bufferedConn{Conn: conn, reader: reader}, p.tls)
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
	p.bodies <- body
	response := []byte(`{"choices":[{"message":{"content":"{\"summary\":\"LLM narrative\",\"risk_assessment\":\"low\",\"recommendations\":[],\"rollback_strategy\":\"none\"}"}}]}`)
	_, _ = fmt.Fprintf(tlsConn, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(response), response)
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.reader.Read(p) }
