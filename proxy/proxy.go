package proxy

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

func New(logger *zap.Logger) *Proxy {
	return &Proxy{Logger: logger}
}

// Start runs the proxy server
func (p *Proxy) Start(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	p.Logger.Info("Proxy server started", zap.String("addr", addr))

	for {
		conn, err := listener.Accept()
		if err != nil {
			p.Logger.Error("Error accepting connection", zap.Error(err))
			continue
		}

		go p.handleConnection(conn)
	}
}

// handleConnection routes the connection to appropriate handler
func (p *Proxy) handleConnection(client net.Conn) {
	defer client.Close()
	client.SetDeadline(time.Now().Add(30 * time.Second))

	reader := bufio.NewReader(client)
	peek, err := reader.Peek(7)
	if err != nil {
		p.Logger.Error("Error peeking connection", zap.Error(err))
		return
	}

	if string(peek) == "CONNECT" {
		p.handleHTTPS(client, reader)
	} else {
		p.handleHTTP(client, reader)
	}
}

// parseResponse parses an HTTP response from the server
func (p *Proxy) parseResponse(reader *bufio.Reader) (*HTTPResponse, error) {
	resp := &HTTPResponse{Headers: make(map[string]string)}
	var buf bytes.Buffer

	// Status line
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	buf.WriteString(statusLine)

	parts := strings.Split(strings.TrimSpace(statusLine), " ")
	if len(parts) < 2 {
		return nil, fmt.Errorf("malformed status line")
	}

	resp.Proto = parts[0]
	resp.StatusCode, _ = strconv.Atoi(parts[1])
	if len(parts) > 2 {
		resp.Status = strings.Join(parts[2:], " ")
	}

	// Headers
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		buf.WriteString(line)

		if line == "\r\n" {
			break
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			resp.Headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	// Body (if Content-Length exists)
	if cl, ok := resp.Headers["Content-Length"]; ok {
		length, _ := strconv.Atoi(cl)
		if length > 0 {
			body := make([]byte, length)
			if _, err := io.ReadFull(reader, body); err != nil {
				return nil, err
			}
			resp.Body = body
			buf.Write(body)
		}
	}

	resp.RawResponse = buf.Bytes()
	return resp, nil
}

// Host Checking
func (p *Proxy) checkHost(host string, clientAddr string) (bool, error) {
	file, err := os.Open("blocked")
	if err != nil {
		return false, fmt.Errorf("failed to open blocked file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		blockedHost := strings.TrimSpace(scanner.Text())
		if blockedHost == "" {
			continue
		}
		fmt.Printf("target: %s, unwanted %s\n", host, blockedHost)
		if strings.EqualFold(host, blockedHost) {
			p.Logger.Warn("Blocked host accessed",
				zap.String("host", host),
				zap.String("client", clientAddr))
			return false, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("error reading blocked file: %w", err)
	}
	return true, nil
}

// Host Blocking Logic
func (p *Proxy) checkAndBlockHost(client net.Conn, domain string) bool {
	ok, err := p.checkHost(domain, client.RemoteAddr().String())
	if err != nil {
		p.Logger.Error("Host check failed", zap.Error(err))
		return false
	}

	if !ok {
		p.sendForbiddenResponse(client, domain)
		return true
	}
	return false
}
