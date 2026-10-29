// SPDX-License-Identifier: GPL-3.0-or-later

// Command TryPatina runs patina in a virtual machine on a Windows PC: it sets up stock
// QEMU, downloads the released patina image, lets its installer put patina on a
// virtual disk, and starts that disk from then on.
package main

// What the launcher uses, in one place. patina's side of the contract is
// devices/try-patina-windows/ in patina-os/patina: it matches smbiosProduct.
const (
	appName       = "Try patina"
	patinaRepo    = "patina-os/patina"
	launcherRepo  = "patina-os/win-emulator"
	isoName       = "patina-amd64.hybrid.iso"
	sumsName      = "SHA256SUMS"
	qemuWingetID  = "SoftwareFreedomConservancy.QEMU"
	smbiosProduct = "Try patina"

	// The virtual disk grows as the guest writes; this is only its ceiling.
	diskSize = "128G"
	// The installer writes several GB; a disk smaller than this after the installer
	// restarted the machine holds no system (the user restarted the live system).
	minInstalledDisk = 2 << 30
	// Free space the first setup asks for: the image, Debian with Plasma and the
	// default apps (docs/vm-run.md in patina names 40 GB for a test VM).
	minFreeSpace = 40 << 30
)

// version is set at build time: -ldflags "-X main.version=v0.1.0".
var version = "dev"
