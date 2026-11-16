# CLAUDE.md

Context and rules for anyone, human or Claude, working in this repository. The README is
for users; this file is for contributors. patina's own rules
([patina-os/patina's CLAUDE.md](https://github.com/patina-os/patina/blob/main/CLAUDE.md))
apply where this file says nothing.

## What this is

Try patina for Windows: `TryPatina.exe`, which runs patina in a QEMU virtual machine on
a Windows PC. It installs stock QEMU with winget, asks in a setup window what patina's
installer asks, downloads patina's released disk image and starts it from then on (or
runs patina's installer from its ISO on an empty disk). It owns no image, no QEMU build
and no guest code: what the guest needs is patina's
`devices/try-patina-windows/`, detected by the product name the launcher sets
(`smbiosProduct` in `config.go`; the two change together).

## Decisions already made (do not re-litigate)

- Go, one program, cross-compiled from Linux, no cgo. Its own code is the only binary
  this repository makes; it is published only as a release asset, built by CI.
- Stock QEMU from winget (`SoftwareFreedomConservancy.QEMU`), on the Windows Hypervisor
  Platform. No patched QEMU bundle (WINQ-EMU), so no GPU in the guest for now:
  confirmed in November 2026, when the machine proved slow, since WINQ-EMU is an alpha,
  weakest on Intel's integrated graphics, and a binary to build or to trust. Speed
  comes from stock QEMU's own options: the guest runs without the hypervisor's separate
  security domain (`ssd=off`, from QEMU 11.1), one mitigation between Windows and one
  guest traded for faster emulated devices. The guest's processor stays the safe
  `qemu64` set; the host's (the start page's box, or `TRY_PATINA_CPU=host`) is a switch until a run
  on a real PC confirms it. Not done, on purpose: `cache=unsafe` (the launcher ends a hung QEMU),
  `pic=off` (Windows 10), more processors.
- patina keeps its look in the machine; the setup window says that it is slower there
  than on a PC and recommends patina's image on a PC (November 2026, in place of a
  plainer desktop in the machine).
- The guest starts from patina's released disk image (`patina-amd64.qcow2`, patina's
  `iso/disk/`: the installer's system without an account), under a qcow2 layer for its
  changes; the launcher asks what the installer asks in a setup window and hands the
  answers over once as QEMU firmware configuration (November 2026, after the installer
  in the VM lost its network twice: Try Omarchy's way, without its patched QEMU).
  Installing from the ISO with the installer stays, from the menu and for a release
  without the disk.
- The setup window is one HTML page served on 127.0.0.1 under a random path and shown
  in an Edge app window, or the default browser without Edge; its pages, words, colours
  and fonts come from the release's `patina-setup.json`, each answer a compact row and
  only the themes with their pictures (November 2026). No WebView2 library: the Go ones
  embed Microsoft's loader DLL or exit the program on an error.
- An installed Try patina opens with a start page, as the setup window is made, which
  starts patina by itself after ten seconds unless it is touched (November 2026, the
  maintainer's choice of a page over a dialog, of every start over only when asked):
  the machine's memory and processors, the host's processor as a switch, and starting
  over, which the tray's menu can do only once a machine runs. Its settings are few and
  kept in `settings.json`, which outlives a machine installed again.
- The display is standard VGA at one resolution, 1920x1080, which QEMU's window scales,
  and the launcher opens that window at four fifths of the screen (November 2026):
  with virtio graphics patina took the window's size as its resolution, so the window
  stayed as small as the firmware's first picture, and resizing it froze QEMU.
- The window takes the keyboard while the pointer is over it (`grab-on-hover`), so the
  Windows key reaches patina as Meta (November 2026): Windows kept it before, patina's
  Meta shortcuts did not work, and a key whose release Windows kept stayed down.
- Copy and paste through QEMU's `qemu-vdagent` and the GTK window, with spice-vdagent in
  patina's Plasma X11 session (its Wayland support is not released).
- Dependencies: `golang.org/x/sys`, `github.com/ncruces/zenity` (dialogs) and
  `fyne.io/systray` (the tray icon). A new one needs a reason in the commit message.
- License: GPL-3.0-or-later. Name: "Try patina", `TryPatina.exe`; patina is always
  lowercase.

## Layout

| File | Holds |
|---|---|
| `config.go` | names, URLs, sizes: what the launcher uses, in one place |
| `machine.go` | QEMU's command line, qemu-img's for the disk, and the machine's size |
| `qmp.go` | the QMP client (commands without arguments, events) |
| `download.go` | the latest release, `SHA256SUMS`, a resumable checked download |
| `page.go` | the local server a page of the launcher's is served by: a random path, the page's signs of life, the wait for its answer |
| `setup.go`, `setup.html` | the setup window: patina's pages, the answers and the file for the guest, the page |
| `start.go`, `start.html` | the start page of an installed machine: what it shows, the choice it sends back, the page |
| `locale.go` | Windows' language, keyboard and time zone in the guest's terms, and the setup's lists |
| `state.go`, `icon.go` | the data directory's state file and the start page's settings, the `.ico` packer |
| `*_windows.go` | Windows only: the host checks, the dialogs, the tray, the flow, what Windows says for the setup and the Edge window of both pages |
| `main_other.go` | a stub, so the rest builds and tests on Linux |
| `build.sh` | fetches patina's icon, writes the Windows resources, builds `dist/` |

## Conventions

- `gofmt`, `go vet` for Linux and for Windows, `go test ./...`; everything but the
  `_windows.go` files is tested on Linux. `build.sh` follows patina's bash conventions
  and is shellcheck-clean.
- The mark is never copied here: `build.sh` fetches it from patina's `branding/export/`
  at a tag; `assets/`, `*.syso` and `dist/` are not committed.
- Dialog text is short and plain, in patina's voice: what happens, what to do.
- Update `CHANGELOG.md` under Unreleased for anything a user sees. Commit messages: an
  imperative subject under 72 characters, a body that explains why.
- A release is a `v*` tag: CI builds and attaches `TryPatina.exe` to a draft release,
  published after one run on a real Windows PC. Its notes are the version's section of
  `CHANGELOG.md`; the release notes workflow, run by hand with a tag, sets them again
  for a release made before its section was cut.
