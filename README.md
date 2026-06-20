[![CI](https://github.com/alyraffauf/appherder/actions/workflows/ci.yml/badge.svg)](https://github.com/alyraffauf/appherder/actions/workflows/ci.yml) [![License: GPL v3](https://img.shields.io/badge/License-GPL%20v3-blue.svg)](http://www.gnu.org/licenses/gpl-3.0) [![Ko-fi](https://img.shields.io/badge/Donate-Ko--fi-ff5e5b?logo=ko-fi&logoColor=white)](https://ko-fi.com/alyraffauf)

<div align="center">
  <h1>AppHerder</h1>
  <h3>A shepherd for your AppImages.</h3>
</div>

AppHerder automatically installs, removes, and upgrades your AppImages. Throw them in `~/AppImages` and AppHerder does the rest: apps appear in your menu, deleted ones disappear from it, and supported apps update in place.

## Features

- **Set it and forget it.** Watches `~/AppImages` and checks for updates in the background.
- **Real apps, not loose files.** Installed AppImages show up natively in your application menu.
- **Install from anywhere.** Point it at a local file or paste a download link.
- **Updates without the pile-up.** A newer version replaces the old one.
- **Verified updates.** Pins the publisher's signing key on first install, then refuses tampered updates.
- **One-command rollback.** A bad update? Put the old version back instantly.
- **Stays out of the way.** It only touches launchers it created. Your Flatpaks and hand-made shortcuts are safe.

## Installation

### Quick install

```bash
curl -sSL https://raw.githubusercontent.com/alyraffauf/appherder/main/scripts/install.sh | bash
```

This downloads the latest AppImage, installs it, and enables automatic sync and upgrades.

### Download a binary

Grab `appherder-linux-amd64` from the [latest release](https://github.com/alyraffauf/appherder/releases/latest), then:

```bash
chmod +x appherder-linux-amd64
sudo mv appherder-linux-amd64 /usr/local/bin/appherder
```

### AppImage

Download the `.AppImage` from the [latest release](https://github.com/alyraffauf/appherder/releases/latest), then:

```bash
chmod +x appherder-*-x86_64.AppImage
./appherder-*-x86_64.AppImage install ./appherder-*-x86_64.AppImage
appherder autosync
appherder autoupgrade
```

The install step copies it into `~/AppImages` and links it to `~/.local/bin/appherder` automatically. You may need to restart your terminal or run `export PATH="$HOME/.local/bin:$PATH"` for the command to be found.

### Nix flake

```bash
nix run github:alyraffauf/appherder
```

Or `nix profile install github:alyraffauf/appherder` to keep it around.

### Build from source

Requires Go 1.25+.

```bash
git clone https://github.com/alyraffauf/appherder.git
cd appherder
go build ./cmd/appherder
```

## Usage

Enable automatic sync and upgrades:

```bash
appherder autosync             # sync whenever ~/AppImages changes
appherder autoupgrade          # check for updates once a day
```

Then use `~/AppImages` like the place AppImages belong. Add a file and it gets a launcher. Remove a file and its launcher goes away. When an update is available, appherder installs it without leaving the old copy behind.

Install an app from a file or URL:

```bash
appherder install ~/Downloads/Foo-x86_64.AppImage
appherder install https://example.com/Foo.AppImage
```

See what you have, remove what you don't want:

```bash
appherder list
appherder uninstall foo
```

Put an app on your PATH so you can launch it from a terminal:

```bash
appherder link foo              # creates ~/.local/bin/foo symlink
appherder unlink foo            # removes it
```

Uninstall cleans up the symlink automatically.

Installing copies the AppImage into `~/AppImages`. That folder is the source of truth: add or remove files there and `appherder sync` matches your launchers to it.

```bash
appherder sync
```

Keep things up to date:

```bash
appherder upgrade              # download and install available updates
appherder upgrade --check      # just see what's out of date
```

Undo a bad update:

```bash
appherder rollback foo         # restore the version the last update replaced
appherder rollback foo 1.2.3   # or restore a specific saved version
```

AppHerder keeps the last few versions of each app and saves the current one whenever an install or upgrade replaces it.

Coming from another AppImage tool? `appherder migrate` adopts the ones in `~/AppImages` and clears out launchers whose AppImage is gone.

## Verified updates

Some AppImages are signed by their publisher. The first time AppHerder installs a signed app, it pins that signing key. From then on, every update must be signed by the same key: an unsigned, tampered, or differently-signed build is refused instead of installed. Changing the trusted key is deliberate, so swapping publishers means uninstalling and reinstalling. Apps that aren't signed keep working as before; the pin only takes effect once a real signature has been seen.

`appherder list` shows each app's status in the **SIGNATURE** column: `pinned` (key locked in), `signed` (carries a signature appherder hasn't pinned yet), or `none`.

## Configuration

AppHerder's directories and update sources can be customized via `~/.config/appherder/config.toml`. See [Configuration](docs/Configuration.md) for all options.

## Under the hood

AppHerder reads the AppImage's filesystem to grab its icon and desktop entry. SquashFS images are parsed in-process. DwarFS images require `dwarfsextract`; the AppHerder CLI AppImage bundles it, while native and source installs need `dwarfsextract` available on `PATH`. AppHerder does not automatically fall back to `--appimage-extract`, because that executes the AppImage runtime. Everything it writes is tagged, so uninstall and sync only touch its own files.

## License

[GPLv3](LICENSE.md).
