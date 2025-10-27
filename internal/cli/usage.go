package cli

import (
	"flag"
	"fmt"
	"strings"

	"chicha-proxy/internal/colors"
)

// ConfigureUsage injects a custom Usage printer so every flag set renders readable guidance.
func ConfigureUsage(fs *flag.FlagSet) {
	fs.Usage = func() {
		fmt.Printf("%sChicha HTTP Proxy%s\n", colors.Title, colors.Reset)
		fmt.Printf("%sStreamlined reverse proxy that ignores upstream TLS hiccups for smooth chaining.%s\n\n", colors.Accent, colors.Reset)

		fmt.Printf("%sUsage:%s\n", colors.Section, colors.Reset)
		fmt.Println("  chicha-http-proxy [flags]\n")

		fmt.Printf("%sFlags:%s\n", colors.Section, colors.Reset)
		padding := 0
		fs.VisitAll(func(f *flag.Flag) {
			if len(f.Name) > padding {
				padding = len(f.Name)
			}
		})
		fs.VisitAll(func(f *flag.Flag) {
			name := fmt.Sprintf("-%s", f.Name)
			fmt.Printf("  %s%-*s%s  %s%s%s (default %q)\n",
				colors.Title,
				padding+1,
				name,
				colors.Reset,
				colors.Accent,
				strings.TrimSpace(f.Usage),
				colors.Reset,
				f.DefValue,
			)
		})

		fmt.Printf("\n%sQuick start:%s\n", colors.Section, colors.Reset)
		fmt.Printf("  %sMinimal:%s  chicha-http-proxy --target-url https://internal.service\n", colors.Example, colors.Reset)
		fmt.Printf("  %sExtended:%s chicha-http-proxy --target-url https://internal.service --http-port 8080 --https-port 8443 --tls-cert server.crt --tls-key server.key\n", colors.Example, colors.Reset)

		fmt.Printf("\n%sNotes:%s\n", colors.Section, colors.Reset)
		fmt.Printf("  %sThe proxy always ignores TLS verification errors from the target service so chained certificates never block traffic.%s\n", colors.Warn, colors.Reset)
		fmt.Printf("  %sHTTPS support requires certificates you manage; supply --tls-cert and --tls-key if you need encryption at the edge.%s\n", colors.Warn, colors.Reset)
	}
}
