# Local configuration

The checked-in YAML files contain no usable credentials. Do not run them unchanged.

1. Copy `etc/liaison.yaml` to `etc/liaison.local.yaml`, and
   `etc/liaison-edge.yaml` to `etc/liaison-edge.local.yaml` as needed.
2. Generate a unique manager JWT secret, for example with `openssl rand -hex 32`,
   and set `manager.jwt_secret` in the local manager configuration.
3. Create a connector in your own Liaison installation and place its issued
   `access_key` and `secret_key` in the local edge configuration.
4. Set the addresses and TLS configuration for your environment. Start each
   binary with `-c` pointing to the corresponding local configuration file.

`*.local.yaml` is ignored by Git. Keep these files private and restrict their
permissions (for example, `chmod 600 etc/*.local.yaml`). Never commit generated
credentials or copy a secret from an example into a deployed environment.

Packaged installations have their own configuration-generation workflow; these
development examples do not change the package installer.

## Remote development deployment

The repository-root `deploy-liaison.sh` requires explicit target hosts:

```sh
MANAGER_HOST=manager.example.com ./deploy-liaison.sh --web
EDGE_HOST=edge.example.com ./deploy-liaison.sh --edge
```

For a full deployment, provide both `MANAGER_HOST` and `EDGE_HOST`. SSH users and
ports can be overridden with `MANAGER_USER`, `MANAGER_PORT`, `EDGE_USER` and
`EDGE_PORT`. Use your local SSH configuration or agent for authentication.
