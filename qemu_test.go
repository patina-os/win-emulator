// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestQEMUStarts runs QEMU with the launcher's command line, where TRY_PATINA_QEMU
// names a QEMU (CI installs Debian's): the GTK window and DirectSound exist only in
// the Windows build, so they become headless here; the firmware is the distribution's
// OVMF. QEMU must come up, answer QMP and report the quit as the shutdown reason.
func TestQEMUStarts(t *testing.T) {
	qemu := os.Getenv("TRY_PATINA_QEMU")
	if qemu == "" {
		t.Skip("TRY_PATINA_QEMU is not set")
	}
	qemuDir, dataDir := t.TempDir(), t.TempDir()
	os.Mkdir(filepath.Join(qemuDir, "share"), 0o755)
	os.Symlink("/usr/share/OVMF/OVMF_CODE_4M.fd", filepath.Join(qemuDir, "share", firmwareCode))
	if err := copyFile("/usr/share/OVMF/OVMF_VARS_4M.fd", filepath.Join(dataDir, varsFile)); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("qemu-img", "create", "-q", "-f", "qcow2",
		filepath.Join(dataDir, diskFile), diskSize).CombinedOutput(); err != nil {
		t.Fatalf("qemu-img: %v %s", err, out)
	}
	m := Machine{QEMUDir: qemuDir, DataDir: dataDir, MemoryMB: 1024, CPUs: 2,
		Accel: accelEmulation, QMP: "127.0.0.1:44555"}
	args := m.Args()
	for i, a := range args {
		switch {
		case strings.HasPrefix(a, "gtk,"):
			args[i] = "none"
		case strings.HasPrefix(a, "dsound,"):
			args[i] = strings.Replace(a, "dsound", "none", 1)
		}
	}
	cmd := exec.Command(qemu, args...)
	var log strings.Builder
	cmd.Stdout, cmd.Stderr = &log, &log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	q, err := dialQMP(ctx, m.QMP)
	if err != nil {
		t.Fatalf("%v\n%s", err, log.String())
	}
	time.Sleep(2 * time.Second) // the firmware runs
	if err := q.Execute("quit"); err != nil {
		t.Fatal(err)
	}
	reason := ""
	for e := range q.Events() {
		if e.Event == "SHUTDOWN" {
			reason = shutdownReason(e)
		}
	}
	cmd.Wait()
	if reason != "host-qmp-quit" {
		t.Errorf("shutdown reason %q, want host-qmp-quit\n%s", reason, log.String())
	}
}
