// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows

package main

import (
	"fmt"
	"os"
)

// On Linux patina needs no launcher: install it (github.com/patina-os/patina). The
// rest of this package builds here so its tests run anywhere.
func main() {
	fmt.Fprintln(os.Stderr, "TryPatina runs on Windows; on Linux, install patina itself: https://github.com/patina-os/patina")
	os.Exit(1)
}
