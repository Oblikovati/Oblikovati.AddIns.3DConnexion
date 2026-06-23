// SPDX-License-Identifier: GPL-2.0-only

package device

import (
	"math"
	"testing"
)

// le encodes a signed value as a little-endian int16 pair, like a SpaceMouse axis field.
func le(v int16) (byte, byte) { return byte(uint16(v)), byte(uint16(v) >> 8) }

func approx(a, b float64) bool { return math.Abs(a-b) <= 1e-9 }

func TestDecodeTranslation(t *testing.T) {
	xl, xh := le(350)  // full +X
	yl, yh := le(-175) // half -Y
	zl, zh := le(0)
	var s MotionSample
	if !decodeInto(&s, []byte{reportTranslation, xl, xh, yl, yh, zl, zh}) {
		t.Fatal("translation report not decoded")
	}
	if !approx(s.Tx, 1) || !approx(s.Ty, -0.5) || !approx(s.Tz, 0) {
		t.Fatalf("translation axes: %+v", s)
	}
	if s.Rx != 0 || s.Ry != 0 || s.Rz != 0 {
		t.Fatalf("translation report set rotation axes: %+v", s)
	}
}

func TestDecodeRotation(t *testing.T) {
	al, ah := le(-350)
	bl, bh := le(350)
	cl, ch := le(0)
	var s MotionSample
	if !decodeInto(&s, []byte{reportRotation, al, ah, bl, bh, cl, ch}) {
		t.Fatal("rotation report not decoded")
	}
	if !approx(s.Rx, -1) || !approx(s.Ry, 1) || !approx(s.Rz, 0) {
		t.Fatalf("rotation axes: %+v", s)
	}
}

func TestDecodeCombinedReport(t *testing.T) {
	// Firmware that packs all six axes into one report id 1 (≥13 bytes).
	xl, xh := le(350)
	rl, rh := le(350)
	zero := []byte{0, 0}
	report := []byte{reportTranslation, xl, xh}
	report = append(report, zero...) // Ty
	report = append(report, zero...) // Tz
	report = append(report, rl, rh)  // Rx
	report = append(report, zero...) // Ry
	report = append(report, zero...) // Rz
	var s MotionSample
	if !decodeInto(&s, report) {
		t.Fatal("combined report not decoded")
	}
	if !approx(s.Tx, 1) || !approx(s.Rx, 1) {
		t.Fatalf("combined axes: %+v", s)
	}
}

func TestDecodePreservesOtherAxes(t *testing.T) {
	var s MotionSample
	tl, th := le(350)
	decodeInto(&s, []byte{reportTranslation, tl, th, 0, 0, 0, 0}) // sets Tx
	rl, rh := le(350)
	decodeInto(&s, []byte{reportRotation, rl, rh, 0, 0, 0, 0}) // sets Rx, must keep Tx
	if !approx(s.Tx, 1) || !approx(s.Rx, 1) {
		t.Fatalf("folding rotation clobbered translation: %+v", s)
	}
}

func TestDecodeButtons(t *testing.T) {
	var s MotionSample
	if !decodeInto(&s, []byte{reportButtons, 0x05, 0x01}) {
		t.Fatal("button report not decoded")
	}
	if s.Buttons != 0x0105 {
		t.Fatalf("button mask: got %#x want 0x0105", s.Buttons)
	}
}

func TestDecodeSaturatesBeyondFullScale(t *testing.T) {
	var s MotionSample
	hi, hih := le(1000)
	lo, loh := le(-1000)
	decodeInto(&s, []byte{reportTranslation, hi, hih, lo, loh, 0, 0})
	if s.Tx != 1 || s.Ty != -1 {
		t.Fatalf("over-range axes did not saturate: %+v", s)
	}
}

func TestDecodeRejectsShortAndUnknown(t *testing.T) {
	var s MotionSample
	if decodeInto(&s, nil) {
		t.Error("empty report decoded")
	}
	if decodeInto(&s, []byte{reportTranslation, 1}) { // too short for translation
		t.Error("short translation report decoded")
	}
	if decodeInto(&s, []byte{0x42, 1, 2, 3}) { // unknown report id
		t.Error("unknown report id decoded")
	}
	if !s.IsZero() {
		t.Fatalf("rejected reports mutated state: %+v", s)
	}
}
