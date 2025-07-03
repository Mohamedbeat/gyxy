package proxy

import "go.uber.org/zap"

type HTTPRequest struct {
	Method          string
	Path            string
	Protocol        string
	Port            string
	Host            string
	UserAgent       string
	ProxyConnection string
	Raw             string
}

type HTTPResponse struct {
	Proto       string
	StatusCode  int
	Status      string
	Headers     map[string]string
	Body        []byte
	RawResponse []byte
}

type Proxy struct {
	Logger *zap.Logger
}
