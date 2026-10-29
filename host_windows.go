// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// What the launcher asks of Windows, through its own APIs: no PowerShell, no WMI.

// hypervisorPresent asks the Windows Hypervisor Platform (WinHvPlatform.dll, an
// optional feature) whether the hypervisor runs. Absent DLL or feature: false.
func hypervisorPresent() bool {
	proc := windows.NewLazySystemDLL("WinHvPlatform.dll").NewProc("WHvGetCapability")
	if proc.Find() != nil {
		return false
	}
	var buf [64]byte // WHV_CAPABILITY; WHvCapabilityCodeHypervisorPresent (0) fills a BOOL
	var written uint32
	hr, _, _ := proc.Call(0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&written)))
	return hr == 0 && *(*uint32)(unsafe.Pointer(&buf[0])) != 0
}

// armHost is true on Windows on ARM, where this x86_64 launcher runs emulated and the
// x86_64 guest could not use the hypervisor.
func armHost() bool {
	var process, native uint16
	if err := windows.IsWow64Process2(windows.CurrentProcess(), &process, &native); err != nil {
		return false
	}
	return native == 0xAA64 // IMAGE_FILE_MACHINE_ARM64
}

// hostMemoryMB is the installed memory, from GlobalMemoryStatusEx.
func hostMemoryMB() uint64 {
	var m struct {
		Length, MemoryLoad                           uint32
		TotalPhys, AvailPhys, TotalPageFile          uint64
		AvailPageFile, TotalVirtual, AvailVirtual, _ uint64
	}
	m.Length = uint32(unsafe.Sizeof(m))
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")
	if r, _, _ := proc.Call(uintptr(unsafe.Pointer(&m))); r == 0 {
		return 8192
	}
	return m.TotalPhys >> 20
}

// freeBytes is the space free for this user on the drive that holds dir.
func freeBytes(dir string) uint64 {
	var free, total, totalFree uint64
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil || windows.GetDiskFreeSpaceEx(p, &free, &total, &totalFree) != nil {
		return 0
	}
	return free
}

// findQEMU is QEMU's installation directory: where its installer recorded it, or the
// default under Program Files. Empty when QEMU is not installed.
func findQEMU() string {
	var dirs []string
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\QEMU`, registry.QUERY_VALUE); err == nil {
		if d, _, err := k.GetStringValue("Install_Dir"); err == nil {
			dirs = append(dirs, d)
		}
		k.Close()
	}
	dirs = append(dirs, filepath.Join(os.Getenv("ProgramFiles"), "qemu"))
	for _, d := range dirs {
		if fileExists(filepath.Join(d, qemuSystem)) && fileExists(filepath.Join(d, "share", firmwareCode)) {
			return d
		}
	}
	return ""
}

const (
	qemuSystem = "qemu-system-x86_64w.exe" // the build without a console window
	qemuImg    = "qemu-img.exe"
)

// installQEMU installs QEMU with winget; winget asks Windows for permission itself.
func installQEMU() error {
	if _, err := exec.LookPath("winget"); err != nil {
		return errors.New("winget is not available on this PC; install QEMU from https://qemu.weilnetz.de/w64/ and open Try patina again")
	}
	cmd := hidden(exec.Command("winget", "install", "--id", qemuWingetID, "--exact", "--silent",
		"--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity"))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("winget could not install QEMU: %v\n%s", err, lastLines(string(out), 5))
	}
	return nil
}

// hidden keeps a console program's window from flashing up.
func hidden(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	return cmd
}

// singleInstance is false when another Try patina runs for this user.
func singleInstance() bool {
	name, _ := windows.UTF16PtrFromString(`Local\TryPatina`)
	_, err := windows.CreateMutex(nil, false, name)
	return !errors.Is(err, windows.ERROR_ALREADY_EXISTS)
}

// open opens a folder, a URL or a program the way Explorer would.
func open(target string) {
	verb, _ := windows.UTF16PtrFromString("open")
	t, _ := windows.UTF16PtrFromString(target)
	windows.ShellExecute(0, verb, t, nil, nil, windows.SW_SHOWNORMAL)
}
