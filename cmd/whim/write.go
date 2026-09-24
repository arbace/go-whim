package main

import "os"

// writeFile writes data over path, keeping the file's permissions (0644 for a
// new one) -- what every subcommand that rewrites a file in place does.
func writeFile(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	return os.WriteFile(path, data, mode)
}
