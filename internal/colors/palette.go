package colors

// This package centralizes terminal styling so we can adjust contrasts once for all outputs.
const (
	Reset   = "\033[0m"
	Title   = "\033[1;34m" // Bold blue stays readable on both light and dark terminal backgrounds.
	Section = "\033[1;32m" // Bold green is gentle enough for day mode yet visible at night.
	Accent  = "\033[36m"   // Cyan draws attention without overwhelming the surrounding text.
	Example = "\033[1;36m" // Bright cyan is reused for examples to keep cues consistent.
	Warn    = "\033[1;33m" // Warm yellow hints at caution and still contrasts on white terminals.
)
