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
		"window-close=off",
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

func TestArgsInstalled(t *testing.T) {
	m := Machine{QEMUDir: "q", DataDir: "d", MemoryMB: 4096, CPUs: 2, Accel: accelEmulation, QMP: "x:1"}
	a := m.Args()
	if slices.Contains(a, "-no-reboot") || strings.Contains(strings.Join(a, " "), "media=cdrom") {
		t.Errorf("an installed machine has no CD and restarts normally: %v", a)
	}
	if i := slices.Index(a, "-cpu"); a[i+1] != cpuEmulation {
		t.Errorf("emulation uses the max CPU model, got %s", a[i+1])
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
		{65536, 1, 8192, 1},
	} {
		mb, cpus := sizing(c.hostMB, c.hostCPUs)
		if mb != c.wantMB || cpus != c.wantCPUs {
			t.Errorf("sizing(%d, %d) = %d, %d; want %d, %d", c.hostMB, c.hostCPUs, mb, cpus, c.wantMB, c.wantCPUs)
		}
	}
}
