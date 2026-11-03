// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Machine is one start of the virtual machine: what QEMU gets on its command line.
type Machine struct {
	QEMUDir  string // holds qemu-system-x86_64w.exe and share/
	Version  string // QEMU's version, such as 11.1.0, from the VERSION file in QEMUDir
	DataDir  string // holds the disk, the UEFI variables and the logs
	MemoryMB int
	CPUs     int
	Accel    string // whpx, whpx,kernel-irqchip=off or tcg
	ISO      string // set for the installer's run: patina's image as a CD
	QMP      string // host:port of the QMP socket the launcher talks to
	NoSound  bool   // no sound device: DirectSound found none on the host
	// Setup is the file of the setup's answers, handed to the guest as QEMU firmware
	// configuration (setupFwCfg) until it has set itself up; empty after.
	Setup     string
	SerialLog string // where the guest's first serial port writes, empty for nowhere
}

// The accelerators tried in order: the Windows Hypervisor Platform, then the same
// without its in-kernel interrupt controller (hosts that refuse nested virtualization
// fail the first, docs/FINDINGS.md of omacom/try-omarchy-windows), then emulation.
const (
	accelWHPX         = "whpx"
	accelWHPXNoIRQ    = "whpx,kernel-irqchip=off"
	accelEmulation    = "tcg"
	cpuWHPX           = "qemu64,+ssse3,+sse4.1,+sse4.2,+popcnt,+aes"
	cpuEmulation      = "max"
	firmwareCode      = "edk2-x86_64-code.fd"
	firmwareVarsBlank = "edk2-i386-vars.fd" // QEMU pairs the x86_64 code with these
)

// The GTK window shares the Windows clipboard from QEMU 11.1, and only when asked to:
// before, the Windows builds had no clipboard in that window at all.
const clipboardMajor, clipboardMinor = 11, 1

// versionAtLeast is true when a QEMU version such as 11.1.0 is major.minor or later.
// An unknown version is not.
func versionAtLeast(v string, major, minor int) bool {
	var a, b int
	if _, err := fmt.Sscanf(v, "%d.%d", &a, &b); err != nil {
		return false
	}
	return a > major || a == major && b >= minor
}

// qemuPath escapes a path for a QEMU option value, where a comma separates options
// and two commas are one comma.
func qemuPath(p string) string {
	return strings.ReplaceAll(p, ",", ",,")
}

// Args is QEMU's command line, without the program. The window is QEMU's own GTK
// window; closing it does nothing, so patina is shut down from inside or from the tray
// and never loses its disk to a closed window.
func (m Machine) Args() []string {
	cpu := cpuWHPX
	if m.Accel == accelEmulation {
		cpu = cpuEmulation
	}
	a := []string{
		"-name", appName, // the window title
		"-machine", "q35",
		"-accel", m.Accel,
		"-cpu", cpu,
		"-smp", fmt.Sprint(m.CPUs),
		"-m", fmt.Sprintf("%dM", m.MemoryMB),
		// devices/try-patina-windows/device.yaml in patina detects this product name.
		"-smbios", "type=1,product=" + smbiosProduct,
		"-drive", "if=pflash,format=raw,unit=0,readonly=on,file=" +
			qemuPath(filepath.Join(m.QEMUDir, "share", firmwareCode)),
		"-drive", "if=pflash,format=raw,unit=1,file=" + qemuPath(filepath.Join(m.DataDir, varsFile)),
		// NVMe rather than virtio: patina's I/O scheduler rules match nvme*, as on a laptop.
		"-drive", "if=none,id=disk,format=qcow2,discard=unmap,detect-zeroes=unmap,file=" +
			qemuPath(filepath.Join(m.DataDir, diskFile)),
		"-device", "nvme,drive=disk,serial=patina,bootindex=1",
	}
	if m.ISO != "" {
		a = append(a,
			"-drive", "if=none,id=cd,media=cdrom,readonly=on,file="+qemuPath(m.ISO),
			"-device", "ide-cd,drive=cd,bus=ide.0,bootindex=0",
		)
	}
	// A restart of the guest is an exit, and the launcher starts the machine again: stock
	// QEMU on the Windows Hypervisor Platform hangs in a guest's reset (docs/FINDINGS.md
	// of omacom/try-omarchy-windows), and the installer's last restart tells the launcher
	// to take the CD out.
	a = append(a, "-no-reboot")
	if m.Setup != "" {
		a = append(a, "-fw_cfg", "name="+setupFwCfg+",file="+qemuPath(m.Setup))
	}
	if m.SerialLog != "" {
		a = append(a, "-chardev", "file,id=serial,path="+qemuPath(m.SerialLog), "-serial", "chardev:serial")
	}
	sound := "dsound,id=audio"
	if m.NoSound {
		sound = "none,id=audio"
	}
	display := "gtk,show-tabs=off,zoom-to-fit=on,window-close=off"
	if versionAtLeast(m.Version, clipboardMajor, clipboardMinor) {
		display += ",clipboard=on" // an older QEMU refuses the option and does not start
	}
	a = append(a,
		"-device", "virtio-vga,xres=1920,yres=1080",
		"-display", display,
		"-device", "qemu-xhci",
		"-device", "usb-tablet",
		"-nic", "user,model=virtio-net-pci",
		"-device", "virtio-rng-pci",
		// Copy and paste: QEMU's own vdagent talks to spice-vdagent in the guest and to
		// the GTK window's clipboard, which is the Windows clipboard.
		"-device", "virtio-serial-pci",
		"-chardev", "qemu-vdagent,id=vdagent,name=vdagent,clipboard=on,mouse=off",
		"-device", "virtserialport,chardev=vdagent,name=com.redhat.spice.0",
		"-audiodev", sound,
		"-device", "intel-hda",
		"-device", "hda-duplex,audiodev=audio",
		"-qmp", "tcp:"+m.QMP+",server=on,wait=off",
	)
	return a
}

// What stopped QEMU as it started, as the end of its log tells: the Windows Hypervisor
// Platform refused it, or DirectSound found no sound device (a PC with no speakers or
// headphones attached, a remote session), which QEMU does not start without.
const (
	failedOther = iota
	failedHypervisor
	failedSound
)

func startFailure(tail string) int {
	t := strings.ToLower(tail)
	switch {
	case strings.Contains(t, "directsound") || strings.Contains(t, "dsound"):
		return failedSound
	case strings.Contains(t, "whpx"):
		return failedHypervisor
	}
	return failedOther
}

// sizing picks the machine's memory and processors from the host's: half of each,
// between 4 and 8 GB of memory and 2 and 8 processors, never more than the host has.
func sizing(hostMB uint64, hostCPUs int) (memoryMB, cpus int) {
	memoryMB = int(hostMB / 2)
	memoryMB = min(max(memoryMB, 4096), 8192)
	if uint64(memoryMB) > hostMB*3/4 {
		memoryMB = int(hostMB * 3 / 4)
	}
	memoryMB -= memoryMB % 256
	cpus = min(max(hostCPUs/2, 2), 8)
	cpus = min(cpus, max(hostCPUs, 1))
	return memoryMB, cpus
}

// lastLines is the end of a log, for an error message.
func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(s, "\r\n", "\n"), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
