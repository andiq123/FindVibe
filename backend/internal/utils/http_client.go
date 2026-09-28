package utils

import (
	"crypto/tls"
	"net/http"
	"time"
)

// NewHTTPClient is the shared client for Last.fm / covers / lyrics (honors HTTP(S)_PROXY).
func NewHTTPClient(timeout time.Duration, maxIdleConns, maxIdlePerHost int, idleTimeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
			MaxIdleConns:          maxIdleConns,
			MaxIdleConnsPerHost:   maxIdlePerHost,
			IdleConnTimeout:       idleTimeout,
			ResponseHeaderTimeout: 8 * time.Second,
			ForceAttemptHTTP2:     true,
		},
	}
}
