// The oblikovati-spacemouse add-in: a c-shared library (.so/.dll/.dylib) loaded by the
// host at runtime, giving 3Dconnexion SpaceMouse devices control of the viewport camera.
// It reads the device's 6-DOF motion natively (per-platform, behind the device/ build
// tags), computes the new look-at camera frame in pure Go (navigate/), and pushes it to
// the host over the Apache-2.0 API (view.setCamera). Its own module so the device deps
// stay independent of the host — the runtime boundary is the C ABI, not Go (see
// include/oblikovati_addin.h).
//
// The SHIPPED library links only the Apache-2.0 contract (oblikovati.org/api). The
// require on the GPL application module (oblikovati) is TEST-SCOPE ONLY — the
// add-in↔real-host integration tests drive the live router/model. Both modules are
// sibling repos resolved by the go.work workspace at this repo's root (no committed
// replace); CI injects the equivalent replaces via .github/actions/siblings.
module oblikovati.org/spacemouse

go 1.24.0

require oblikovati.org/api v0.102.0
