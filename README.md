# Try patina for Windows

patina in a window on your Windows PC: a virtual machine that leaves your drive and
Windows as they are. `TryPatina.exe` sets it up, installs patina from its released
image, and starts it from then on. Your files in patina stay between sessions.

Windows 10 or 11 · x86_64 · hardware virtualization

## Get started

1. **Turn on the Windows Hypervisor Platform.** Open "Turn Windows features on or off",
   tick "Windows Hypervisor Platform", press OK and restart. Virtualization must be on
   in the PC's firmware settings (Intel VT-x or AMD-V); most PCs ship with it on.
   Without the hypervisor patina still runs, emulated and very slowly.
2. **Download and open `TryPatina.exe`** from the
   [latest release](https://github.com/patina-os/win-emulator/releases/latest), with
   its SHA-256 beside it. It is not signed yet, so Windows SmartScreen warns the first
   time: "More info", then "Run anyway".
3. **Let it set up.** It installs [QEMU](https://www.qemu.org/) with winget if it is
   missing (Windows asks for permission), downloads patina's installer (about 1 GB)
   from [patina's releases](https://github.com/patina-os/patina/releases) and checks
   it against the release's `SHA256SUMS`.
4. **Install patina in the window that opens**, as on a PC. Press Enter at the boot
   menu. On the disk page "Erase disk" erases only the virtual disk. When the installer
   is done, patina restarts by itself and its first start sets up its apps, which needs
   the internet and takes a while.

From then on, `TryPatina.exe` starts patina straight away.

## Using it

- **Closing.** Shut patina down from its own menu, or with "Shut down patina" on the
  patina icon in the notification area. The window's close button does nothing, so a
  click cannot cut the power to patina's disk; "Turn off now…" in the icon's menu does,
  when patina hangs.
- **Copy and paste** of text works both ways between Windows and patina.
  patina's login screen offers the Plasma X11 session first on this machine, because
  that is where the clipboard is shared; the Wayland session stays on its session menu.
- **Full screen and zoom** are in the window's View menu.
- **Updates** of patina are patina's own: `patina update` in a terminal. When a newer
  `TryPatina.exe` is released, its icon menu says so.
- **Starting over.** "Install patina again…" deletes the virtual disk and runs the
  installer of the latest release. "Remove patina from this PC…" deletes the data
  folder; QEMU stays installed, remove it under Settings, Apps.

Everything lives in `%LOCALAPPDATA%\patina-vm`: the virtual disk (`disk.qcow2`, which
grows as patina uses it), its UEFI variables, `launcher.log` and `qemu.log`. Keep about
40 GB free on that drive.

## What it is not

- No GPU in the guest: stock QEMU draws patina's desktop on the CPU, which is fine for
  trying patina and slower than on hardware. Effects and video are the first to show it.
- No shared folders, no USB devices and no SSH port yet; copy files through the
  clipboard or a cloud folder (the Nextcloud client is in patina's default apps).
- Not for Windows on ARM: patina's image is x86_64 there.

## Under the hood

The launcher is one Go program (`TryPatina.exe`, no installer of its own) with two
libraries: [zenity](https://github.com/ncruces/zenity) for its dialogs and
[systray](https://github.com/fyne-io/systray) for its icon. It owns no image and no
binary but itself:

- **QEMU** is the official Windows build (`SoftwareFreedomConservancy.QEMU` in winget,
  from [qemu.weilnetz.de](https://qemu.weilnetz.de/)), on the Windows Hypervisor
  Platform (`-accel whpx`). When the hypervisor refuses the in-kernel interrupt
  controller (some laptops refuse nested virtualization) it retries with
  `kernel-irqchip=off`, then offers emulation.
- **The machine** is a q35 PC with UEFI (QEMU's edk2), an NVMe disk (so patina's I/O
  scheduler rules apply as on a laptop), virtio graphics at 1920x1080, a USB tablet,
  virtio networking, Intel HD audio on DirectSound, half the host's memory (4 to 8 GB)
  and processors (2 to 8). The CPU model is `qemu64` with SSSE3, SSE4 and AES, the
  set the Windows Hypervisor Platform runs safely in stock QEMU.
- **Copy and paste** is QEMU's own vdagent (`-chardev qemu-vdagent`) between QEMU's GTK
  window and `spice-vdagent` in patina; no SPICE viewer is involved.
- **patina** comes from its released `patina-amd64.hybrid.iso` and its installer, as
  on a PC (patina's Path B, which runs Path A on first boot). The launcher names the machine
  `Try patina` in its firmware tables (`-smbios type=1,product=Try patina`), and
  patina's [`devices/try-patina-windows/`](https://github.com/patina-os/patina/tree/main/devices/try-patina-windows)
  takes it from there: spice-vdagent and the X11 session first.
- **The install run** boots the image with `-no-reboot`: the installer's restart ends
  QEMU, the launcher reads the reason from QMP (`guest-reset`), takes the image out and
  starts the installed system.

## Building

Go (the version in `go.mod`), bash and curl, on any OS:

```sh
./build.sh v0.1.0
```

It fetches patina's small icon from `branding/export/` of patina (this repository
carries no copy of the mark), writes the Windows resources with
[go-winres](https://github.com/tc-hib/go-winres) and cross-compiles
`dist/TryPatina.exe` and its checksum. `go test ./...` runs the tests on any OS;
with `TRY_PATINA_QEMU=qemu-system-x86_64` (and OVMF installed) it also starts QEMU
with the launcher's command line, as CI does.

## License

GPL-3.0-or-later, as patina. QEMU, patina's image and the libraries carry their own
licenses.
