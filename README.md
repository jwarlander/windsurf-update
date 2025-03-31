# windsurf-update

A CLI tool to automatically update Windsurf to the latest version.

## Installation

```bash
go install
```

## Usage

Simply run the binary:

```bash
windsurf-update
```

The tool will:
1. Check for the latest version from the Windsurf update API
2. Download the appropriate version for your platform to `$HOME/Downloads`
3. Extract the archive to `$HOME/apps/Windsurf/`

## Requirements

- Go 1.21 or later
- Write access to `$HOME/Downloads` and `$HOME/apps` directories
