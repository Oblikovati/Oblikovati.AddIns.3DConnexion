// SPDX-License-Identifier: GPL-2.0-only

package navigate

import (
	"math"
	"testing"
	"time"

	"oblikovati.org/api/types"
	"oblikovati.org/spacemouse/device"
)

// A canonical start frame: eye 10cm down +X looking at the origin, +Z up. distance = 10.
func startFrame() Frame {
	return Frame{
		Eye:    types.NewPoint(10, 0, 0),
		Target: types.NewPoint(0, 0, 0),
		Up:     types.NewVector(0, 0, 1),
		FOV:    math.Pi / 4,
	}
}

const eps = 1e-9

func approx(a, b float64) bool { return math.Abs(a-b) <= 1e-6 }

func vlen(v types.Vector) float64 { return v.Length() }

// oneSecond gives a sample a 1s period so "per second" speeds apply once, full strength.
func sample(s device.MotionSample) device.MotionSample {
	s.Period = time.Second
	return s
}

func cfg() Config { return DefaultConfig() }

func TestZeroSampleIsNoOp(t *testing.T) {
	in := startFrame()
	got := Apply(in, sample(device.MotionSample{}), cfg())
	if got != in {
		t.Fatalf("zero sample changed the frame: %+v -> %+v", in, got)
	}
}

func TestDeadzoneDropsJitter(t *testing.T) {
	in := startFrame()
	jitter := sample(device.MotionSample{Tx: 0.01, Ry: 0.01}) // below the 0.02 deadzone
	if got := Apply(in, jitter, cfg()); got != in {
		t.Fatalf("sub-deadzone sample moved the camera: %+v", got)
	}
}

func TestPanMovesEyeAndTargetTogether(t *testing.T) {
	in := startFrame()
	c := cfg()
	got := Apply(in, sample(device.MotionSample{Ty: 1}), c)

	// Distance is preserved (pan is a pure translation of the whole frame).
	if d := got.Eye.DistanceTo(got.Target); !approx(d, 10) {
		t.Fatalf("pan changed distance: got %v want 10", d)
	}
	// Eye and target shift by the same offset.
	de := in.Eye.VectorTo(got.Eye)
	dt := in.Target.VectorTo(got.Target)
	if !approx(de.X, dt.X) || !approx(de.Y, dt.Y) || !approx(de.Z, dt.Z) {
		t.Fatalf("pan offset differs between eye %+v and target %+v", de, dt)
	}
	// Ty pans along the camera up axis (+Z here); magnitude = PanSpeed·dist.
	if !approx(de.Z, c.PanSpeed*10) || math.Abs(de.X) > 1e-6 || math.Abs(de.Y) > 1e-6 {
		t.Fatalf("Ty did not pan along +Z by PanSpeed·dist: %+v", de)
	}
}

func TestDollyZoomsTowardTarget(t *testing.T) {
	in := startFrame()
	c := cfg()
	// Smaller deflection so the move stays away from the min-distance clamp.
	got := Apply(in, sample(device.MotionSample{Tz: 0.5}), c)

	d := got.Eye.DistanceTo(got.Target)
	want := 10 - 0.5*c.ZoomSpeed*10
	if !approx(d, want) {
		t.Fatalf("dolly distance: got %v want %v", d, want)
	}
	// The eye stays on the +X view line (only the distance changes).
	if math.Abs(got.Eye.Y) > 1e-6 || math.Abs(got.Eye.Z) > 1e-6 {
		t.Fatalf("dolly left the view line: %+v", got.Eye)
	}
}

func TestDollyClampsAtMinDistance(t *testing.T) {
	in := startFrame()
	c := cfg()
	c.ZoomSpeed = 100 // try to blow through the target in one step
	got := Apply(in, sample(device.MotionSample{Tz: 1}), c)
	d := got.Eye.DistanceTo(got.Target)
	if !approx(d, 10*c.MinDistanceFrac) {
		t.Fatalf("dolly did not clamp at the min distance: got %v want %v", d, 10*c.MinDistanceFrac)
	}
}

func TestOrbitYawPreservesDistance(t *testing.T) {
	in := startFrame()
	c := cfg()
	got := Apply(in, sample(device.MotionSample{Rz: 1}), c)

	if d := got.Eye.DistanceTo(got.Target); !approx(d, 10) {
		t.Fatalf("orbit changed distance: %v", d)
	}
	// Yaw about the camera up (+Z) rotates the eye in the XY plane by OrbitSpeed radians.
	wantAngle := c.OrbitSpeed
	gotAngle := math.Atan2(got.Eye.Y, got.Eye.X)
	if !approx(math.Abs(gotAngle), wantAngle) {
		t.Fatalf("yaw angle: got %v want ±%v (eye %+v)", gotAngle, wantAngle, got.Eye)
	}
	if math.Abs(got.Eye.Z) > 1e-6 {
		t.Fatalf("yaw moved the eye off the XY plane: %+v", got.Eye)
	}
}

func TestRollRotatesUpOnly(t *testing.T) {
	in := startFrame()
	got := Apply(in, sample(device.MotionSample{Ry: 1}), cfg())

	if got.Eye != in.Eye || got.Target != in.Target {
		t.Fatalf("roll moved eye/target: %+v", got)
	}
	if approx(got.Up.Z, 1) {
		t.Fatalf("roll did not rotate the up vector: %+v", got.Up)
	}
	if l := vlen(got.Up); math.Abs(l-1) > 1e-6 {
		t.Fatalf("roll changed up length: %v", l)
	}
}

func TestInvertFlipsDirection(t *testing.T) {
	in := startFrame()
	c := cfg()
	plain := Apply(in, sample(device.MotionSample{Ty: 1}), c)
	c.InvertPan = true
	inverted := Apply(in, sample(device.MotionSample{Ty: 1}), c)

	dz := plain.Eye.Z - in.Eye.Z
	idz := inverted.Eye.Z - in.Eye.Z
	if !approx(dz, -idz) || math.Abs(dz) < eps {
		t.Fatalf("invert did not flip pan: %v vs %v", dz, idz)
	}
}

func TestPeriodScalesMotion(t *testing.T) {
	in := startFrame()
	c := cfg()
	half := Apply(in, device.MotionSample{Ty: 1, Period: time.Second / 2}, c)
	full := Apply(in, device.MotionSample{Ty: 1, Period: time.Second}, c)

	dh := in.Eye.Z - half.Eye.Z
	df := in.Eye.Z - full.Eye.Z
	if !approx(df, 2*dh) {
		t.Fatalf("motion not proportional to period: half=%v full=%v", dh, df)
	}
}

func TestDegenerateFrameUnchanged(t *testing.T) {
	// Eye on target: no view direction.
	bad := Frame{Eye: types.NewPoint(0, 0, 0), Target: types.NewPoint(0, 0, 0), Up: types.NewVector(0, 0, 1)}
	if got := Apply(bad, sample(device.MotionSample{Tx: 1}), cfg()); got != bad {
		t.Fatalf("degenerate frame changed: %+v", got)
	}
	// Up parallel to the view direction: no stable right axis.
	par := Frame{Eye: types.NewPoint(10, 0, 0), Target: types.NewPoint(0, 0, 0), Up: types.NewVector(1, 0, 0)}
	if got := Apply(par, sample(device.MotionSample{Rz: 1}), cfg()); got != par {
		t.Fatalf("parallel-up frame changed: %+v", got)
	}
}
