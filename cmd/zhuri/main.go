// Command zhuri finds files that are supposed to be UTF-8 but were actually
// saved in Windows-1252/ISO-8859-1 (or UTF-16), and rewrites them as valid
// UTF-8, showing exactly which bytes were broken.
package main

import "os"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
