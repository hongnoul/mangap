package network

import (
	"net/http"
	"time"
)

var transport = http.DefaultTransport.(*http.Transport).Clone()

func init() {
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 100
	transport.MaxConnsPerHost = 200
	transport.IdleConnTimeout = 90 * time.Second // outlives the TUI keep-warm interval
	transport.ResponseHeaderTimeout = 30 * time.Second
	transport.ExpectContinueTimeout = 30 * time.Second
}

var Client = &http.Client{
	Timeout:   time.Minute,
	Transport: transport,
}
