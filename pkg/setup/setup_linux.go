//go:build linux

package setup

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ----- Flag registration -----

// RegisterFlag is split by build tags so non-Linux builds avoid exposing the setup flag.
func RegisterFlag() *bool {
	return flag.Bool("setup", false, "Run interactive service setup for Linux hosts.")
}

// ----- Interactive setup flow -----

// initSystem identifies which init system we should generate service files for.
type initSystem string

const (
	initSystemd initSystem = "systemd"
	initInitd   initSystem = "initd"
)

// setupAnswers groups prompt output so the main setup flow stays readable.
type setupAnswers struct {
	useHTTPS bool
	domain   string
	target   string
}

// RunInteractive gathers setup details, installs the service, and writes runbook details to /etc/info.
func RunInteractive() error {
	answersCh := make(chan setupAnswers, 1)
	promptErrCh := make(chan error, 1)
	initCh := make(chan initDetectResult, 1)

	go func() {
		answers, err := promptAnswers()
		if err != nil {
			promptErrCh <- err
			return
		}
		answersCh <- answers
	}()

	go func() {
		initCh <- detectInitSystem()
	}()

	var answers setupAnswers
	select {
	case err := <-promptErrCh:
		return err
	case answers = <-answersCh:
	}

	initResult := <-initCh
	if initResult.err != nil {
		return initResult.err
	}

	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve binary path: %w", err)
	}

	binaryPath, err = filepath.Abs(binaryPath)
	if err != nil {
		return fmt.Errorf("normalize binary path: %w", err)
	}

	serviceConfig := buildServiceConfig(binaryPath, answers)
	if err := writeServiceFiles(initResult.system, serviceConfig); err != nil {
		return err
	}

	if err := appendInfo(initResult.system, serviceConfig); err != nil {
		return err
	}

	return nil
}

// ----- Prompt helpers -----

// promptAnswers coordinates input prompts so the caller can keep orchestration minimal.
func promptAnswers() (setupAnswers, error) {
	reader := bufio.NewReader(os.Stdin)
	useHTTPS, err := promptYesNo(reader, "Use HTTPS for the public endpoint? (y/n): ")
	if err != nil {
		return setupAnswers{}, err
	}

	domain := ""
	if useHTTPS {
		domain, err = promptNonEmpty(reader, "Enter the domain to secure with Let's Encrypt: ")
		if err != nil {
			return setupAnswers{}, err
		}
	}

	target, err := promptNonEmpty(reader, "Where should requests be forwarded? (example: https://backend.local): ")
	if err != nil {
		return setupAnswers{}, err
	}

	parsedTarget, err := url.Parse(target)
	if err != nil {
		return setupAnswers{}, fmt.Errorf("target URL parse failed: %w", err)
	}
	if parsedTarget.Scheme == "" || parsedTarget.Host == "" {
		return setupAnswers{}, errors.New("target URL must include scheme and host")
	}

	return setupAnswers{useHTTPS: useHTTPS, domain: domain, target: target}, nil
}

// promptYesNo asks for a yes/no answer and keeps prompting until valid input arrives.
func promptYesNo(reader *bufio.Reader, prompt string) (bool, error) {
	for {
		fmt.Print(prompt)
		answer, err := reader.ReadString('\n')
		if err != nil {
			return false, fmt.Errorf("read answer: %w", err)
		}
		answer = strings.TrimSpace(strings.ToLower(answer))
		switch answer {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Println("Please answer with y or n.")
		}
	}
}

// promptNonEmpty ensures we do not accept blank answers for required values.
func promptNonEmpty(reader *bufio.Reader, prompt string) (string, error) {
	for {
		fmt.Print(prompt)
		answer, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("read answer: %w", err)
		}
		answer = strings.TrimSpace(answer)
		if answer != "" {
			return answer, nil
		}
		fmt.Println("Value cannot be empty.")
	}
}

// ----- Init system detection -----

// initDetectResult bundles init detection output for channel use.
type initDetectResult struct {
	system initSystem
	err    error
}

// detectInitSystem inspects common Linux locations to determine the active init system.
func detectInitSystem() initDetectResult {
	if isSystemdAvailable() {
		return initDetectResult{system: initSystemd}
	}
	if isInitdAvailable() {
		return initDetectResult{system: initInitd}
	}
	return initDetectResult{err: errors.New("unable to detect systemd or init.d")}
}

// isSystemdAvailable checks for systemd runtime markers instead of executing systemctl.
func isSystemdAvailable() bool {
	if _, err := os.Stat("/run/systemd/system"); err == nil {
		return true
	}
	if _, err := os.Stat("/bin/systemctl"); err == nil {
		return true
	}
	if _, err := os.Stat("/usr/bin/systemctl"); err == nil {
		return true
	}
	return false
}

// isInitdAvailable checks for legacy init scripts so we can fallback when systemd is absent.
func isInitdAvailable() bool {
	if _, err := os.Stat("/etc/init.d"); err == nil {
		return true
	}
	return false
}

// ----- Service rendering -----

// serviceConfig centralizes install paths so we can render multiple outputs consistently.
type serviceConfig struct {
	binaryPath  string
	useHTTPS    bool
	domain      string
	targetURL   string
	logFilePath string
}

// buildServiceConfig normalizes values and keeps future changes localized.
func buildServiceConfig(binaryPath string, answers setupAnswers) serviceConfig {
	return serviceConfig{
		binaryPath:  binaryPath,
		useHTTPS:    answers.useHTTPS,
		domain:      answers.domain,
		targetURL:   answers.target,
		logFilePath: "/var/log/chicha-http-proxy.log",
	}
}

// writeServiceFiles writes either systemd or init.d artifacts depending on detection.
func writeServiceFiles(system initSystem, cfg serviceConfig) error {
	switch system {
	case initSystemd:
		return writeSystemd(cfg)
	case initInitd:
		return writeInitd(cfg)
	default:
		return fmt.Errorf("unsupported init system: %s", system)
	}
}

// writeSystemd writes the unit file and reloads systemd so the service is discoverable.
func writeSystemd(cfg serviceConfig) error {
	unitPath := "/etc/systemd/system/chicha-http-proxy.service"
	unitContent := renderSystemdUnit(cfg)
	if err := os.WriteFile(unitPath, []byte(unitContent), 0644); err != nil {
		return fmt.Errorf("write systemd unit: %w", err)
	}

	cmd := exec.Command("systemctl", "daemon-reload")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemctl daemon-reload failed: %w", err)
	}

	return nil
}

// renderSystemdUnit generates the systemd unit contents using the resolved binary path.
func renderSystemdUnit(cfg serviceConfig) string {
	execLine := fmt.Sprintf("%s --target-url %s", cfg.binaryPath, cfg.targetURL)
	if cfg.useHTTPS {
		execLine = fmt.Sprintf("%s --domain %s", execLine, cfg.domain)
	}

	return fmt.Sprintf(`[Unit]
Description=Chicha HTTP Proxy
After=network.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`, execLine)
}

// writeInitd writes a SysV init script and marks it executable.
func writeInitd(cfg serviceConfig) error {
	initPath := "/etc/init.d/chicha-http-proxy"
	content := renderInitdScript(cfg)
	if err := os.WriteFile(initPath, []byte(content), 0755); err != nil {
		return fmt.Errorf("write init.d script: %w", err)
	}
	return nil
}

// renderInitdScript returns a basic init script with pidfile and log redirection.
func renderInitdScript(cfg serviceConfig) string {
	execLine := fmt.Sprintf("%s --target-url %s", cfg.binaryPath, cfg.targetURL)
	if cfg.useHTTPS {
		execLine = fmt.Sprintf("%s --domain %s", execLine, cfg.domain)
	}

	return fmt.Sprintf(`#!/bin/sh
#
# /etc/init.d/chicha-http-proxy
#
### BEGIN INIT INFO
# Provides:          chicha-http-proxy
# Required-Start:    $network
# Required-Stop:     $network
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: Chicha HTTP Proxy
### END INIT INFO

DAEMON="%s"
DAEMON_ARGS="%s"
PIDFILE="/var/run/chicha-http-proxy.pid"
LOGFILE="%s"

start() {
	printf "Starting chicha-http-proxy...\n"
	if [ -f "$PIDFILE" ]; then
		printf "Already running.\n"
		return 1
	fi
	nohup $DAEMON $DAEMON_ARGS >> "$LOGFILE" 2>&1 &
	echo $! > "$PIDFILE"
	printf "Started.\n"
}

stop() {
	printf "Stopping chicha-http-proxy...\n"
	if [ ! -f "$PIDFILE" ]; then
		printf "Not running.\n"
		return 1
	fi
	kill "$(cat "$PIDFILE")"
	rm -f "$PIDFILE"
	printf "Stopped.\n"
}

status() {
	if [ -f "$PIDFILE" ]; then
		printf "Running (PID $(cat "$PIDFILE")).\n"
	else
		printf "Stopped.\n"
	fi
}

case "$1" in
	start)
		start
		;;
	stop)
		stop
		;;
	status)
		status
		;;
	restart)
		stop
		start
		;;
	*)
		echo "Usage: $0 {start|stop|status|restart}"
		exit 1
		;;
esac

exit 0
`, cfg.binaryPath, execLine, cfg.logFilePath)
}

// ----- /etc/info updates -----

// appendInfo adds a runbook section to /etc/info with commands tailored to the init system.
func appendInfo(system initSystem, cfg serviceConfig) error {
	infoPath := "/etc/info"
	now := time.Now().Format(time.RFC3339)
	content := renderInfo(system, cfg, now)

	file, err := os.OpenFile(infoPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open /etc/info: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		return fmt.Errorf("write /etc/info: %w", err)
	}
	return nil
}

// renderInfo writes a timestamped block so operators can discover service usage quickly.
func renderInfo(system initSystem, cfg serviceConfig, timestamp string) string {
	var builder strings.Builder
	builder.WriteString("\n")
	builder.WriteString("Chicha HTTP Proxy setup (generated at " + timestamp + ")\n")
	builder.WriteString("Target URL: " + cfg.targetURL + "\n")
	if cfg.useHTTPS {
		builder.WriteString("HTTPS domain: " + cfg.domain + "\n")
	} else {
		builder.WriteString("HTTPS disabled for this setup.\n")
	}
	builder.WriteString("Binary path: " + cfg.binaryPath + "\n")

	switch system {
	case initSystemd:
		builder.WriteString("Start: systemctl start chicha-http-proxy\n")
		builder.WriteString("Stop: systemctl stop chicha-http-proxy\n")
		builder.WriteString("Status: systemctl status chicha-http-proxy\n")
		builder.WriteString("Logs: journalctl -u chicha-http-proxy -f\n")
	case initInitd:
		builder.WriteString("Start: service chicha-http-proxy start\n")
		builder.WriteString("Stop: service chicha-http-proxy stop\n")
		builder.WriteString("Status: service chicha-http-proxy status\n")
		builder.WriteString("Logs: tail -f " + cfg.logFilePath + "\n")
	default:
		builder.WriteString("Unknown init system detected; manual startup required.\n")
	}

	return builder.String()
}
