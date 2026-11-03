// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestArgsInstallRun(t *testing.T) {
	m := Machine{QEMUDir: `C:\Program Files\qemu`, DataDir: `C:\Users\a,b\patina-vm`,
		MemoryMB: 8192, CPUs: 4, Accel: accelWHPX, ISO: `C:\x\patina.iso`, QMP: "127.0.0.1:4444"}
	a := m.Args()
	joined := strings.Join(a, " ")
	for _, want := range []string{
		"-accel whpx", "-cpu " + cpuWHPX, "-smp 4", "-m 8192M", "-no-reboot",
		"-smbios type=1,product=Try patina",
		"file=" + strings.ReplaceAll(filepath.Join(m.DataDir, diskFile), "a,b", "a,,b"), // a comma is doubled
		"ide-cd,drive=cd,bus=ide.0,bootindex=0",
		"qemu-vdagent,id=vdagent,name=vdagent,clipboard=on",
		"virtserialport,chardev=vdagent,name=com.redhat.spice.0",
		"window-close=off", "zoom-to-fit=on", "grab-on-hover=on",
		"-device VGA,xres=1920,yres=1080", // one resolution, which the window scales
		"-qmp tcp:127.0.0.1:4444,server=on,wait=off",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args lack %q:\n%s", want, joined)
		}
	}
	// Every option is followed by its value: the list alternates.
	for i := 0; i < len(a); i++ {
		if !strings.HasPrefix(a[i], "-") {
			t.Fatalf("argument %d (%q) is not an option", i, a[i])
		}
		if a[i] != "-no-reboot" {
			i++
		}
	}
}

func TestSizingFitsTheInstaller(t *testing.T) {
	// The setup refuses a host whose machine would get less than the installer needs;
	// a host with 6 GB or more must pass.
	for _, hostMB := range []uint64{6144, 8192, 16384} {
		if mb, _ := sizing(hostMB, 4); mb < installerMemoryMB {
			t.Errorf("a host with %d MB gives the machine %d MB, less than the installer's %d", hostMB, mb, installerMemoryMB)
		}
	}
	for _, hostMB := range []uint64{3891, 4096} {
		if mb, _ := sizing(hostMB, 4); mb >= installerMemoryMB {
			t.Errorf("a host with %d MB gives the machine %d MB; the setup would let the installer refuse it", hostMB, mb)
		}
	}
}

func TestClipboardByVersion(t *testing.T) {
	for _, c := range []struct {
		version string
		want    bool
	}{
		{"11.1.0", true}, {"11.2.0-rc1", true}, {"12.0.0", true},
		{"11.0.0", false}, {"10.2.0", false}, {"", false}, {"unknown", false},
	} {
		m := Machine{QEMUDir: "q", Version: c.version, DataDir: "d", MemoryMB: 4096, CPUs: 2, Accel: accelWHPX, QMP: "x:1"}
		a := m.Args()
		display := a[slices.Index(a, "-display")+1]
		if got := strings.HasSuffix(display, ",clipboard=on"); got != c.want {
			t.Errorf("QEMU %q: -display %s, want clipboard=on: %v", c.version, display, c.want)
		}
	}
}

func TestNoSound(t *testing.T) {
	m := Machine{QEMUDir: "q", DataDir: "d", MemoryMB: 4096, CPUs: 2, Accel: accelWHPX, QMP: "x:1", NoSound: true}
	a := m.Args()
	if i := slices.Index(a, "-audiodev"); a[i+1] != "none,id=audio" {
		t.Errorf("without sound the audio backend is none, got %s", a[i+1])
	}
}

func TestStartFailure(t *testing.T) {
	// What QEMU 11.1 for Windows prints; seen under wine, which has neither.
	for _, c := range []struct {
		tail string
		want int
	}{
		{`C:\Program Files\qemu\qemu-system-x86_64w.exe: -accel whpx: Could not load library WinHvPlatform.dll.
C:\Program Files\qemu\qemu-system-x86_64w.exe: -accel whpx: failed to initialize whpx: Function not implemented`, failedHypervisor},
		{`C:\Program Files\qemu\qemu-system-x86_64w.exe: Could not initialize DirectSound: No sound driver is available for use, or the given GUID is not a valid DirectSound device ID`, failedSound},
		{`qemu-system-x86_64w.exe: -drive if=none,id=disk: Could not open 'disk.qcow2': The process cannot access the file because it is being used by another process.`, failedOther},
	} {
		if got := startFailure(c.tail); got != c.want {
			t.Errorf("startFailure(%q) = %d, want %d", c.tail, got, c.want)
		}
	}
}

func TestArgsInstalled(t *testing.T) {
	m := Machine{QEMUDir: "q", DataDir: "d", MemoryMB: 4096, CPUs: 2, Accel: accelEmulation, QMP: "x:1"}
	a := m.Args()
	joined := strings.Join(a, " ")
	if !slices.Contains(a, "-no-reboot") || strings.Contains(joined, "media=cdrom") ||
		slices.Contains(a, "-fw_cfg") || slices.Contains(a, "-serial") {
		t.Errorf("an installed machine has no CD, no answers and no serial log, and exits on a restart: %v", a)
	}
	if i := slices.Index(a, "-cpu"); a[i+1] != cpuEmulation {
		t.Errorf("emulation uses the max CPU model, got %s", a[i+1])
	}
}

func TestArgsSetup(t *testing.T) {
	m := Machine{QEMUDir: "q", DataDir: "d", MemoryMB: 4096, CPUs: 2, Accel: accelWHPX, QMP: "x:1",
		Setup: `C:\Users\a,b\patina-vm\setup.env`, SerialLog: `C:\Users\a,b\patina-vm\serial.log`}
	joined := strings.Join(m.Args(), " ")
	for _, want := range []string{
		"-fw_cfg name=opt/io.github.patina_os/setup,file=C:\\Users\\a,,b\\patina-vm\\setup.env",
		"-chardev file,id=serial,path=C:\\Users\\a,,b\\patina-vm\\serial.log -serial chardev:serial",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args lack %q:\n%s", want, joined)
		}
	}
}

func TestSizing(t *testing.T) {
	for _, c := range []struct {
		hostMB           uint64
		hostCPUs         int
		wantMB, wantCPUs int
	}{
		{32768, 16, 8192, 8},
		{16384, 8, 8192, 4},
		{8192, 4, 4096, 2},
		{4096, 2, 3072, 2}, // never more than three quarters of a small host
		{3891, 4, 2816, 2}, // a PC sold with 4 GB: too little for patina's installer
		{65536, 1, 8192, 1},
	} {
		mb, cpus := sizing(c.hostMB, c.hostCPUs)
		if mb != c.wantMB || cpus != c.wantCPUs {
			t.Errorf("sizing(%d, %d) = %d, %d; want %d, %d", c.hostMB, c.hostCPUs, mb, cpus, c.wantMB, c.wantCPUs)
		}
	}
}
