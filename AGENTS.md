# VAR-Studio Agent Guide

These instructions apply to the entire repository.

## Product

- Use `Variscite Studio` as the public product name.
- Keep `var-studio` for commands, services, images, and repository paths.
- Run the application locally on Variscite boards.
- Do not add external telemetry or upload board data.
- Build and run the dashboard with Docker on the target board.
- Build images locally. Do not use GHCR or another image registry.
- Preserve `aarch64`, `arm64`, and `armv7l` support.

## Installer

- Keep this public installation command functional:

  ```sh
  curl -kfsSL \
    https://github.com/dorta/var-studio/raw/main/install.sh | sh
  ```

- Keep the installer compatible with POSIX `sh`.
- Recover an incorrect system clock before verified downloads.
- Use Docker Buildx when present and the legacy builder otherwise.
- Make `var-studio upgrade` install the latest source.
- Make uninstall remove every VAR-Studio resource and legacy VAR-Scope
  resource without removing unrelated Docker resources.

## Interface and documentation

- Follow the visual language used by the Variscite website.
- Do not add arbitrary fonts, colors, gradients, or decorative bars.
- Use stable skeleton layouts while data is loading.
- Keep page footers at the bottom of the viewport on short pages.
- Explain diagnostics before exposing technical evidence.
- Keep hardware inventory and capabilities in Hardware Explorer.
- Limit the README to the centered logo, installation, and license.

## Code and validation

- No source-code line may exceed 80 columns.
- Keep shell scripts compatible with POSIX `sh` unless marked as Bash.
- Keep diagnostic and information actions read-only by default.
- Preserve user changes and avoid unrelated rewrites.
- Do not add generated build artifacts to Git.
- Keep every file covered by the SPDX declarations in `REUSE.toml`.
- Preserve third-party license overrides and attribution.
- Run the following checks before publishing:

  ```sh
  ./scripts/check-line-length
  sh -n bootstrap.sh
  sh -n install.sh
  sh -n scripts/var-studio
  sh -n scripts/var-studio-stack
  go vet ./...
  go test ./...
  docker build --tag var-studio:ci .
  ```

- Run `node --check` for every JavaScript file changed.
- Build Linux AMD64 and ARM64 binaries when build logic changes.
- Test runtime changes on an available Variscite board.

## Git and releases

- For now, keep the reachable history as exactly one commit.
- Amend the existing root commit instead of adding another commit.
- Author and commit as `Diego Dorta <diego.d@variscite.com>`.
- Keep this trailer in the commit message:

  ```text
  Signed-off-by: Diego Dorta <diego.d@variscite.com>
  ```

- A cryptographic GPG or SSH signature is not required.
- Keep only one version tag and one GitHub release.
- Make the tag match `VERSION` using the format `vX.Y.Z`.
- Mark the current release as `Latest`.
- Remove the old release only after all current checks pass.
- Do not publish a container image to a registry.
- Confirm GitHub Actions succeeded before reporting completion.
