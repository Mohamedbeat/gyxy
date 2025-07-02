package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mohamedbeat/gyxy/logger" // Assuming this is a valid logger setup
	"go.uber.org/zap"
)

type HTTPObject struct {
	Method          string
	Path            string
	Protocol        string
	Port            string
	Host            string
	UserAgent       string
	ProxyConnection string
	raw             string
}

// ParseRawHTTP parses HTTP headers and returns the HTTP object
func ParseRawHTTP(reader *bufio.Reader) (*HTTPObject, error) {
	httpObj := &HTTPObject{Port: "80"} // Default to HTTP port
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

	httpObj.Method = parts[0]
	httpObj.Path = parts[1]
	httpObj.Protocol = parts[2]

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
			break // End of headers
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue // Skip malformed headers
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Host":
			// Handle host:port
			hostParts := strings.Split(value, ":")
			httpObj.Host = hostParts[0]
			if len(hostParts) > 1 {
				httpObj.Port = hostParts[1]
			}
		case "User-Agent":
			httpObj.UserAgent = value
		case "Proxy-Connection":
			httpObj.ProxyConnection = value
		}
	}

	httpObj.raw = raw.String()
	return httpObj, nil
}

func main() {
	logg, err := logger.InitLogger()
	if err != nil {
		log.Fatal("Error initializing logger:", err)
	}
	defer logg.Sync()

	logg.Info("Starting proxy server on :8080")

	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		logg.Sugar().Panicln("Error starting listener:", err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			logg.Sugar().Errorln("Error accepting connection:", err)
			continue
		}

		go handleConnection(conn, logg)
	}
}

func handleConnection(client net.Conn, logg *zap.Logger) {
	defer client.Close()
	client.SetDeadline(time.Now().Add(30 * time.Second))

	reader := bufio.NewReader(client)

	// Peek to determine if this is HTTPS CONNECT
	peek, err := reader.Peek(7)
	if err != nil {
		logg.Sugar().Errorln("Error peeking connection:", err)
		return
	}

	if string(peek) == "CONNECT" {
		handleHTTPS(client, reader, logg)
	} else {
		handleHTTP(client, reader, logg)
	}
}

func handleHTTP(client net.Conn, reader *bufio.Reader, logg *zap.Logger) {
	httpObj, err := ParseRawHTTP(reader)
	if err != nil {
		logg.Sugar().Errorln("Error parsing HTTP request:", err)
		return
	}

	// Use Infof for formatted strings with zap's Sugar()
	logg.Sugar().Infof("HTTP %s %s%s", httpObj.Method, httpObj.Host, httpObj.Path)

	targetAddr := net.JoinHostPort(httpObj.Host, httpObj.Port)
	server, err := net.Dial("tcp", targetAddr)
	if err != nil {
		logg.Sugar().Errorln("Error connecting to target:", err)
		return
	}
	defer server.Close()
	server.SetDeadline(time.Now().Add(30 * time.Second))

	// Forward request
	if _, err := server.Write([]byte(httpObj.raw)); err != nil {
		logg.Sugar().Errorln("Error forwarding request:", err)
		return
	}

	// Forward response
	resp, err := parseHTTPResponse(bufio.NewReader(server))
	if err != nil {
		logg.Sugar().Errorln("Error parsing response:", err)
		return
	}

	logg.Sugar().Infof("Response: %s %d %s", resp.Proto, resp.StatusCode, resp.Status)
	if _, err := client.Write(resp.RawResponse); err != nil {
		logg.Sugar().Errorln("Error forwarding response:", err)
	}
}

func handleHTTPS(client net.Conn, reader *bufio.Reader, logg *zap.Logger) {
	// Read the entire CONNECT request including headers
	var connectRequest strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			logg.Sugar().Errorln("Error reading CONNECT request:", err)
			return
		}
		connectRequest.WriteString(line)
		if line == "\r\n" { // End of headers
			break
		}
	}

	// Parse target from first line
	firstLine := strings.Split(connectRequest.String(), "\r\n")[0]
	parts := strings.Split(strings.TrimSpace(firstLine), " ")
	if len(parts) < 3 {
		logg.Error("Malformed CONNECT request")
		return
	}

	target := parts[1]
	if !strings.Contains(target, ":") {
		target += ":443" // Default HTTPS port
	}

	logg.Sugar().Infof("HTTPS CONNECT to: %s", target)

	// Establish raw TCP connection to target
	server, err := net.Dial("tcp", target)
	if err != nil {
		logg.Sugar().Errorf("Error connecting to target: %v", err)
		client.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	// Ensure the server connection is closed when handleHTTPS exits.
	defer server.Close()

	// Send 200 Connection Established response back to the client
	if _, err := client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		logg.Sugar().Errorf("Error sending 200 response: %v", err)
		return
	}

	// Handle the TLS handshake on the server side
	// tlsConn := tls.Client(server, &tls.Config{
	// 	InsecureSkipVerify: true, // For testing only - DO NOT USE IN PRODUCTION
	// 	ServerName:         strings.Split(target, ":")[0],
	// })
	//
	// Perform TLS handshake immediately
	// if err := tlsConn.Handshake(); err != nil {
	// 	logg.Sugar().Errorf("TLS handshake failed: %v", err)
	// 	return
	// }

	// Set up bidirectional tunneling

	// At this point, the proxy has established a raw TCP tunnel.
	// The client will now initiate its TLS handshake directly with the target server
	// through this tunnel. The proxy just forwards raw bytes.

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: Copy data from client to server
	go func() {
		defer wg.Done()
		// io.Copy will automatically read any buffered data from 'reader' first,
		// then continue reading directly from its underlying client connection.

		// if _, err := io.Copy(tlsConn, reader); err != nil {
		// 	logg.Sugar().Debugf("Client->Server error: %v", err)

		if _, err := io.Copy(server, reader); err != nil {
			logg.Sugar().Debugf("Client->Server tunnel error: %v", err)
		}
		// It's good practice to close the write-half of the server connection
		// when the client side of the tunnel closes. This signals EOF to the server.
		if tcpConn, ok := server.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()

	// Goroutine 2: Copy data from server to client
	go func() {
		defer wg.Done()
		if _, err := io.Copy(client, server); err != nil {
			logg.Sugar().Debugf("Server->Client tunnel error: %v", err)
		}
		// Similarly, close the write-half of the client connection.
		if tcpConn, ok := client.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()

	// Wait for both copying operations to complete.
	wg.Wait()
}

type HTTPResponse struct {
	Proto       string
	StatusCode  int
	Status      string
	Headers     map[string]string
	Body        []byte
	RawResponse []byte
}

func parseHTTPResponse(reader *bufio.Reader) (*HTTPResponse, error) {
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
			// If EOF is encountered before \r\n, it means the response ended abruptly
			// This can happen for responses without a body or if the connection is closed.
			// We should still return what we have.
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
	// Note: This parsing logic for body is simplistic and doesn't handle
	// Transfer-Encoding: chunked or other complexities.
	// For a general-purpose proxy, it's often better to just stream the body.
	if cl, ok := resp.Headers["Content-Length"]; ok {
		length, err := strconv.Atoi(cl)
		if err != nil {
			// Malformed Content-Length, proceed without reading body
			log.Printf("Warning: Malformed Content-Length header: %s", cl)
		} else if length > 0 {
			body := make([]byte, length)
			if _, err := io.ReadFull(reader, body); err != nil {
				// If we can't read the full body, it's an error
				return nil, fmt.Errorf("error reading response body: %w", err)
			}
			resp.Body = body
			buf.Write(body)
		}
	} else if te, ok := resp.Headers["Transfer-Encoding"]; ok && strings.Contains(te, "chunked") {
		// This simple parser does not handle chunked encoding.
		// For a real proxy, you'd need to stream chunked data.
		// For now, we'll just capture what's in the buffer up to this point.
		log.Printf("Warning: Transfer-Encoding: chunked not fully handled by parser.")
	}

	resp.RawResponse = buf.Bytes()
	return resp, nil
}

