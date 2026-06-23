# Oblikovati SpaceMouse

3Dconnexion **SpaceMouse** support for [Oblikovati](https://oblikovati.org): move the
puck and the viewport camera orbits, pans, zooms and rolls — fluid one-handed navigation
while your other hand keeps modelling.

It ships as an **add-in**: a small `c-shared` library the host loads at startup. It reads
the device's six axes natively (per platform), turns them into a camera move in pure Go,
and applies it through the public API (`view.setCamera`). It links only the Apache-2.0
[`oblikovati.org/api`](https://github.com/Oblikovati/Oblikovati.API) contract and never
the GPL host.

## How it works

```
SpaceMouse ──device/──▶ MotionSample ──navigate/──▶ camera frame ──bridge/──▶ host (view.setCamera)
            (per-OS)     (6 axes, neutral)  (pure Go math)        (API client)
```

- **device/** — the only hardware-aware layer. Linux reads `/dev/hidraw*` directly;
  macOS uses the system `3DconnexionClient` framework; Windows uses the Win32 HID API.
  All three normalize to a neutral `MotionSample` (3 translation + 3 rotation axes).
- **navigate/** — pure, deterministic camera math (orbit/pan/zoom/roll) with configurable
  sensitivity, axis inversion and deadzone. Fully unit-tested, no hardware needed.
- **bridge/** — opens the device, runs the read loop, and pushes camera frames to the host;
  registers the ribbon controls.

## Install

1. Download the release bundle for your OS, or `make install` from a checkout (copies the
   library + `manifest.json` into the host's add-ins folder).
2. **Linux only:** install the udev rule from `deploy/` so the device is readable without
   root, then replug the SpaceMouse (see [`docs/`](docs/)).
3. Start Oblikovati — the **SpaceMouse** controls appear on the ribbon.

## Build & test

```sh
make test          # pure-Go camera math + bridge (no hardware, CGO off)
make build         # the c-shared library for the current OS
make install       # build + copy into ../Oblikovati/head/addins
```

## Status

The pure camera math and the Linux reader are exercised in CI; the macOS and Windows
device bindings are doc-faithful and build-verified but **need verification on hardware**.

GPL-2.0-only. The downloaded 3Dconnexion SDKs are reference only and are **not** vendored
into this repository.
