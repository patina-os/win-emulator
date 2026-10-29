# CLAUDE.md

Context and rules for anyone, human or Claude, working in this repository. The README is
for users; this file is for contributors. patina's own rules
([patina-os/patina's CLAUDE.md](https://github.com/patina-os/patina/blob/main/CLAUDE.md))
apply where this file says nothing.

## What this is

Try patina for Windows: `TryPatina.exe`, which runs patina in a QEMU virtual machine on
a Windows PC. It installs stock QEMU with winget, downloads patina's released image,
runs patina's installer on a virtual disk and starts that disk from then on. It owns no
image, no QEMU build and no guest code: what the guest needs is patina's
`devices/try-patina-windows/`, detected by the product name the launcher sets
(`smbiosProduct` in `config.go`; the two change together).

## Decisions already made (do not re-litigate)

- Go, one program, cross-compiled from Linux, no cgo. Its own code is the only binary
  this repository makes; it is published only as a release asset, built by CI.
- Stock QEMU from winget (`SoftwareFreedomConservancy.QEMU`), on the Windows Hypervisor
  Platform. No patched QEMU bundle (WINQ-EMU), so no GPU in the guest for now.
- The guest is installed from patina's released ISO with its installer, never from a
  prebuilt disk image.
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
| `machine.go` | QEMU's command line and the machine's size |
| `qmp.go` | the QMP client (commands without arguments, events) |
| `download.go` | the latest release, `SHA256SUMS`, a resumable checked download |
| `state.go`, `icon.go` | the data directory's state file, the `.ico` packer |
| `*_windows.go` | Windows only: the host checks, the dialogs, the tray, the flow |
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
  published after one run on a real Windows PC.
