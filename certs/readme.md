# Generate a private key for your CA:

```bash
openssl genrsa -out rootCA.key 2048
```

## Create a self-signed root certificate (valid for 10 years):
```bash
openssl req -x509 -new -nodes -key rootCA.key -sha256 -days 3650 -out rootCA.crt
```
* You'll need to fill in some details (Country, Org Name, etc.)
* Common Name could be "My MITM Proxy CA"


## Convert to PEM format (Go prefers this):

```bash
openssl x509 -in rootCA.crt -out rootCA.pem -outform PEM
```

## Verification:
```bash
openssl x509 -in rootCA.pem -text -noout
```



# Installation Steps

## Copy Certificate to System Store
```bash
sudo cp certs/rootCA.pem /usr/share/ca-certificates/mitm-proxy-ca.crt
```
## Update Certificate Trust
```bash
sudo trust anchor --store /usr/share/ca-certificates/mitm-proxy-ca.crt
```
## Verification:
```bash
trust list | grep "mitm-proxy-ca" -A 5
```
* Should show your CA with trusted flag.

## Update curl/Firefox Separately
* curl: Already uses system trust store (no extra steps needed).

* Firefox:
```bash
certutil -A -n "MITM Proxy CA" -t "TCu,Cu,Tu" -i /usr/share/ca-certificates/mitm-proxy-ca.crt -d sql:$HOME/.pki/nssdb
```
bash

## Testing the Installation
```bash
curl --proxy http://localhost:8080 https://example.com
```
* Should now work without -k or --cacert.

# Removal Steps (Complete Reversal)
## Remove System Trust
```bash
sudo trust anchor --remove /usr/share/ca-certificates/mitm-proxy-ca.crt
```
## Delete Certificate File
```bash
sudo rm /usr/share/ca-certificates/mitm-proxy-ca.crt
```
## Remove from Firefox (if added)
```bash
certutil -D -n "MITM Proxy CA" -d sql:$HOME/.pki/nssdb
```
## Clear Cache
```bash
sudo update-ca-trust
```
