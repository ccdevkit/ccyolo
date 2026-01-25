<h1 align="center">ccyolo</h1>

<p align="center">
Run Claude Code in Docker with auto-accept enabled. No setup, full parity with `claude` CLI.
</p>

<p align="center">
<img src="screenshots/demo.gif" alt="Demo" />
</p>

> [!CAUTION]
> ccyolo is a safety net, not a sandbox. It protects against overzealous agents doing stupid things, but it won't contain a fully compromised, malicious agent. Only use with repositories you trust.

## Install

```bash
npm install -g @ccdevkit/ccyolo
```

## Usage

Use it exactly like `claude`:

```bash
ccyolo                    # Interactive mode
ccyolo -p "hello"         # One-shot prompt
ccyolo -c                 # Continue previous session
ccyolo -r                 # Resume session picker
```

All `claude` flags work as expected.

## Why

- Works out of the box - no container setup required
- Full CLI parity with `claude`
- Runs with auto-accept enabled (YOLO mode)
- Your working directory is mounted into the container
- Your Claude credentials are passed through automatically

## ccyolo flags

ccyolo flags go before `--`, claude flags go after:

```bash
ccyolo [ccyolo-flags] -- [claude-args]
ccyolo [claude-args]

ccyolo -v -- -p "hello"              # Verbose mode
ccyolo --log /tmp/debug.log -- -c    # Log to file
ccyolo -pt:git -- -p "git status"    # Run git on host instead of container
ccyolo --use 2.1.16 --               # Use specific Claude Code version
```

| Flag                                  | Description                                       |
| ------------------------------------- | ------------------------------------------------- |
| `-v`, `--verbose`                     | Enable debug logging to stderr                    |
| `--log <path>`                        | Write debug logs to file (implies -v)             |
| `-c`, `--claudePath <path>`           | Path to claude CLI (default: claude in PATH)      |
| `--use <version>`                     | Use specific Claude Code version (e.g., 2.1.16)   |
| `-pt:<cmd>`, `--passthrough:<cmd>`    | Run commands matching prefix on host (repeatable) |
| `--version`                           | Print ccyolo version                              |

### Passthrough

By default, all commands run inside the container. This is usually fine, but some commands need to run on your host machine - things like `docker`, `gh`, or commands that need access to host resources.

Use `-pt:` to specify command prefixes that should run on the host:

```bash
ccyolo -pt:git -pt:docker -- -p "build and push the image"
```

This runs `git` and `docker` commands on your host, while everything else runs in the container. You can specify `-pt:` multiple times.

## Clipboard & Drag-Drop

ccyolo supports pasting images from your clipboard (Ctrl+V / Cmd+V) and dragging files into the terminal, just like the native `claude` CLI.

### How it works

When you paste or drag a file, ccyolo intercepts the input, copies the file into the container via a shared bridge directory, and rewrites the path so Claude sees it correctly.

### Platform support

| Platform | Clipboard (Ctrl+V) | File drag-drop |
|----------|-------------------|----------------|
| macOS (Intel & Apple Silicon) | ✅ | ✅ |
| Linux x64 | ✅ | ✅ |
| Windows x64 | ✅ | ✅ |
| Linux ARM64 | ❌ | ✅ |
| Windows ARM64 | ❌ | ✅ |

**Why no clipboard on ARM64?** Clipboard image support requires native system APIs (NSPasteboard, Win32, X11) which need CGO compilation. GitHub Actions doesn't provide native ARM64 runners for Linux or Windows, so those builds are cross-compiled without CGO. File drag-drop still works because it only requires path rewriting, not system clipboard access.

If you need clipboard support on ARM64, you can build from source on a native ARM64 machine with CGO enabled.

## Settings

ccyolo can be configured using a settings file in your project or home directory. Settings files are discovered by walking up from your current directory to root.

### Settings file location

Create a settings file at `.ccdevkit/ccyolo/settings.json` (or `.yaml`/`.yml`) in your project directory or home directory:

```
your-project/
  .ccdevkit/
    ccyolo/
      settings.json    # Project-specific settings
```

Or in your home directory for global settings:

```
~/.ccdevkit/
  ccyolo/
    settings.json      # Global settings
```

### Available settings

```json
{
  "claudePath": "/path/to/claude",
  "passthrough": ["git", "docker", "gh"]
}
```

| Setting | Type | Description |
|---------|------|-------------|
| `claudePath` | string | Path to the claude CLI executable (default: `claude` in PATH) |
| `passthrough` | array | List of command prefixes to run on the host instead of in the container |

### Settings priority

Settings are merged with the following priority (highest to lowest):

1. Command-line flags
2. Project settings (`.ccdevkit/ccyolo/settings.json` in current directory or ancestors)
3. Global settings (`~/.ccdevkit/ccyolo/settings.json`)
4. Defaults

### Example configurations

**Minimal setup:**
```json
{
  "passthrough": ["git"]
}
```

**Advanced setup:**
```json
{
  "claudePath": "/usr/local/bin/claude",
  "passthrough": ["git", "docker", "gh", "npm"]
}
```

**YAML format:**
```yaml
claudePath: /usr/local/bin/claude
passthrough:
  - git
  - docker
  - gh
```

## Requirements

- Docker
- `claude` must be authenticated on your machine
