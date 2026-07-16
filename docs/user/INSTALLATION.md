# Installation

CalmsToolkit is distributed as a prebuilt Linux binary on
[GitHub Releases](https://github.com/calmcacil/CalmsToolkit/releases). Release
archives are available for `amd64` and `arm64`; no Go toolchain is required.

## Choose the correct archive

Check the machine architecture:

```bash
uname -m
```

| `uname -m` result | Release architecture |
|---|---|
| `x86_64` | `amd64` |
| `aarch64` or `arm64` | `arm64` |

Other architectures are not currently published.

## Install a release manually

1. Open the [latest release](https://github.com/calmcacil/CalmsToolkit/releases/latest).
2. Download the archive for the machine, such as
   `calmstoolkit_1.2.3_linux_amd64.tar.gz`, and `checksums.txt` from the release's
   **Assets** section.
3. In the download directory, verify and extract the archive. Replace the
   example filename with the file you downloaded:

   ```bash
   sha256sum --ignore-missing --check checksums.txt
   tar -xzf calmstoolkit_1.2.3_linux_amd64.tar.gz
   ```

   Do not install the binary if checksum verification fails.

4. Install the binary system-wide and confirm the installed version:

   ```bash
   sudo install -m 0755 calmstoolkit /usr/local/bin/calmstoolkit
   calmstoolkit version
   ```

If `/usr/local/bin` is not on your `PATH`, add it to `PATH` or choose a binary
directory that is. For an installation without `sudo`, use a user-owned
directory instead:

```bash
mkdir -p "$HOME/.local/bin"
install -m 0755 calmstoolkit "$HOME/.local/bin/calmstoolkit"
```

Ensure `$HOME/.local/bin` is on your `PATH` before invoking the command.

## Initial setup

Run the guided configuration after installation:

```bash
calmstoolkit config setup
calmstoolkit config validate
calmstoolkit doctor
```

The configuration is written to `~/.config/calmstoolkit/config.json` with
permissions `0600`. Use `--config` or `CALMSTOOLKIT_CONFIG` to select a different
file.

## Upgrade

Download and verify the desired newer release using the same process, then run
the `install` command again. The binary is replaced; the configuration file is
left unchanged. Read the release notes and the [migration guide](MIGRATION_UNIFIED_CLI.md)
before upgrading across a breaking release.

## Uninstall

Remove the installed binary:

```bash
sudo rm /usr/local/bin/calmstoolkit
```

For a user-local installation, remove `$HOME/.local/bin/calmstoolkit` instead.
Configuration is not removed automatically. If it is no longer needed, delete
`~/.config/calmstoolkit/config.json` separately; it can contain API keys and
tokens.

## Build from source

Building from source is intended for contributors. It requires the Go version
declared in `go.mod` plus `make`:

```bash
git clone https://github.com/calmcacil/CalmsToolkit.git
cd CalmsToolkit
make build
sudo make install
```

See [Contributing](../../CONTRIBUTING.md) for the full development checks.
