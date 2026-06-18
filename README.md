[![Tests](https://github.com/alyraffauf/appherder/actions/workflows/tests.yml/badge.svg)](https://github.com/alyraffauf/appherder/actions/workflows/tests.yml) [![License: GPL v3](https://img.shields.io/badge/License-GPL%20v3-blue.svg)](http://www.gnu.org/licenses/gpl-3.0) [![Ko-fi](https://img.shields.io/badge/Donate-Ko--fi-ff5e5b?logo=ko-fi&logoColor=white)](https://ko-fi.com/alyraffauf)

<div align="center">
  <h1>appherder</h1>
  <h3>A herder for your AppImages.</h3>
  <p>Install AppImages so they act like real apps instead of loose files in your Downloads.</p>
</div>

On its own, an AppImage is just an executable in a folder. No icon, no menu entry, nothing in your launcher. appherder fixes that: point it at an AppImage and you get a real app, kind of like dropping something into Applications on macOS. Delete it later and everything it set up goes too.

## Features

- **Shows up like a real app.** Lands in your application menu with its real name and icon.
- **Uninstalls cleanly.** Remove an app and its launcher and icon go with it. No leftovers.
- **Upgrades replace instead of piling up.** appherder names an app by what's inside it, not the download's filename, so a newer version of `Foo` just replaces the old one.
- **Won't touch your other apps.** It only removes launchers it made itself, so your Flatpaks, Snaps, and hand-made shortcuts are safe.
- **Quiet when nothing changed.** Re-installing an unchanged app does nothing. Drop your AppImages in one folder and `appherder sync` lines everything up.

## Installation

### Download a binary

Grab `appherder-linux-amd64` (or `-arm64`) from the [latest release](https://github.com/alyraffauf/appherder/releases/latest), then:

```bash
chmod +x appherder-linux-amd64
sudo mv appherder-linux-amd64 /usr/local/bin/appherder
```

### Nix flake

```bash
nix run github:alyraffauf/appherder
```

Or `nix profile install github:alyraffauf/appherder` to keep it around.

### Build from source

Requires Go 1.24+.

```bash
git clone https://github.com/alyraffauf/appherder.git
cd appherder
go build ./cmd/appherder
```

## Usage

```bash
appherder install ~/Downloads/Foo-x86_64.AppImage    # install one
appherder uninstall foo                              # remove one
appherder sync                                       # match your apps to what's in ~/AppImages
appherder migrate                                    # adopt apps another tool set up
```

Installing copies the AppImage into `~/AppImages`, so you can delete the original download. That folder is the source of truth: add or remove files there and `appherder sync` matches your launchers to it. To uninstall, use the name the file has in `~/AppImages` (without `.appimage`).

Coming from another AppImage tool? `appherder migrate` adopts the ones in `~/AppImages` and clears out launchers whose AppImage is gone, leaving everything else alone.

## Under the hood

appherder reads the AppImage's squashfs filesystem directly to grab its icon and desktop entry, then writes a launcher pointing back at the file in `~/AppImages`. It does this without ever running the AppImage, unlike tools that launch it to unpack. Everything it writes is tagged, so uninstall and sync only touch its own files.

## License

[GPLv3](LICENSE.md).
