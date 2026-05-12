# Release

Liaison uses one repository and two release tag families:

- `vX.Y.Z` publishes the server platform release: `liaison`, `frontier`, the Web console, CLI edge packages, and the offline Docker bundle.
- `desktop-vX.Y.Z` publishes the desktop client installers. Use it only when `desktop-client/**` or desktop-bundled edge behavior changes.

## Server Release

1. Sync the release version in docs:

   ```bash
   make version=1.6.0
   ```

   This keeps `VERSION` as `1.6.0`, updates localized README badges, and rewrites server download URLs to `v1.6.0`.

2. Commit the version/docs change and merge it to `main`.

3. Create and push a server release tag from the target commit:

   ```bash
   git tag v1.6.0
   git push origin v1.6.0
   ```

4. GitHub Actions runs `Server Release` and uploads:

   ```text
   liaison-1.6.0-linux-amd64.tar.gz
   liaison-1.6.0-docker-amd64.tar.gz
   SHA256SUMS
   ```

The platform package already contains CLI edge packages for Linux, macOS, and Windows under its `edge/` directory, so edge packages are not uploaded as separate release assets.

The release workflow also runs the version sync in its checkout, so the README files embedded in the release tarball always match the tag.

The same workflow can also be started manually from GitHub Actions with a version input, but tag-based release is the normal path.

## Desktop Release

Create a desktop tag only when the desktop app itself or its bundled edge behavior changed:

```bash
git tag desktop-v1.6.0
git push origin desktop-v1.6.0
```
