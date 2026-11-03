# Try patina for Windows

patina in a window on your Windows PC: a virtual machine that leaves your drive and
Windows as they are. `TryPatina.exe` asks for your account, downloads patina's released
disk, and starts it from then on. Your files in patina stay between sessions.

Windows 10 or 11 · x86_64 · hardware virtualization · more than 4 GB of memory

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
   missing (Windows asks for permission), then opens its setup window: your name, login
   name and password, the language, keyboard and time zone (taken from Windows), and
   what patina's installer asks on a PC: the apps, more apps, the AI agent and the
   theme.
4. **Wait for patina's first start.** The launcher downloads patina's disk (about
   4 GB, in two parts) from the newest of [patina's releases](https://github.com/patina-os/patina/releases)
   that has it, checks it against the release's `SHA256SUMS` and starts it. Its first
   start sets up your account and then its apps, which needs the internet and takes a
   while; the login screen comes after it.

From then on, `TryPatina.exe` starts patina straight away.

## Using it

- **Closing.** Shut patina down from its own menu, or with "Shut down patina" on the
  patina icon in the notification area. The window's close button does nothing, so a
  click cannot cut the power to patina's disk; "Turn off now…" in the icon's menu does,
  when patina hangs. So does QEMU's own Machine → Quit in the window (Ctrl+Alt+Q), at
  once and without asking, like pulling the plug: patina may lose what it had not
  saved.
- **Copy and paste** of text works both ways between Windows and patina, with QEMU
  11.1 or later (the setup offers to update an older one; `winget upgrade
  SoftwareFreedomConservancy.QEMU` does it by hand). patina's login screen offers the
  Plasma X11 session first on this machine, because that is where the clipboard is
  shared; the Wayland session stays on its session menu. Logging in automatically (a
  checkbox in the installer) starts Plasma (X11) too, with images after patina
  0.1.0-alpha.4. One installed from an earlier image logs into Wayland that way: pick
  Plasma (X11) for it in System Settings → Login Screen (SDDM) → Behavior.
- **Full screen and zoom** are in the window's View menu.
- **Updates** of patina are patina's own: `patina update` in a terminal. When a newer
  `TryPatina.exe` is released, its icon menu says so.
- **Starting over.** "Install patina again…" deletes the virtual disk and asks the
  setup again; the downloaded disk is used again unless a newer release has another.
  "Install with patina's installer…" deletes it and installs from patina's
  `patina-amd64.hybrid.iso` with the installer instead, as on a PC: press Enter at the
  boot menu, and "Erase disk" on the disk page erases only the virtual disk. The
  launcher takes that route by itself for a release without the disk. "Remove patina
  from this PC…" deletes the data folder; QEMU stays installed, remove it under
  Settings, Apps.

Everything lives in `%LOCALAPPDATA%\patina-vm`: patina's disk as downloaded
(`patina.qcow2`), the machine's own disk over it (`disk.qcow2`, which keeps what
patina changes and grows as patina uses it), its UEFI variables, `launcher.log`,
`qemu.log` and `serial.log`, where patina's first start reports. Keep about 40 GB free
on that drive.

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

- **The setup window** is one page in patina's look, served by the launcher on
  `127.0.0.1` under a random path and shown by Microsoft Edge as an app window (Edge
  draws WebView2 too; with a profile of its own, deleted afterwards), or by the default
  browser where Edge was removed. Its pages, words, pictures, colours and fonts are
  patina's: the release's `patina-setup.json`, the installer's own pages.

- **QEMU** is the official Windows build (`SoftwareFreedomConservancy.QEMU` in winget,
  from [qemu.weilnetz.de](https://qemu.weilnetz.de/)), on the Windows Hypervisor
  Platform (`-accel whpx`). When the hypervisor refuses the in-kernel interrupt
  controller (some laptops refuse nested virtualization) it retries with
  `kernel-irqchip=off`, then offers emulation.
- **The machine** is a q35 PC with UEFI (QEMU's edk2), an NVMe disk (so patina's I/O
  scheduler rules apply as on a laptop), virtio graphics at 1920x1080, a USB tablet,
  virtio networking, Intel HD audio on DirectSound (none when Windows has no sound
  device, which QEMU does not start without), half the host's memory (4 to 8 GB)
  and processors (2 to 8). The CPU model is `qemu64` with SSSE3, SSE4 and AES, the
  set the Windows Hypervisor Platform runs safely in stock QEMU.
- **Copy and paste** is QEMU's own vdagent (`-chardev qemu-vdagent`) between QEMU's GTK
  window and `spice-vdagent` in patina; no SPICE viewer is involved. The GTK window
  shares the Windows clipboard from QEMU 11.1 on, when asked (`-display
  gtk,clipboard=on`); the Windows builds before it have no clipboard in that window.
- **patina** comes from its released `patina-amd64.qcow2`: the system patina's
  installer leaves on a disk, without an account. The machine's own disk is a layer
  over it. The setup's answers reach patina once, as QEMU firmware configuration
  (`-fw_cfg name=opt/io.github.patina_os/setup`); its first start sets the account up
  from them and runs `patina install`, then says so on the serial port, and the
  launcher deletes the answers, the password among them. The launcher names the machine
  `Try patina` in its firmware tables (`-smbios type=1,product=Try patina`), and
  patina's [`devices/try-patina-windows/`](https://github.com/patina-os/patina/tree/main/devices/try-patina-windows)
  takes it from there: that first-start step, spice-vdagent and the X11 session first.
- **Restarts are exits** (`-no-reboot`): stock QEMU on the Windows Hypervisor Platform
  hangs in a guest's reset, so the launcher starts the machine again itself. The
  installer's last restart is how it sees that the installer is done.
- **Two hangs of stock QEMU** on the Windows Hypervisor Platform, found by [Try Omarchy
  for Windows](https://github.com/omacom/try-omarchy-windows/blob/main/docs/FINDINGS.md),
  are worked around: the launcher connects to QEMU's machine protocol (QMP) only ten
  seconds after the start, because a connection in the guest's first seconds can freeze
  the window for good, and it ends a QEMU that is still there fifteen seconds after the
  guest shut down.

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
