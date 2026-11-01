# Changelog

All notable changes to Try patina for Windows are documented here. The format follows
[Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/), and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Copy and paste between Windows and patina: QEMU 11.1, the one winget installs,
  shares the Windows clipboard with its window only when asked, and the launcher did
  not ask. It does now (`-display gtk,clipboard=on`), and the setup offers to update an
  older QEMU, whose window has no clipboard at all.
- patina starts on a PC without a sound device (no speakers or headphones attached):
  QEMU stopped at once there, because DirectSound found nothing to play on. The
  launcher now starts it again without sound.
- The setup no longer fails with a bare "404 Not Found" while patina's latest release
  has no image: its images are attached a quarter of an hour after it is published,
  once they passed their boot test, or not at all when it failed. The launcher then
  installs the newest release before it that has one.
- On a PC with 4 GB of memory the setup downloaded the image and opened an installer
  that refused to go on: the machine gets at most three quarters of the PC's memory,
  less than the 3 GB patina's installer asks for. The setup now says so before the
  download.
- A download stopped before a new patina release came out no longer ends in "does
  not match its checksum": the launcher continued the old image's part with the new
  image. It now starts over by itself when a continued download does not match.

## [0.0.2-alpha] - 2026-10-29

The second alpha, for the image of patina 0.1.0-alpha.4.

### Added

- When the last install did not finish, as far as the launcher can tell, it offers to
  start the disk as it is ("patina is installed: start it"), next to starting the
  installer again or over with an empty disk. An installer closed or restarted without
  the launcher noticing no longer leaves only a reinstall.

## [0.0.1-alpha] - 2026-10-29

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

[Unreleased]: https://github.com/patina-os/win-emulator/compare/v0.0.2-alpha...HEAD
[0.0.2-alpha]: https://github.com/patina-os/win-emulator/releases/tag/v0.0.2-alpha
[0.0.1-alpha]: https://github.com/patina-os/win-emulator/releases/tag/v0.0.1-alpha
