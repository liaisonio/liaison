# Repository scripts

Run these commands from the repository root unless noted otherwise.

- `deploy-liaison.sh`: development deployment over SSH; requires explicit target
  hosts. See [configuration instructions](../etc/README.md#remote-development-deployment).
  The script resolves repository paths itself and can also be invoked by absolute path.
  Packaged Docker installation remains under `deploy/docker/`.
- `convert_svg_to_ico.py`: favicon conversion utility; requires `rsvg-convert`
  and Pillow. Input and output paths are relative to the caller's working directory.
- `test-config-portability.sh`: checks deployment target validation and safe
  example configuration without deploying anything.
- `resolve-release-version.sh`, `sync-release-version.sh`,
  `test-release-version.sh`: release version resolution, synchronization and checks.

Temporary screenshots and verification artifacts belong in the Git-ignored
`output/` directory. Documentation images that are actually referenced belong
in `docs/assets/`; application assets stay with their consuming application.
