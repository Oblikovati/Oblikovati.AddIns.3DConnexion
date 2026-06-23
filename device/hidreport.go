// SPDX-License-Identifier: GPL-2.0-only

package device

// SpaceMouse HID input-report decoding — pure, so it is unit-tested against captured
// report bytes without a device. 3Dconnexion pucks report motion as signed 16-bit
// little-endian axis values in a handful of report ids:
//
//	id 1: translation  — Tx,Ty,Tz (and, on combined-report firmware, Rx,Ry,Rz appended)
//	id 2: rotation     — Rx,Ry,Rz
//	id 3: buttons      — a little-endian bitmask
//
// A reader keeps one running MotionSample and folds each report into it with decodeInto,
// so a translation report and the rotation report that follows compose into one 6-axis
// state; when the puck recenters it reports zeros and the state returns to rest.

// fullScale is the axis magnitude treated as full (±1) deflection. SpaceMouse firmware
// reports motion in roughly ±350; the per-axis value is normalized by this and clamped, so
// over-range firmware just saturates. Sensitivity presets scale the result downstream.
const fullScale = 350.0

const (
	reportTranslation = 1
	reportRotation    = 2
	reportButtons     = 3
)

// decodeInto folds one HID input report into s, preserving the axes the report does not
// carry, and reports whether the report was a recognised motion/button report. It never
// reads out of bounds: a short report for its id is ignored (returns false).
func decodeInto(s *MotionSample, report []byte) bool {
	if len(report) < 1 {
		return false
	}
	switch report[0] {
	case reportTranslation:
		return decodeTranslation(s, report)
	case reportRotation:
		if len(report) >= 7 {
			s.Rx, s.Ry, s.Rz = axis(report[1:]), axis(report[3:]), axis(report[5:])
			return true
		}
	case reportButtons:
		s.Buttons = buttonMask(report[1:])
		return true
	}
	return false
}

// decodeTranslation reads the translation report, including the rotation axes when the
// firmware appends them in one combined report (≥13 bytes).
func decodeTranslation(s *MotionSample, report []byte) bool {
	if len(report) < 7 {
		return false
	}
	s.Tx, s.Ty, s.Tz = axis(report[1:]), axis(report[3:]), axis(report[5:])
	if len(report) >= 13 {
		s.Rx, s.Ry, s.Rz = axis(report[7:]), axis(report[9:]), axis(report[11:])
	}
	return true
}

// axis decodes one signed 16-bit little-endian value at b[0:2] and normalizes it to
// [-1, 1] (saturating beyond full scale).
func axis(b []byte) float64 {
	raw := int16(uint16(b[0]) | uint16(b[1])<<8)
	v := float64(raw) / fullScale
	if v > 1 {
		return 1
	}
	if v < -1 {
		return -1
	}
	return v
}

// buttonMask reads up to four little-endian bytes into a button bitmask.
func buttonMask(b []byte) uint32 {
	var m uint32
	for i := 0; i < len(b) && i < 4; i++ {
		m |= uint32(b[i]) << (8 * i)
	}
	return m
}
