# windsurf-update

A CLI tool to automatically update tarball-based Windsurf installations.

This tool is primarily intended for use on non-Debian Linux distributions, where there are currently no official native packages available for convenient updates.

_Please note that `windsurf-update` **will** try to create the directory you specify with the `--install-path` option, and install files into it, so make sure that's where you actually want the `windsurf` binary and all related files to be._

## Installation

You can install the tool in your GOBIN using `go install`:
```bash
go install github.com/jwarlander/windsurf-update@latest
```

Alternatively, you can clone the repo, build, and install manually, for example in `$HOME/.local/bin`:
```bash
git clone https://github.com/jwarlander/windsurf-update
cd windsurf-update
go build .
mv windsurf-update $HOME/.local/bin/
```

## Usage

Simply run the binary:

```bash
windsurf-update
```

The tool will:
1. Check for the latest version from the Windsurf update API
2. Download the appropriate version for your platform to `$HOME/Downloads`
3. Install into `$HOME/apps/windsurf/`

_**NOTE:** The Windsurf archive contains a top-level `Windsurf` directory, which is stripped off during installation so that the files below it actually end up where you expect them to be (eg. in `$HOME/apps/windsurf/` by default, not `$HOME/apps/windsurf/Windsurf/`)._

### Useful Options

- `--download-path`: Where to download the archive (default: `$HOME/Downloads`)
- `--install-path`: Where to install Windsurf (default: `$HOME/apps/windsurf`)
- `--force-update`: Force update even if already up to date
- `--yes`: Assume yes to all prompts (like removing an existing directory referenced by `--install-path`)
