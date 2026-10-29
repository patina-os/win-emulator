# Changelog

All notable changes to Try patina for Windows are documented here. The format follows
[Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/), and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0-alpha.1] - 2026-10-29

The first alpha, for testers with a Windows PC.

### Added

- `TryPatina.exe`: sets up stock QEMU with winget, downloads the latest patina image
  and checks it against its `SHA256SUMS`, runs patina's installer on a virtual disk
  and starts the installed patina from then on, on the Windows Hypervisor Platform,
  with emulation as the slow fallback.
- Copy and paste of text between Windows and patina, through QEMU's vdagent and
  spice-vdagent in patina's X11 session (patina's `devices/try-patina-windows/`).
- An icon in the notification area: shut patina down, turn it off, open the data
  folder, install patina again, remove it, and a note when a newer launcher is out.

[Unreleased]: https://github.com/patina-os/win-emulator/compare/v0.1.0-alpha.1...HEAD
[0.1.0-alpha.1]: https://github.com/patina-os/win-emulator/releases/tag/v0.1.0-alpha.1
