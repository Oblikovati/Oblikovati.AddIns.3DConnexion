# SpaceMouse setup & internals

How to install the Oblikovati SpaceMouse add-in, get the device readable on each OS, and a
map of how it works.

## Install the add-in

The add-in is one `c-shared` library plus `manifest.json`.

- **From the catalogue (recommended):** the host's add-in browser
  (addins.oblikovati.org) lists it; install and restart Oblikovati.
- **From a release:** download the bundle for your OS from the GitHub releases and unzip
  the library + `manifest.json` into the host's add-ins folder.
- **From a checkout:** `make install` builds the library and copies it (with the manifest)
  into `../Oblikovati/head/addins`.

On start you get a **SpaceMouse** tab on the part ribbon with three controls:

| Control | Action |
| --- | --- |
| **SpaceMouse** (toggle) | enable / disable navigation |
| **Home View** | jump to a fitted isometric view |
| **Sensitivity** | cycle speed: Low → Medium → High |

## Per-OS device access

### Linux — udev rule (required)

The add-in reads the device from `/dev/hidraw*`, which is root-only by default. Install the
shipped rule so your user can read it:

```sh
sudo cp deploy/99-spacemouse.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules && sudo udevadm trigger
```

Then unplug and replug the SpaceMouse. No 3Dconnexion driver, X11 or Wayland session is
needed — the add-in talks to the raw HID device.

### macOS — 3Dconnexion driver

Install the official **3DxWare / 3Dconnexion** driver (it provides the
`3DconnexionClient` framework). The add-in loads that framework at runtime; without it,
navigation is simply inactive (the add-in still loads).

### Windows — driver optional

The add-in reads the device through the standard Win32 HID interface, so it works with the
in-box HID driver. Installing 3DxWare is optional (useful for the device's own button
customization, which this add-in does not manage).

## How it works

```
SpaceMouse ─device/─▶ MotionSample ─navigate/─▶ camera frame ─bridge/─▶ host (view.setCamera)
            per-OS     6 neutral axes  pure math               API client
```

- **device/** — the only hardware-aware layer. Linux: `/dev/hidraw` raw reports; Windows:
  Win32 HID (`hid.dll`/`setupapi.dll`); macOS: the `3DconnexionClient` framework. Each
  normalizes to a neutral `MotionSample` (3 translation + 3 rotation axes, −1..1).
- **navigate/** — pure camera math. Velocity control: each frame moves the camera by
  `deflection × speed × dt`, so a held push pans/zooms/orbits continuously and the feel is
  frame-rate independent. Pan/zoom scale with the eye→target distance; dolly clamps short
  of the target; roll turns the up vector.
- **bridge/** — opens the device and runs the read loop: at a gesture's start it reads the
  camera once (`view.getCamera`), then integrates each sample locally and pushes
  `view.setCamera`, re-syncing at rest. Also registers the ribbon controls.

### Default axis mapping

| Puck motion | Camera |
| --- | --- |
| slide (Tx, Ty) | pan |
| push / pull (Tz) | zoom (dolly) |
| tilt fore/aft (Rx) | orbit pitch |
| twist (Rz) | orbit yaw |
| tilt left/right (Ry) | roll |

Each axis has an invert flag and a shared sensitivity (the ribbon's Sensitivity cycle).

## Verification status

The pure layers — the camera math and the HID/identity decode — are unit-tested in CI
(>80% coverage). The per-OS device readers are build-verified on every platform but need
**confirmation on hardware**: the feel and the exact axis signs are tuned with a physical
puck. The downloaded 3Dconnexion SDKs are reference only and are not vendored into this
repository.
