// SPDX-License-Identifier: GPL-3.0-or-later

package main

import "embed"

// patina's small icon, as branding/export/ in patina-os/patina draws it; build.sh
// copies it here, so this repository carries no copy of the mark.
//
//go:embed assets/icon-small-32.png assets/icon-small-64.png assets/icon-small-256.png
var assets embed.FS

func appIcon() ([]byte, error) {
	var pngs [][]byte
	for _, size := range []string{"32", "64", "256"} {
		b, err := assets.ReadFile("assets/icon-small-" + size + ".png")
		if err != nil {
			return nil, err
		}
		pngs = append(pngs, b)
	}
	return icoFromPNGs(pngs...)
}
