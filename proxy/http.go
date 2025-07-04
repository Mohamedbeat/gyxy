package proxy

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"go.uber.org/zap"
)

func (p *Proxy) handleHTTP(client net.Conn, reader *bufio.Reader) {
	defer client.Close()
	client.SetDeadline(time.Now().Add(30 * time.Second))

	req, err := p.parseRequest(reader)
	if err != nil {
		p.Logger.Error("Error parsing HTTP request", zap.Error(err))
		return
	}
	if shouldBlock := p.checkAndBlockHost(client, req.Host); shouldBlock {
		return
	}

	p.Logger.Info("HTTP request",
		zap.String("method", req.Method),
		zap.String("host", req.Host),
		zap.String("path", req.Path),
		zap.String("clientLocalAddr", client.LocalAddr().String()),
		zap.String("clientRemoteAddr", client.RemoteAddr().String()))

	// Connect to target
	target := net.JoinHostPort(req.Host, req.Port)
	server, err := net.Dial("tcp", target)
	if err != nil {
		p.Logger.Error("Error connecting to target", zap.Error(err))
		return
	}
	defer server.Close()

	// Forward request
	if _, err := server.Write([]byte(req.Raw)); err != nil {
		p.Logger.Error("Error forwarding request", zap.Error(err))
		return
	}

	// Forward response
	if err := p.forwardResponse(client, server); err != nil {
		p.Logger.Error("Error forwarding response", zap.Error(err))
	}
}

func (p *Proxy) parseRequest(reader *bufio.Reader) (*HTTPRequest, error) {
	req := &HTTPRequest{Port: "80"}
	var raw strings.Builder

	// Read request line
	firstLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	raw.WriteString(firstLine)

	parts := strings.Split(strings.TrimSpace(firstLine), " ")
	if len(parts) < 3 {
		return nil, fmt.Errorf("malformed request line")
	}

	req.Method = parts[0]
	req.Path = parts[1]
	req.Protocol = parts[2]

	// Read headers
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		raw.WriteString(line)

		if line == "\r\n" {
			break
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Host":
			hostParts := strings.Split(value, ":")
			req.Host = hostParts[0]
			if len(hostParts) > 1 {
				req.Port = hostParts[1]
			}
		case "User-Agent":
			req.UserAgent = value
		case "Proxy-Connection":
			req.ProxyConnection = value
		}
	}

	req.Raw = raw.String()
	return req, nil
}

func (p *Proxy) forwardResponse(client net.Conn, server net.Conn) error {
	resp, err := p.parseResponse(bufio.NewReader(server))
	if err != nil {
		return err
	}

	p.Logger.Info("HTTP response",
		zap.String("proto", resp.Proto),
		zap.Int("status", resp.StatusCode))

	_, err = client.Write(resp.RawResponse)
	return err
}
