# Contributing to lincli

> **Note**: This is a personal fork of [dorkitude/linctl](https://github.com/dorkitude/linctl) maintained by Shane Dolley for personal use.

This fork is not intended for public contribution or upstream PRs. If you'd like to contribute to the Linear CLI ecosystem, please contribute to the [upstream project](https://github.com/dorkitude/linctl) instead.

## Development (Personal Reference)

## Development

- Requirements: Go 1.22+, `gh` CLI (optional), `jq` (for examples), `golangci-lint` (optional).
- Useful targets:
  - `make deps` — install/tidy deps
  - `make build` — local build
  - `make test` — smoke tests (read-only commands)
  - `make lint` — lint if you have golangci-lint
  - `make fmt` — go fmt

## Release Process (Personal Reference)

This is a personal fork without Homebrew distribution. To create a new release:

1. **Prepare**
   - Ensure README and help text match behavior
   - Run `make test` to verify smoke tests pass
   - Update version numbers if needed

2. **Tag and Build**
   ```bash
   git tag vX.Y.Z -a -m "vX.Y.Z: short summary"
   git push origin vX.Y.Z
   make build
   make install
   ```

3. **Verify**
   ```bash
   lincli --version
   lincli docs | head -n 5
   ```

