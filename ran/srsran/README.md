# srsRAN Integration

This directory keeps the srsRAN/free5GC integration files for this project.
The srsRAN source tree is vendored under `external/srsRAN` so changes
to srsRAN code are tracked by this repository.

## Layout

- `external/srsRAN/`: vendored srsRAN Project source. This directory is intentionally tracked by the parent repository.
- `ran/srsran/gnb_zmq.yaml`: gNB config for software RF tests without USRP.
- `ran/srsran/gnb_usrp.yaml`: gNB config template for later USRP tests.
- `ran/srsran/free5gc-*.patch`: free5GC config changes needed when gNB/UPF use the host network instead of loopback N3.
- `scripts/bootstrap_srsran_local.sh`: fetches upstream srsRAN source and vendors it into `external/srsRAN`.
- `scripts/sync_srsran_to_remote.sh`: pushes local srsRAN source to the remote Linux host.
- `scripts/sync_srsran_from_remote.sh`: pulls remote srsRAN source back to local.
- `scripts/run_free5gc_srsran_remote.sh`: starts free5GC with srsRAN-specific SMF/UPF N3 config.
- `scripts/build_srsran_remote.sh`: configures and builds srsRAN on the remote Linux host.
- `scripts/run_srsran_gnb_remote.sh`: runs remote gNB with `zmq` or `usrp` config.

## Bootstrap

```bash
./scripts/bootstrap_srsran_local.sh
git status --short external/srsRAN
```

The bootstrap script removes the nested upstream `.git` directory. This is
required because the current repository, not an srsRAN submodule, should track
local srsRAN source edits.

## Remote Sync

Push local source and integration files to the remote host:

```bash
./scripts/sync_srsran_integration.sh
./scripts/sync_srsran_to_remote.sh
```

If code was edited on the remote host, pull it back before continuing local
work:

```bash
./scripts/sync_srsran_from_remote.sh
```

Use one direction per editing session. Running `push` after remote-only edits
will overwrite the remote source tree.

## Build

```bash
./scripts/build_srsran_remote.sh
```

The build is expected to run on Linux. The local macOS workspace is used for
editing, version control, and configuration management.

The default remote build is ZMQ/no-USRP:

```bash
SRSRAN_CMAKE_ARGS="-DENABLE_UHD=OFF -DENABLE_ZEROMQ=ON -DENABLE_EXPORT=ON -DENABLE_BACKWARD=OFF -DENABLE_WERROR=OFF"
```

This avoids installing UHD/GNURadio on the current server. Enable UHD later
only after the remote root filesystem has enough free space.

## free5GC Config

The current AMF DSMF-trigger config already exposes N2 on `10.0.0.197`:

```yaml
configuration:
  ngapIpList:
    - 10.0.0.197
```

For gNB user-plane traffic, UPF N3 and SMF N3 endpoint should also use a
non-loopback address reachable by the gNB. This repository keeps dedicated
free5GC configs for that mode:

```bash
config/upfcfg.srsran.yaml
config/smfcfg.srsran.yaml
```

`run.sh` supports `SMF_CONFIG_PATH` and `UPF_CONFIG_PATH`, so the default
loopback configs remain available for non-RAN tests.

## Run Order

1. Start free5GC with the DSMF trigger AMF config and srsRAN N3 config:

```bash
./scripts/run_free5gc_srsran_remote.sh
```

2. Start srsRAN gNB:

```bash
./scripts/run_srsran_gnb_remote.sh zmq
```

3. Start a compatible software UE or later a USRP/COTS UE setup.
4. Watch AMF logs for `Handle Registration Request`, `Dispatch DSMF trigger`,
and `Triggered DSMF data transfer`.

## Current Status

Remote ZMQ/no-USRP build is verified on `chensb@172.27.33.78`.

Smoke test result:

- `external/srsRAN/build/apps/gnb/gnb --version` reports `srsRAN 5G gNB version 25.10.0 (fa995e6)`.
- `gnb_zmq.yaml` connects to free5GC AMF on `10.0.0.197:38412`.
- AMF log shows `Handle NGSetupRequest` and `Send NG-Setup response`.

The current server root filesystem is tight on space, so USRP/UHD support is
intentionally disabled for now.
