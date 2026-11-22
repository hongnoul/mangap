package network

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"sort"
	"time"
)

// ProbeResult is the outcome of probing one source host.
type ProbeResult struct {
	OK bool
	// Latency is the median time to first byte of a minimal request on an
	// already open connection: one server round trip, with DNS, TCP and
	// TLS setup excluded so hosts are compared fairly.
	Latency time.Duration
	// RTT is the network round trip: the fastest of RTTSamples raw TCP
	// handshakes to the host. Independent of server load or page weight.
	RTT    time.Duration
	Method string
	Status int
	Detail string
}

// ProbeSamples is how many warm requests are timed after the cold one.
const ProbeSamples = 3

// RTTSamples is how many TCP handshakes are timed for RTT.
const RTTSamples = 3

// Probe measures any source host the same way, so builtin and custom Lua
// sources get comparable numbers:
//
//  1. Strip the URL to its origin, so every source gets the same kind of
//     request no matter which page it was configured with.
//  2. Cold HEAD request on a fresh connection: checks the host is up and
//     records the TCP connect time. Hosts that reject HEAD (405/501) fall
//     back to GET with `Range: bytes=0-0`, so neither method downloads a
//     page body.
//  3. RTTSamples raw TCP dials to host:port. RTT is the minimum, the
//     standard way to read network latency without queueing noise.
//  4. ProbeSamples warm requests with the chosen method over the same
//     keep-alive connection. Latency is their median TTFB.
//
// Any status below 500 counts as up: 403/404 still prove the server answers.
func Probe(ctx context.Context, rawURL string) ProbeResult {
	var res ProbeResult
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		res.Detail = "bad url"
		return res
	}
	origin := (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: "/"}).String()

	// Private transport: no connection shared with other probes or the
	// app, and exactly one connection per host so the warm samples reuse it.
	tr := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		ForceAttemptHTTP2:   true,
		MaxIdleConnsPerHost: 1,
		MaxConnsPerHost:     1,
		TLSHandshakeTimeout: 5 * time.Second,
		TLSClientConfig:     &tls.Config{},
	}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr}

	res.Method = http.MethodHead
	t, status, err := timedRequest(ctx, client, res.Method, origin)
	if err == nil && (status == http.StatusMethodNotAllowed || status == http.StatusNotImplemented) {
		res.Method = http.MethodGet
		t, status, err = timedRequest(ctx, client, res.Method, origin)
	}
	if err != nil {
		res.Detail = describeErr(ctx, err)
		return res
	}
	res.Status = status
	res.RTT = tcpRTT(ctx, u)
	if status >= 500 {
		res.Detail = fmt.Sprintf("HTTP %d", status)
		return res
	}
	res.OK = true

	var samples []time.Duration
	for i := 0; i < ProbeSamples; i++ {
		ts, _, err := timedRequest(ctx, client, res.Method, origin)
		if err != nil {
			break
		}
		samples = append(samples, ts.ttfb)
	}
	if len(samples) == 0 {
		// Server closed the connection after one request: fall back to
		// the cold TTFB minus the setup we measured.
		res.Latency = t.ttfb - t.setup
		return res
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	res.Latency = samples[len(samples)/2]
	return res
}

type timing struct {
	setup time.Duration // DNS + TCP + TLS, zero on a reused connection
	ttfb  time.Duration // request start to first response byte
}

func timedRequest(ctx context.Context, c *http.Client, method, target string) (timing, int, error) {
	var (
		t     timing
		start time.Time
	)
	trace := &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			if !info.Reused {
				t.setup = time.Since(start)
			}
		},
		GotFirstResponseByte: func() { t.ttfb = time.Since(start) },
	}
	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), method, target, nil)
	if err != nil {
		return t, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (mangap source check)")
	if method == http.MethodGet {
		req.Header.Set("Range", "bytes=0-0")
	}
	start = time.Now()
	resp, err := c.Do(req)
	if err != nil {
		return t, 0, err
	}
	// Drain so the connection goes back to the pool for the next sample.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	_ = resp.Body.Close()
	return t, resp.StatusCode, nil
}

func describeErr(ctx context.Context, err error) string {
	var dnsErr *net.DNSError
	switch {
	case ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.As(err, &dnsErr):
		return "dns failed"
	default:
		var opErr *net.OpError
		if errors.As(err, &opErr) && opErr.Op == "dial" {
			return "refused"
		}
		return "unreachable"
	}
}

// tcpRTT returns the fastest TCP handshake to u's host, or 0 if none
// succeeded. DNS is resolved once up front so it is not counted.
func tcpRTT(ctx context.Context, u *url.URL) time.Duration {
	port := u.Port()
	if port == "" {
		port = "443"
		if u.Scheme == "http" {
			port = "80"
		}
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, u.Hostname())
	if err != nil || len(ips) == 0 {
		return 0
	}
	addr := net.JoinHostPort(ips[0].IP.String(), port)
	var d net.Dialer
	var best time.Duration
	for i := 0; i < RTTSamples; i++ {
		t0 := time.Now()
		c, err := d.DialContext(ctx, "tcp", addr)
		el := time.Since(t0)
		if err != nil {
			continue
		}
		_ = c.Close()
		if best == 0 || el < best {
			best = el
		}
	}
	return best
}

// PingRTT measures only the network round trip to rawURL's host: the
// fastest of RTTSamples TCP handshakes. No HTTP request is sent. detail
// is a short reason when the host cannot be reached.
func PingRTT(ctx context.Context, rawURL string) (rtt time.Duration, detail string) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return 0, "bad url"
	}
	if _, err := net.DefaultResolver.LookupIPAddr(ctx, u.Hostname()); err != nil {
		return 0, describeErr(ctx, err)
	}
	if rtt = tcpRTT(ctx, u); rtt == 0 {
		if ctx.Err() != nil {
			return 0, "timeout"
		}
		return 0, "unreachable"
	}
	return rtt, ""
}
