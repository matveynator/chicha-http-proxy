package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"chicha-proxy/internal/cli"
	"chicha-proxy/internal/colors"
	"chicha-proxy/internal/proxy"
	"chicha-proxy/internal/server"
)

// version is patched during builds; we keep it global so release tooling can replace it easily.
var version = "dev"

func main() {
	// Plain log formatting keeps the colored prefixes tidy.
	log.SetFlags(0)
	log.SetPrefix(colors.Section + "[proxy] " + colors.Reset)

	// Wire our custom usage printer before parsing so -h always shows the stylized guidance.
	cli.ConfigureUsage(flag.CommandLine)

	httpPort := flag.String("http-port", "80", "Port that accepts inbound HTTP traffic.")
	httpsPort := flag.String("https-port", "443", "Port for HTTPS when certificates are supplied.")
	targetURL := flag.String("target-url", "", "Destination URL that will receive forwarded requests.")
	tlsCert := flag.String("tls-cert", "", "Path to a PEM encoded certificate for HTTPS listeners.")
	tlsKey := flag.String("tls-key", "", "Path to a PEM encoded private key for HTTPS listeners.")
	showVersion := flag.Bool("version", false, "Print the application version and exit.")

	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	if *targetURL == "" {
		log.Fatal(colors.Warn + "target-url must be set so the proxy knows where to send traffic" + colors.Reset)
	}

	forwarder, err := proxy.NewForwarder(*targetURL)
	if err != nil {
		log.Fatalf("%sfailed to parse target-url%s %v", colors.Warn, colors.Reset, err)
	}

	log.Printf("%sForwarding%s requests to %s while ignoring target TLS validation errors.", colors.Accent, colors.Reset, *targetURL)

	cfg := server.Config{
		HTTPPort:  *httpPort,
		HTTPSPort: *httpsPort,
		CertFile:  *tlsCert,
		KeyFile:   *tlsKey,
		Handler:   forwarder,
	}

	errs := server.Start(cfg)

	// Block on the merged error stream so the process only exits when a listener stops.
	if err := <-errs; err != nil {
		log.Printf("%sproxy stopped%s %v", colors.Warn, colors.Reset, err)
		os.Exit(1)
	}
}
