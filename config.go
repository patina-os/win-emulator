// SPDX-License-Identifier: GPL-3.0-or-later

// Command TryPatina runs patina in a virtual machine on a Windows PC: it sets up stock
// QEMU, downloads the released patina image, lets its installer put patina on a
// virtual disk, and starts that disk from then on.
package main

import "time"

// What the launcher uses, in one place. patina's side of the contract is
// devices/try-patina-windows/ in patina-os/patina: it matches smbiosProduct.
const (
	appName      = "Try patina"
	patinaRepo   = "patina-os/patina"
	launcherRepo = "patina-os/win-emulator"
	isoName      = "patina-amd64.hybrid.iso"
	sumsName     = "SHA256SUMS"
	// The ready-made disk (iso/disk/ in patina) and the installer's pages for the setup
	// window, both release assets beside the image; the first boot reads the answers
	// under setupFwCfg (devices/try-patina-windows/firstboot/ in patina).
	diskImageName  = "patina-amd64.qcow2"
	setupPagesName = "patina-setup.json"
	setupFwCfg     = "opt/io.github.patina_os/setup"
	qemuWingetID   = "SoftwareFreedomConservancy.QEMU"
	smbiosProduct  = "Try patina"

	// The virtual disk grows as the guest writes; this is only its ceiling.
	diskSize = "128G"
	// The installer writes several GB; a disk smaller than this after the installer
	// restarted the machine holds no system (the user restarted the live system).
	minInstalledDisk = 2 << 30
	// The smallest machine, in MB, that patina's installer accepts: it wants 95% of
	// 3 GiB as the kernel's total memory (requiredRam in patina's Calamares
	// welcome.conf), and firmware and kernel keep about 170 MB of the machine's, so a
	// machine of 3072 MB fails (2906 MB) and one of 3328 MB passes (3158 MB).
	installerMemoryMB = 3328
	// Free space the first setup asks for: the image, Debian with Plasma and the
	// default apps (docs/vm-run.md in patina names 40 GB for a test VM).
	minFreeSpace = 40 << 30
)

// Stock QEMU on the Windows Hypervisor Platform hangs when its machine protocol is
// connected to in the guest's first seconds, and when the guest powers off
// (docs/FINDINGS.md of omacom/try-omarchy-windows): the launcher waits this long before
// it connects, and ends a QEMU that is still there this long after the guest shut down.
const (
	qmpDelay      = 10 * time.Second
	shutdownGrace = 15 * time.Second
)

// version is set at build time: -ldflags "-X main.version=v0.1.0".
var version = "dev"
