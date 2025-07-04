package proxy

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"math/big"
	"net"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	forbiddenHTMLTemplate = `<!DOCTYPE html>
<html>
<head>
    <title>Access Denied</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
        h1 { color: #d9534f; }
    </style>
</head>
<body>
    <h1>403 Forbidden</h1>
    <p>Access to %s has been restricted by the administrator.</p>
</body>
</html>`
)

func (p *Proxy) handleHTTPS(client net.Conn, reader *bufio.Reader) {
	defer client.Close()

	target, domain, err := p.processConnectRequest(reader)
	if err != nil {
		p.Logger.Error("Failed to process CONNECT request", zap.Error(err))
		return
	}

	if shouldBlock := p.checkAndBlockHost(client, domain); shouldBlock {
		return
	}

	if err := p.establishMITMTunnel(client, target, domain); err != nil {
		p.Logger.Error("MITM tunneling failed", zap.Error(err))
	}
}

// Connection Request Handling
func (p *Proxy) processConnectRequest(reader *bufio.Reader) (string, string, error) {
	target, err := p.readConnectRequest(reader)
	if err != nil {
		return "", "", fmt.Errorf("error reading CONNECT request: %w", err)
	}

	// Add default port if missing
	if !strings.Contains(target, ":") {
		target += ":443"
	}

	domain := strings.Split(target, ":")[0]
	return target, domain, nil
}

func (p *Proxy) readConnectRequest(reader *bufio.Reader) (string, error) {
	var request strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		request.WriteString(line)
		if line == "\r\n" {
			break
		}
	}

	firstLine := strings.Split(request.String(), "\r\n")[0]
	parts := strings.Split(strings.TrimSpace(firstLine), " ")
	if len(parts) < 3 {
		return "", fmt.Errorf("malformed CONNECT request")
	}

	return parts[1], nil
}

func (p *Proxy) sendForbiddenResponse(client net.Conn, domain string) {
	htmlContent := fmt.Sprintf(forbiddenHTMLTemplate, domain)
	response := fmt.Sprintf("HTTP/1.1 403 Forbidden\r\n"+
		"Content-Type: text/html; charset=utf-8\r\n"+
		"Content-Length: %d\r\n"+
		"Connection: close\r\n"+
		"\r\n%s", len(htmlContent), htmlContent)

	if _, err := client.Write([]byte(response)); err != nil {
		p.Logger.Error("Failed to send 403 response", zap.Error(err))
	}
}

// MITM Tunneling
func (p *Proxy) establishMITMTunnel(client net.Conn, target, domain string) error {
	if _, err := client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		return fmt.Errorf("failed to send 200 response: %w", err)
	}

	rootCA, err := tls.LoadX509KeyPair("certs/rootCA.pem", "certs/rootCA.key")
	if err != nil {
		return fmt.Errorf("failed to load root CA: %w", err)
	}

	fakeCert, err := p.generateCertificate(domain, &rootCA)
	if err != nil {
		return fmt.Errorf("certificate generation failed: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{fakeCert},
		MinVersion:   tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
		},
		PreferServerCipherSuites: true,
	}

	clientTLS := tls.Server(client, tlsConfig)
	if err := clientTLS.Handshake(); err != nil {
		return fmt.Errorf("TLS handshake failed: %w", err)
	}

	serverTLS, err := tls.Dial("tcp", target, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to target: %w", err)
	}
	defer serverTLS.Close()

	p.tunnelConnections(clientTLS, serverTLS)
	return nil
}

// Certificate Generation
func (p *Proxy) generateCertificate(domain string, rootCA *tls.Certificate) (tls.Certificate, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	cleanDomain := strings.Split(domain, ":")[0]
	h := sha256.New()
	h.Write([]byte(domain))
	h.Write([]byte(time.Now().Format(time.RFC3339Nano)))
	serial := new(big.Int).SetBytes(h.Sum(nil))

	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: cleanDomain,
		},
		DNSNames:              []string{cleanDomain, "*." + cleanDomain},
		NotBefore:             time.Now().Add(-5 * time.Minute),
		NotAfter:              time.Now().Add(2 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
		SubjectKeyId:          []byte{1, 2, 3, 4},
	}

	signedCert, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		rootCA.Leaf,
		&privateKey.PublicKey,
		rootCA.PrivateKey,
	)
	if err != nil {
		return tls.Certificate{}, err
	}

	return tls.Certificate{
		Certificate: [][]byte{signedCert},
		PrivateKey:  privateKey,
	}, nil
}

// Connection Tunneling
func (p *Proxy) tunnelConnections(client, server net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer server.Close()
		io.Copy(server, client)
	}()

	go func() {
		defer wg.Done()
		defer client.Close()
		p.forwardServerResponse(client, server)
	}()

	wg.Wait()
}

func (p *Proxy) forwardServerResponse(client, server net.Conn) {
	serverReader := bufio.NewReader(server)
	headers, err := p.readServerResponse(serverReader)
	if err != nil {
		p.Logger.Error("Failed reading headers", zap.Error(err))
		return
	}

	var bodyBuffer bytes.Buffer
	teeReader := io.TeeReader(serverReader, &bodyBuffer)

	if _, err := client.Write([]byte(headers)); err != nil {
		p.Logger.Error("Failed writing headers", zap.Error(err))
		return
	}

	if _, err := io.Copy(client, teeReader); err != nil {
		p.Logger.Debug("Streaming error", zap.Error(err))
	}

	p.Logger.Info("Full response",
		zap.String("headers", headers),
		// zap.ByteString("body", bodyBuffer.Bytes()),
	)
}

func (p *Proxy) readServerResponse(reader *bufio.Reader) (string, error) {
	var response strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		_, err = response.WriteString(line)
		if err != nil {
			return "", fmt.Errorf("error reading response: %w", err)
		}
		if line == "\r\n" {
			break
		}
	}
	return response.String(), nil
}

