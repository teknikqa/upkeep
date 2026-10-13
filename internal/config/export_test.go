package config

import "io"

// SetWarnOutput redirects Load's warnings for a test and returns a restore func.
func SetWarnOutput(w io.Writer) (restore func()) {
	orig := warnOutput
	warnOutput = w
	return func() { warnOutput = orig }
}
