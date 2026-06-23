// SPDX-License-Identifier: GPL-2.0-only

// Package device reads a 3Dconnexion SpaceMouse and emits host-neutral 6-DOF motion.
//
// It is the only part of the add-in that touches platform/hardware APIs; everything
// above it (navigate, bridge) consumes the neutral [MotionSample] and is therefore
// platform-free and unit-testable. The concrete reader is chosen per OS at build time
// (device_linux.go / device_windows.go / device_darwin.go), with device_nodevice.go a
// no-op fallback so the pure layers build and test on any runner (CGO_ENABLED=0).
package device

import "time"

// MotionSample is one frame of SpaceMouse motion in a normalized, platform-neutral form.
// The six axes are the puck's instantaneous displacement, each clamped to [-1, 1] where
// ±1 is the device's mechanical full-scale; a centred (released) puck reads all zeros.
// This is the pivot type between the OS reader and the camera math (cf. the exporters'
// host-neutral IR): the reader's whole job is producing it, navigate's whole job is
// consuming it.
type MotionSample struct {
	// Translation axes (push/pull/slide the cap). Tz is the push/pull (zoom) axis.
	Tx, Ty, Tz float64
	// Rotation axes (tilt/spin/twist the cap). Rx/Ry orbit, Rz rolls.
	Rx, Ry, Rz float64
	// Buttons is the device's button bitmask at the time of this sample.
	Buttons uint32
	// Period is the time the device reports this sample covers (0 if unknown); the
	// camera math scales motion by it so the feel is frame-rate independent.
	Period time.Duration
}

// IsZero reports whether every axis is at rest (no translation, no rotation). The reader
// emits a rest sample once when motion stops; the engine uses it to end a gesture.
func (s MotionSample) IsZero() bool {
	return s.Tx == 0 && s.Ty == 0 && s.Tz == 0 && s.Rx == 0 && s.Ry == 0 && s.Rz == 0
}

// Device is an open SpaceMouse: a stream of motion samples until it is closed. A reader
// pushes samples (including a single rest sample when the puck recenters) onto Samples
// and closes the channel when the device goes away or Close is called.
type Device interface {
	// Samples is the live motion stream; it is closed when the device is done.
	Samples() <-chan MotionSample
	// Close stops the reader and releases the OS handle. Idempotent.
	Close() error
}

// Opener opens the platform SpaceMouse. The bridge takes one (rather than calling Open
// directly) so tests can inject a fake device. [Open] is the production opener.
type Opener func() (Device, error)
