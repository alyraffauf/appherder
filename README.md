[![CI](https://github.com/alyraffauf/appherder/actions/workflows/ci.yml/badge.svg)](https://github.com/alyraffauf/appherder/actions/workflows/ci.yml) [![License: GPL v3](https://img.shields.io/badge/License-GPL%20v3-blue.svg)](http://www.gnu.org/licenses/gpl-3.0) [![Ko-fi](https://img.shields.io/badge/Donate-Ko--fi-ff5e5b?logo=ko-fi&logoColor=white)](https://ko-fi.com/alyraffauf)

<div align="center">
  <h1>appherder</h1>
  <h3>A herder for your AppImages.</h3>
  <p>Install AppImages so they act like real apps instead of loose files in your Downloads.</p>
</div>

On its own, an AppImage is just an executable in a folder. No icon, no menu entry, nothing in your launcher.

appherder fixes that. Point it at an AppImage and you get a real app, kind of like dropping something into Applications on macOS. Delete it later and everything it set up goes too.

## Features

- **Set it and forget it.** Manages and upgrades your AppImages for you in the background.
- **Real apps, not loose files.** Installed AppImages show up in your application menu with their real name and icon.
- **Install from anywhere.** Point it at a local file or paste a download link.
- **Updates without the pile-up.** A newer version replaces the old one.
- **Stays out of the way.** It only touches launchers it created. Your Flatpaks and hand-made shortcuts are safe.

## Installation

### Download a binary

Grab `appherder-linux-amd64` from the [latest release](https://github.com/alyraffauf/appherder/releases/latest), then:

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

Install an app from a file or URL:

```bash
appherder install ~/Downloads/Foo-x86_64.AppImage
appherder install https://example.com/Foo.AppImage
```

See what you have, remove what you don't:

```bash
appherder list
appherder uninstall foo
```

Installing copies the AppImage into `~/AppImages`. That folder is the source of truth: add or remove files there and `appherder sync` matches your launchers to it.

```bash
appherder sync
```

Keep things up to date:

```bash
appherder upgrade              # download and install available updates
appherder upgrade --check      # just see what's out of date
```

Or let it handle itself:

```bash
appherder autosync             # sync whenever ~/AppImages changes
appherder autoupgrade          # check for updates once a day
```

Coming from another AppImage tool? `appherder migrate` adopts the ones in `~/AppImages` and clears out launchers whose AppImage is gone.

## Under the hood

appherder reads the AppImage's squashfs filesystem directly to grab its icon and desktop entry, then writes a launcher pointing back at the file in `~/AppImages`. It does this without ever running the AppImage, unlike tools that launch it to unpack. Everything it writes is tagged, so uninstall and sync only touch its own files.

## License

[GPLv3](LICENSE.md).
