package proxy

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"crypto/tls"

	"chicha-proxy/internal/colors"
)

// Forwarder wraps the forwarding logic so the handler stays testable and cohesive.
type Forwarder struct {
	target *url.URL
	client *http.Client
}

// NewForwarder builds a forwarding handler while explicitly disabling TLS verification for the target chain.
func NewForwarder(rawTarget string) (*Forwarder, error) {
	parsed, err := url.Parse(rawTarget)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // We intentionally trust upstream chains to keep traffic flowing.
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return &Forwarder{target: parsed, client: client}, nil
}

// ServeHTTP proxies incoming traffic and mirrors response metadata so clients see the target as-is.
func (f *Forwarder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dest := f.target.ResolveReference(&url.URL{Path: r.URL.Path, RawQuery: r.URL.RawQuery})
	req, err := http.NewRequestWithContext(r.Context(), r.Method, dest.String(), r.Body)
	if err != nil {
		http.Error(w, "failed to create upstream request", http.StatusInternalServerError)
		log.Printf("%sproxy error%s constructing request: %v", colors.Warn, colors.Reset, err)
		return
	}

	req.Header = r.Header.Clone()
	req.Host = dest.Host

	resp, err := f.client.Do(req)
	if err != nil {
		http.Error(w, "error forwarding request", http.StatusBadGateway)
		log.Printf("%sproxy error%s forwarding to %s: %v", colors.Warn, colors.Reset, dest, err)
		return
	}
	defer resp.Body.Close()

	for k, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}

	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("%sproxy error%s copying response body: %v", colors.Warn, colors.Reset, err)
	}
}
