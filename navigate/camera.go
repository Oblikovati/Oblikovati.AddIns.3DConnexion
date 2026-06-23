// SPDX-License-Identifier: GPL-2.0-only

package navigate

import (
	"math"

	"oblikovati.org/api/types"
	"oblikovati.org/spacemouse/device"
)

// Apply returns the camera frame after one motion sample. The order is orbit (around the
// target) → roll → pan (constant distance) → dolly (toward/away from the target). A
// degenerate frame (eye on target, or up parallel to the view direction) or an all-zero
// sample is returned unchanged.
func Apply(cam Frame, s device.MotionSample, cfg Config) Frame {
	b, ok := makeBasis(cam)
	if !ok {
		return cam
	}
	m := cfg.motion(s)
	if m.zero() {
		return cam
	}
	dist := cam.Eye.DistanceTo(cam.Target)

	eye, target, up := cam.Eye, cam.Target, cam.Up
	if m.pitch != 0 || m.yaw != 0 {
		eye, up = orbit(eye, target, up, b, m.pitch, m.yaw)
	}
	if m.roll != 0 {
		up = rotateAbout(up, b.f, m.roll)
	}
	if m.panX != 0 || m.panY != 0 {
		offset := b.r.Scale(m.panX * dist).Add(b.u.Scale(m.panY * dist))
		eye, target = eye.TranslateBy(offset), target.TranslateBy(offset)
	}
	if m.dolly != 0 {
		eye = dolly(eye, target, m.dolly*dist, cfg.MinDistanceFrac)
	}
	return Frame{Eye: eye, Target: target, Up: up, FOV: cam.FOV}
}

// motion is one sample reduced to camera-space quantities: pan and dolly are fractions of
// the eye→target distance, the angles are radians. Deadzone, inversion, per-axis speed and
// the time step are all baked in here, so Apply is pure geometry.
type motion struct {
	panX, panY, dolly float64
	pitch, yaw, roll  float64
}

func (m motion) zero() bool {
	return m.panX == 0 && m.panY == 0 && m.dolly == 0 &&
		m.pitch == 0 && m.yaw == 0 && m.roll == 0
}

// motion maps a raw sample to camera-space quantities. The axis assignment is the default
// SpaceMouse-flat layout: slide → pan, push/pull → dolly, tilt fore/aft → pitch, twist →
// yaw, tilt left/right → roll. Signs are tuned on hardware via the invert flags.
func (c Config) motion(s device.MotionSample) motion {
	dt := c.dt(s.Period)
	pan := sign(c.InvertPan) * c.PanSpeed * dt
	return motion{
		panX:  c.deadzoned(s.Tx) * pan,
		panY:  c.deadzoned(s.Ty) * pan,
		dolly: c.deadzoned(s.Tz) * sign(c.InvertZoom) * c.ZoomSpeed * dt,
		pitch: c.deadzoned(s.Rx) * sign(c.InvertOrbit) * c.OrbitSpeed * dt,
		yaw:   c.deadzoned(s.Rz) * sign(c.InvertOrbit) * c.OrbitSpeed * dt,
		roll:  c.deadzoned(s.Ry) * sign(c.InvertRoll) * c.RollSpeed * dt,
	}
}

// basis is the camera's orthonormal frame: unit forward (eye→target), right and up.
type basis struct{ f, r, u types.Vector }

// makeBasis builds the camera basis, reporting false for a degenerate frame (eye on the
// target, or up parallel to the view direction) where no stable right/up axis exists.
func makeBasis(cam Frame) (basis, bool) {
	fwd := cam.Eye.VectorTo(cam.Target)
	d := fwd.Length()
	if d == 0 {
		return basis{}, false
	}
	f := fwd.Scale(1 / d)
	r := f.Cross(cam.Up)
	rl := r.Length()
	if rl == 0 {
		return basis{}, false
	}
	r = r.Scale(1 / rl)
	return basis{f: f, r: r, u: r.Cross(f)}, true
}

// orbit rotates the eye (and the up vector) around the target: pitch about the right axis,
// then yaw about the up axis. The target and the eye→target distance are preserved.
func orbit(eye, target types.Point, up types.Vector, b basis, pitch, yaw float64) (types.Point, types.Vector) {
	ev := target.VectorTo(eye) // target→eye, rotated in place
	if pitch != 0 {
		ev = rotateAbout(ev, b.r, pitch)
		up = rotateAbout(up, b.r, pitch)
	}
	if yaw != 0 {
		ev = rotateAbout(ev, b.u, yaw)
		up = rotateAbout(up, b.u, yaw)
	}
	return target.TranslateBy(ev), up
}

// dolly moves the eye along the view line by move (positive = toward the target), never
// closer than minFrac of the current distance so the eye never reaches or crosses the
// target.
func dolly(eye, target types.Point, move, minFrac float64) types.Point {
	d := eye.DistanceTo(target)
	if d == 0 {
		return eye
	}
	dir := eye.VectorTo(target).Scale(1 / d) // unit eye→target
	newD := d - move
	if floor := d * minFrac; newD < floor {
		newD = floor
	}
	return target.TranslateBy(dir.Scale(-newD)) // eye = target - dir·newD
}

// rotateAbout rotates v by angle radians about the unit axis k (Rodrigues' rotation):
// v·cosθ + (k×v)·sinθ + k·(k·v)(1−cosθ).
func rotateAbout(v, k types.Vector, angle float64) types.Vector {
	cos, sin := math.Cos(angle), math.Sin(angle)
	return v.Scale(cos).
		Add(k.Cross(v).Scale(sin)).
		Add(k.Scale(k.Dot(v) * (1 - cos)))
}
