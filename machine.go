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
	DataDir  string // holds the disk, the UEFI variables and the logs
	MemoryMB int
	CPUs     int
	Accel    string // whpx, whpx,kernel-irqchip=off or tcg
	ISO      string // set for the install run: the image as a CD, and no reboot
	QMP      string // host:port of the QMP socket the launcher talks to
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
		// The installer ends with a restart; -no-reboot turns it into an exit, so the
		// launcher can take the CD out and start the installed system.
		a = append(a,
			"-drive", "if=none,id=cd,media=cdrom,readonly=on,file="+qemuPath(m.ISO),
			"-device", "ide-cd,drive=cd,bus=ide.0,bootindex=0",
			"-no-reboot",
		)
	}
	a = append(a,
		"-device", "virtio-vga,xres=1920,yres=1080",
		"-display", "gtk,show-tabs=off,zoom-to-fit=on,window-close=off",
		"-device", "qemu-xhci",
		"-device", "usb-tablet",
		"-nic", "user,model=virtio-net-pci",
		"-device", "virtio-rng-pci",
		// Copy and paste: QEMU's own vdagent talks to spice-vdagent in the guest and to
		// the GTK window's clipboard, which is the Windows clipboard.
		"-device", "virtio-serial-pci",
		"-chardev", "qemu-vdagent,id=vdagent,name=vdagent,clipboard=on,mouse=off",
		"-device", "virtserialport,chardev=vdagent,name=com.redhat.spice.0",
		"-audiodev", "dsound,id=audio",
		"-device", "intel-hda",
		"-device", "hda-duplex,audiodev=audio",
		"-qmp", "tcp:"+m.QMP+",server=on,wait=off",
	)
	return a
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
