package render

import (
	"io"
	"os"

	"golang.org/x/term"
)

// useColor decides whether to emit ANSI color: never when noColor is set, never
// for non-terminal output, never when the NO_COLOR env var is present.
func useColor(noColor, isTTY bool) bool {
	if noColor || !isTTY {
		return false
	}
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	return true
}

// isTerminal reports whether w is a terminal file descriptor.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// managerPalette is a small ANSI 256-color set; managers map into it by hash so
// the same manager is consistently colored within a run.
var managerPalette = []int{36, 33, 32, 35, 34, 31, 96, 93}

func colorizeManager(name string, on bool) string {
	if !on || name == "" {
		return name
	}
	var h uint32 = 2166136261
	for i := 0; i < len(name); i++ {
		h ^= uint32(name[i])
		h *= 16777619
	}
	code := managerPalette[int(h)%len(managerPalette)]
	return "\x1b[" + itoa(code) + "m" + name + "\x1b[0m"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [4]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
