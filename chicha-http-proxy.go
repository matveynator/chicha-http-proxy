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

// ANSI color codes are kept in one place so styling changes remain easy to tweak.
const (
	colorReset   = "\033[0m"
	colorTitle   = "\033[95m"
	colorSection = "\033[96m"
	colorText    = "\033[97m"
	colorExample = "\033[92m"
	colorWarn    = "\033[93m"
)

// Program version (will be printed if the --version flag is used)
var version = "dev"

// configureUsage sets a colorful usage message so operators can quickly discover how to run the proxy.
func configureUsage() {
	flag.Usage = func() {
		fmt.Printf("%sChicha HTTP Proxy%s\n", colorTitle, colorReset)
		fmt.Printf("%sTransparent reverse proxy with optional automatic TLS.%s\n\n", colorText, colorReset)

		fmt.Printf("%sUsage:%s\n", colorSection, colorReset)
		fmt.Printf("  chicha-http-proxy [flags]\n\n")

		fmt.Printf("%sFlags:%s\n", colorSection, colorReset)
		flag.VisitAll(func(f *flag.Flag) {
			fmt.Printf("  %s-%s%s %s(default: %q)%s\n",
				colorTitle, f.Name, colorReset,
				colorText, f.DefValue, colorReset,
			)
			fmt.Printf("    %s%s%s\n", colorText, f.Usage, colorReset)
		})

		fmt.Printf("\n%sQuick start:%s\n", colorSection, colorReset)
		fmt.Printf("  %sMinimal:%s chicha-http-proxy --target-url https://example.com\n", colorExample, colorReset)
		fmt.Printf("  %sAdvanced:%s chicha-http-proxy --target-url https://example.com --domain proxy.example.com --https-port 8443\n", colorExample, colorReset)

		fmt.Printf("\n%sNotes:%s\n", colorSection, colorReset)
		fmt.Printf("  %sThis proxy skips target TLS verification because intermediate TLS errors are not relevant to chained setups.%s\n", colorWarn, colorReset)
	}
}

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
				log.Printf("%sError%s reading request body: %v", colorWarn, colorReset, err)
				return
			}
		}

		// Construct the initial forwarding URL by combining the target URL with the requested path
		originalURL := targetURL + r.URL.Path
		currentURL := originalURL

		// Create an HTTP client for making outgoing requests to the target server.
		// TLS verification is disabled intentionally because this proxy may sit in front of
		// services that use custom or short-lived certificates, and we prefer smooth traffic flow.
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}

		for {
			// Create a new outgoing request using the incoming request's method, headers, and body.
			req, err := http.NewRequest(r.Method, currentURL, bytes.NewReader(body))
			if err != nil {
				http.Error(w, "Failed to create request", http.StatusInternalServerError)
				log.Printf("%sError%s creating request: %v", colorWarn, colorReset, err)
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
				log.Printf("%sError%s forwarding request: %v", colorWarn, colorReset, err)
				return
			}
			defer resp.Body.Close()

			// If the response is a redirect (3xx), follow it
			if resp.StatusCode >= 300 && resp.StatusCode < 400 {
				location, err := resp.Location()
				if err != nil {
					http.Error(w, "Failed to handle redirect", http.StatusInternalServerError)
					log.Printf("%sError%s handling redirect: %v", colorWarn, colorReset, err)
					return
				}
				currentURL = location.String()
				log.Printf("%sRedirect%s Following redirect to %s", colorSection, colorReset, currentURL)
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
				log.Printf("%sError%s reading response body: %v", colorWarn, colorReset, err)
				return
			}
			_, err = w.Write(responseBody)
			if err != nil {
				log.Printf("%sError%s writing response body: %v", colorWarn, colorReset, err)
			}
			return
		}
	}
}

func main() {
	// Colorful logging helps operators read console output faster when juggling multiple proxies.
	log.SetFlags(0)
	log.SetPrefix(colorSection + "[proxy] " + colorReset)

	// Enable the custom usage renderer before parsing flags so users always see the styled help.
	configureUsage()

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
		log.Fatal(colorWarn + "Target URL (--target-url) is not specified" + colorReset)
	}

	// If a domain is provided for certificate retrieval:
	// - Force HTTP port to 80 (required for Let's Encrypt HTTP challenge).
	// - Allow user to specify HTTPS port (default 443), if desired.
	if *domain != "" {
		*httpPort = "80"
		log.Printf("%sDomain%s HTTP port forced to 80. HTTPS port: %s", colorSection, colorReset, *httpsPort)
	} else {
		// If no domain is specified:
		// - The user can use any HTTP port they like.
		// - No HTTPS will be started as no certificate is requested.
		fmt.Printf("%sNo domain specified.%s Running HTTP on port %s only.\n", colorWarn, colorReset, *httpPort)
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
			log.Printf("%sHTTP%s Starting HTTP proxy on port %s targeting %s", colorTitle, colorReset, *httpPort, *targetURL)
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("%sError%s HTTP server error: %v", colorWarn, colorReset, err)
			}
		}()
	}

	// If a domain is specified, set up HTTPS with Let's Encrypt on the specified port.
	if *domain != "" {
		// Obtain the user's home directory to store certificates.
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("%sError%s Failed to get user home directory: %v", colorWarn, colorReset, err)
		}

		// Setup the directory to store TLS certificates.
		certDir := filepath.Join(homeDir, ".chicha-http-proxy-ssl-certs")
		if err := os.MkdirAll(certDir, 0700); err != nil {
			log.Fatalf("%sError%s Failed to create cert directory: %v", colorWarn, colorReset, err)
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

			log.Printf("%sHTTPS%s Starting HTTPS proxy on domain %s and port %s targeting %s", colorTitle, colorReset, *domain, *httpsPort, *targetURL)
			if err := httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				log.Fatalf("%sError%s HTTPS server error: %v", colorWarn, colorReset, err)
			}
		}()
	}

	// Block until something signals the 'done' channel.
	<-done
}
