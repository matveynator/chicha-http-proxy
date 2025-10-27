package server

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"

	"chicha-proxy/internal/colors"
)

// Config groups listener settings so callers keep flag parsing separate from runtime wiring.
type Config struct {
	HTTPPort  string
	HTTPSPort string
	CertFile  string
	KeyFile   string
	Handler   http.Handler
}

// Start spins up HTTP and optional HTTPS listeners, returning a merged error channel for monitoring.
func Start(cfg Config) <-chan error {
	errs := make(chan error)
	httpErrs := make(chan error, 1)
	httpsErrs := make(chan error, 1)

	go func() {
		addr := ":" + cfg.HTTPPort
		log.Printf("%sHTTP%s listening on %s", colors.Section, colors.Reset, addr)
		srv := &http.Server{Addr: addr, Handler: cfg.Handler}
		httpErrs <- srv.ListenAndServe()
	}()

	enableHTTPS := cfg.CertFile != "" && cfg.KeyFile != ""
	if enableHTTPS {
		go func() {
			addr := ":" + cfg.HTTPSPort
			log.Printf("%sHTTPS%s listening on %s", colors.Section, colors.Reset, addr)
			cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
			if err != nil {
				httpsErrs <- fmt.Errorf("loading TLS material: %w", err)
				return
			}
			srv := &http.Server{Addr: addr, Handler: cfg.Handler, TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}}}
			httpsErrs <- srv.ListenAndServeTLS("", "")
		}()
	} else {
		close(httpsErrs)
	}

	// The aggregator keeps one reporting channel so the caller can select once in main.
	go func() {
		defer close(errs)
		remaining := 1
		if enableHTTPS {
			remaining++
		}
		for remaining > 0 {
			select {
			case err, ok := <-httpErrs:
				if ok {
					errs <- fmt.Errorf("http listener stopped: %w", err)
				}
				httpErrs = nil
				remaining--
			case err, ok := <-httpsErrs:
				if ok {
					errs <- fmt.Errorf("https listener stopped: %w", err)
				}
				httpsErrs = nil
				remaining--
			}
		}
	}()

	return errs
}
