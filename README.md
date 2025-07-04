# gyxy - Go MITM Proxy

## Overview

gyxy is a high-performance HTTP/HTTPS proxy server written in Go. It features MITM (Man-In-The-Middle) capabilities for HTTPS inspection, domain blocking, and colorful structured logging. The proxy is designed for network analysis, parental control, or research purposes.

## Features
- **HTTP/HTTPS Proxy**: Handles both HTTP and HTTPS traffic.
- **MITM Inspection**: Dynamically generates certificates to inspect HTTPS traffic.
- **Domain Blocking**: Blocks access to domains listed in the `blocked` file, returning a 403 Forbidden page.
- **Colorful Logging**: Uses structured, colorized logs for better visibility.

## Getting Started

### Prerequisites
- Go 1.24+
- OpenSSL (for certificate generation)

### Installation
1. **Clone the repository:**
   ```bash
   git clone https://github.com/mohamedbeat/gyxy.git
   cd gyxy
   ```
2. **Build the proxy:**
   ```bash
   go build -o gyxy .
   ```

### Certificate Generation
To inspect HTTPS traffic, you must generate a root CA certificate and install it as trusted on your system and browsers.

#### 1. Generate a private key for your CA:
```bash
openssl genrsa -out certs/rootCA.key 2048
```

#### 2. Create a self-signed root certificate (valid for 10 years):
```bash
openssl req -x509 -new -nodes -key certs/rootCA.key -sha256 -days 3650 -out certs/rootCA.crt
```
* Fill in the details as prompted. Common Name can be "My MITM Proxy CA".

#### 3. Convert to PEM format (Go prefers this):
```bash
openssl x509 -in certs/rootCA.crt -out certs/rootCA.pem -outform PEM
```

#### 4. Verify the certificate:
```bash
openssl x509 -in certs/rootCA.pem -text -noout
```

#### 5. Install the certificate as trusted (Linux example):
```bash
sudo cp certs/rootCA.pem /usr/share/ca-certificates/mitm-proxy-ca.crt
sudo trust anchor --store /usr/share/ca-certificates/mitm-proxy-ca.crt
```
* For Firefox:
```bash
certutil -A -n "MITM Proxy CA" -t "TCu,Cu,Tu" -i /usr/share/ca-certificates/mitm-proxy-ca.crt -d sql:$HOME/.pki/nssdb
```

#### 6. Test the proxy:
```bash
curl --proxy http://localhost:8080 https://example.com
```

#### 7. To remove the certificate:
```bash
sudo trust anchor --remove /usr/share/ca-certificates/mitm-proxy-ca.crt
sudo rm /usr/share/ca-certificates/mitm-proxy-ca.crt
certutil -D -n "MITM Proxy CA" -d sql:$HOME/.pki/nssdb
sudo update-ca-trust
```

For more details, see `certs/readme.md`.

## Usage

1. **Start the proxy:**
   ```bash
   ./gyxy
   ```
   By default, the proxy listens on `:8080`.

2. **Configure your browser or system to use `localhost:8080` as the HTTP/HTTPS proxy.**

3. **Blocking domains:**
   - Add domains (one per line) to the `blocked` file to block them. Example:
     ```
     example.com
     www.tiktok.com
     ```

## Configuration
- **Blocked domains:** Edit the `blocked` file in the project root.
- **Certificates:** Place your generated `rootCA.key` and `rootCA.pem` in the `certs/` directory.
- **Port:** The proxy listens on port 8080 by default. To change, edit `main.go` or add a flag (future improvement).

## Logging
- Logs are colorized and printed to stdout using [uber-go/zap](https://github.com/uber-go/zap) and [fatih/color](https://github.com/fatih/color).

## License
MIT License. See [LICENSE](LICENSE) for details. 