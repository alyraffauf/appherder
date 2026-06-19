# Configuration

AppHerder reads `~/.config/appherder/config.toml` on startup. Missing or malformed keys fall back to defaults.

## Example

```toml
appimages_dir = "/data/AppImages"
max_saved_versions = 5
bin_dir = "/usr/local/bin"

[sources.firefox]
type = "github"
owner = "mozilla"
repo = "geckodriver"
tag = "latest"
pattern = "Firefox-*.AppImage"

[sources.librewolf]
type = "gitlab"
host = "gitlab.com"
project = "librewolf-community/browser/appimage"
tag = "latest"
pattern = "LibreWolf-*.AppImage"

[sources.myapp]
type = "static"
url = "https://example.com/MyApp-latest.AppImage"
```

## Settings

| Key | Type | Default | Description |
|---|---|---|---|
| `appimages_dir` | string | `~/AppImages` | Directory where AppImages live. |
| `max_saved_versions` | int | `3` | Number of prior versions kept for rollback. |
| `bin_dir` | string | `~/.local/bin` | Directory for `appherder link` symlinks. |

## Source overrides

The `[sources]` table overrides the update source for an app, taking priority over the embedded `.upd_info` ELF section. The key is the app name (the filename without `.appimage`).

### github

GitHub Releases.

```toml
[sources.appname]
type = "github"
owner = "owner"
repo = "repo"
tag = "latest"
pattern = "*-x86_64.AppImage"
```

Set `GH_TOKEN` or `GITHUB_TOKEN` in the environment for higher API rate limits.

### gitlab

GitLab Releases (gitlab.com or self-hosted).

```toml
[sources.appname]
type = "gitlab"
host = "gitlab.com"
project = "group/project"
tag = "latest"
pattern = "*-x86_64.AppImage"
```

Set `GL_TOKEN` or `GITLAB_TOKEN` in the environment for higher API rate limits.

### static

A fixed URL that always serves the latest AppImage.

```toml
[sources.appname]
type = "static"
url = "https://example.com/App-latest.AppImage"
```

### zsync

A zsync control file at a fixed URL.

```toml
[sources.appname]
type = "zsync"
url = "https://example.com/App-latest.AppImage.zsync"
```
