// SPDX-License-Identifier: GPL-2.0-only

// Package navigate turns SpaceMouse motion into camera moves. It is pure: given the
// current look-at [Frame] and one [device.MotionSample], [Apply] returns the next frame.
// No hardware, no host, no I/O — so it is deterministic and fully unit-tested, and the
// bridge can drive it from a live device or a test can drive it from a fixed sample list.
//
// The model is velocity control (the authentic SpaceMouse feel): a held deflection is
// reported every frame, and each frame moves the camera by deflection × speed × dt. So a
// steady push pans/zooms/orbits continuously, and the motion is frame-rate independent.
package navigate

import (
	"time"

	"oblikovati.org/api/types"
)

// Frame is a look-at camera: where the eye is, what it looks at, which way is up, and the
// vertical field of view (radians). It mirrors the host's camera (wire.CameraView); the
// bridge converts at the boundary so this package stays free of the wire types. Lengths
// are in database units (cm), like every geometric quantity crossing the API.
type Frame struct {
	Eye    types.Point
	Target types.Point
	Up     types.Vector
	FOV    float64
}

// Config tunes how device deflection maps to camera motion. Speeds are "per second at
// full (±1) deflection": the translation speeds are a fraction of the eye→target distance
// (so the feel is the same whether zoomed in or out), the rotation speeds are radians.
// The invert flags flip a whole motion (set on hardware to taste); Deadzone drops the
// small idle jitter a centred puck still reports.
type Config struct {
	PanSpeed   float64 // fraction of eye→target distance panned per second at full deflection
	ZoomSpeed  float64 // fraction of distance dollied per second at full deflection
	OrbitSpeed float64 // radians orbited per second at full deflection
	RollSpeed  float64 // radians rolled per second at full deflection

	Deadzone        float64 // axis |value| at or below this is treated as zero
	MinDistanceFrac float64 // dolly never reduces the eye→target distance below this fraction of it

	InvertPan   bool
	InvertZoom  bool
	InvertOrbit bool
	InvertRoll  bool

	// DefaultPeriod is the time step used when a sample reports an unknown Period (0).
	DefaultPeriod time.Duration
}

// DefaultConfig is a sensible CAD object-inspection tuning. Speeds are moderate, a small
// deadzone removes idle jitter, and dolly stops just short of the target.
func DefaultConfig() Config {
	return Config{
		PanSpeed:        0.9,
		ZoomSpeed:       1.2,
		OrbitSpeed:      2.2,
		RollSpeed:       1.6,
		Deadzone:        0.02,
		MinDistanceFrac: 0.02,
		DefaultPeriod:   time.Second / 60,
	}
}

// dt is the time step for a sample, falling back to DefaultPeriod when the device does not
// report one.
func (c Config) dt(period time.Duration) float64 {
	if period > 0 {
		return period.Seconds()
	}
	return c.DefaultPeriod.Seconds()
}

// sign returns -1 when inverted, else +1.
func sign(inverted bool) float64 {
	if inverted {
		return -1
	}
	return 1
}

// deadzoned drops an axis value at or below the deadzone (idle jitter) and otherwise
// passes it through unchanged.
func (c Config) deadzoned(v float64) float64 {
	if v < 0 {
		if -v <= c.Deadzone {
			return 0
		}
		return v
	}
	if v <= c.Deadzone {
		return 0
	}
	return v
}
