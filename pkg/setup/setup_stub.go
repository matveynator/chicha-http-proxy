//go:build !linux

package setup

// RegisterFlag is a no-op on non-Linux platforms so the setup flag stays Linux-only.
func RegisterFlag() *bool {
	return nil
}

// RunInteractive is intentionally unavailable on non-Linux platforms.
func RunInteractive() error {
	return nil
}
