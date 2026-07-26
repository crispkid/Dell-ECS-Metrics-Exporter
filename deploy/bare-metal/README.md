# Bare Metal deployment

This deployment mode targets systemd-based Linux hosts on amd64 or arm64. It
installs the exporter as the unprivileged `ecs-exporter` account, preserves
operator configuration across upgrades, and applies systemd sandboxing.
The host needs systemd, curl, tar, and standard Linux install/find/user
management tools; installation and removal require root privileges.

Use a release archive for the host architecture and verify its checksum and
Sigstore signature before extracting it. From the extracted directory:

```bash
sudo ./deploy/bare-metal/install.sh \
  --binary ./ecs-exporter \
  --profiles ./profiles \
  --no-start

sudo install -m 0640 -o root -g ecs-exporter /secure/path/username \
  /etc/dell-ecs-metrics-exporter/secrets/username
sudo install -m 0640 -o root -g ecs-exporter /secure/path/password \
  /etc/dell-ecs-metrics-exporter/secrets/password
sudo install -m 0640 -o root -g ecs-exporter /secure/path/inventory-token \
  /etc/dell-ecs-metrics-exporter/secrets/inventory-token

sudoedit /etc/dell-ecs-metrics-exporter/config.yaml
sudo -u ecs-exporter /usr/local/bin/ecs-exporter \
  -config=/etc/dell-ecs-metrics-exporter/config.yaml \
  -profiles-dir=/usr/share/dell-ecs-metrics-exporter/profiles \
  -validate-config
sudo systemctl enable --now dell-ecs-metrics-exporter
sudo ./deploy/bare-metal/verify.sh
```

The default configuration binds to `127.0.0.1:8080`. Keep this default when a
local Prometheus agent or reverse proxy is used. If the service must listen on a
host interface, restrict port 8080 with the host firewall and protect Inventory
API access.

Running `install.sh` again upgrades the binary, profiles, and unit while
preserving `config.yaml` and `secrets/`. Without `--no-start`, an upgrade
restarts the service and restores the previous binary and profiles if startup
fails. With `--no-start`, restart the already-running service explicitly after
validation so it begins using the new binary. To uninstall:

```bash
sudo ./deploy/bare-metal/uninstall.sh
```

Configuration and secrets remain under `/etc/dell-ecs-metrics-exporter`.
Deleting them requires the explicit `--purge-config` option.
