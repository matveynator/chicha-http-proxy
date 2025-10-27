package main

import (
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"golang.org/x/crypto/acme/autocert"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// Program version (will be printed if the --version flag is used)
var version = "dev"

// proxyHandler returns an HTTP handler function that forwards incoming requests
// to a specified target URL (reverse proxy functionality).
func proxyHandler(targetURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Attempt to read the request body (if present)
		var body []byte
		if r.Body != nil {
			var err error
			body, err = io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Failed to read request body", http.StatusInternalServerError)
				log.Printf("Error reading request body: %v", err)
				return
			}
		}

		// Construct the initial forwarding URL by combining the target URL with the requested path
		originalURL := targetURL + r.URL.Path
		currentURL := originalURL

		// Create an HTTP client for making outgoing requests to the target server.
		// We skip certificate verification because the proxy is meant to trust the upstream blindly.
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}

		for {
			// Create a new outgoing request using the incoming request's method, headers, and body.
			req, err := http.NewRequest(r.Method, currentURL, bytes.NewReader(body))
			if err != nil {
				http.Error(w, "Failed to create request", http.StatusInternalServerError)
				log.Printf("Error creating request: %v", err)
				return
			}

			// Copy all headers from the incoming request to the outgoing request.
			for header, values := range r.Header {
				for _, value := range values {
					req.Header.Add(header, value)
				}
			}

			// Preserve the query string parameters
			req.URL.RawQuery = r.URL.RawQuery

			// Perform the HTTP request to the target server
			resp, err := client.Do(req)
			if err != nil {
				http.Error(w, "Error forwarding request", http.StatusBadGateway)
				log.Printf("Error forwarding request: %v", err)
				return
			}
			defer resp.Body.Close()

			// If the response is a redirect (3xx), follow it
			if resp.StatusCode >= 300 && resp.StatusCode < 400 {
				location, err := resp.Location()
				if err != nil {
					http.Error(w, "Failed to handle redirect", http.StatusInternalServerError)
					log.Printf("Error handling redirect: %v", err)
					return
				}
				currentURL = location.String()
				log.Printf("Redirecting to: %s", currentURL)
				continue
			}

			// Copy the response headers from the target server to the client
			for header, values := range resp.Header {
				for _, value := range values {
					w.Header().Add(header, value)
				}
			}

			// Set the status code in the client response
			w.WriteHeader(resp.StatusCode)

			// Copy the response body
			responseBody, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("Error reading response body: %v", err)
				return
			}
			_, err = w.Write(responseBody)
			if err != nil {
				log.Printf("Error writing response body: %v", err)
			}
			return
		}
	}
}

func main() {
	// Color palette keeps help output readable on both dark and light backgrounds.
	const (
		colorReset       = "\033[0m"
		colorTitle       = "\033[38;5;39m"
		colorSection     = "\033[38;5;69m"
		colorHighlight   = "\033[38;5;208m"
		colorDescription = "\033[38;5;244m"
	)

	// Custom usage function keeps the CLI friendly and shows minimal and advanced recipes.
	flag.Usage = func() {
		fmt.Printf("%sChicha HTTP Proxy%s\n", colorTitle, colorReset)
		fmt.Printf("%sMinimalist reverse proxy with optional automatic TLS.%s\n\n", colorDescription, colorReset)

		fmt.Printf("%sUsage%s\n", colorSection, colorReset)
		fmt.Printf("  chicha-http-proxy [flags]\n\n")

		fmt.Printf("%sFlags%s\n", colorSection, colorReset)
		flag.VisitAll(func(f *flag.Flag) {
			fmt.Printf("  %s--%s%s %s%s%s\n",
				colorHighlight, f.Name, colorReset,
				colorDescription, f.Usage, colorReset,
			)
		})
		fmt.Println()

		fmt.Printf("%sQuick Start%s\n", colorSection, colorReset)
		fmt.Printf("  %sMinimal:%s chicha-http-proxy --target-url https://example.com\n", colorHighlight, colorReset)
		fmt.Printf("  %sAdvanced:%s chicha-http-proxy --target-url https://example.com --http-port 8080 --domain proxy.example.com --https-port 8443\n", colorHighlight, colorReset)
		fmt.Println()

		fmt.Printf("%sNotes%s\n", colorSection, colorReset)
		fmt.Printf("  %s• Certificates fetched automatically when --domain is set.%s\n", colorDescription, colorReset)
		fmt.Printf("  %s• Upstream TLS verification is disabled intentionally for trust-on-first-use scenarios.%s\n\n", colorDescription, colorReset)
	}

	// Define command-line flags
	httpPort := flag.String("http-port", "80", "Port for the HTTP server. If -domain is set, this is forced to 80.")
	httpsPort := flag.String("https-port", "443", "Port for the HTTPS server (only used if -domain is set).")
	targetURL := flag.String("target-url", "https://twochicks.ru", "Target URL for forwarding requests.")
	domain := flag.String("domain", "", "Domain for automatic Let's Encrypt certificate. Forces HTTP port to 80 and admin rights, HTTPS can be changed.")
	showVersion := flag.Bool("version", false, "Show program version")

	// Parse the flags
	flag.Parse()

	// If --version is specified, print the program version and exit
	if *showVersion {
		fmt.Printf("Program version: %s\n", version)
		os.Exit(0)
	}

	// The target URL must be specified.
	if *targetURL == "" {
		log.Fatal("Target URL (--target-url) is not specified")
	}

	// If a domain is provided for certificate retrieval:
	// - Force HTTP port to 80 (required for Let's Encrypt HTTP challenge).
	// - Allow user to specify HTTPS port (default 443), if desired.
	if *domain != "" {
		*httpPort = "80"
		log.Printf("Domain specified. HTTP port forced to 80. HTTPS port: %s", *httpsPort)
	} else {
		// If no domain is specified:
		// - The user can use any HTTP port they like.
		// - No HTTPS will be started as no certificate is requested.
		fmt.Printf("No domain specified. Running HTTP on port %s only.\n", *httpPort)
	}

	// Create the proxy handler
	handler := proxyHandler(*targetURL)

	// 'done' channel is used to keep the main goroutine running.
	done := make(chan bool)

	// Start HTTP server. If a domain is given, this will always be on port 80.
	// If no domain is given, this uses the user-specified port.
	if *httpPort != "" {
		go func() {
			httpServer := &http.Server{
				Addr:    ":" + *httpPort,
				Handler: handler,
			}
			log.Printf("Starting HTTP proxy on port %s targeting %s", *httpPort, *targetURL)
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("HTTP server error: %v", err)
			}
		}()
	}

	// If a domain is specified, set up HTTPS with Let's Encrypt on the specified port.
	if *domain != "" {
		// Obtain the user's home directory to store certificates.
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to get user home directory: %v", err)
		}

		// Setup the directory to store TLS certificates.
		certDir := filepath.Join(homeDir, ".chicha-http-proxy-ssl-certs")
		if err := os.MkdirAll(certDir, 0700); err != nil {
			log.Fatalf("Failed to create cert directory: %v", err)
		}

		go func() {
			m := &autocert.Manager{
				Cache:      autocert.DirCache(certDir),
				Prompt:     autocert.AcceptTOS,
				HostPolicy: autocert.HostWhitelist(*domain),
			}

			httpsServer := &http.Server{
				Addr:      ":" + *httpsPort,
				TLSConfig: m.TLSConfig(),
				Handler:   handler,
			}

			log.Printf("Starting HTTPS proxy on domain %s and port %s targeting %s", *domain, *httpsPort, *targetURL)
			if err := httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				log.Fatalf("HTTPS server error: %v", err)
			}
		}()
	}

	// Block until something signals the 'done' channel.
	<-done
}
