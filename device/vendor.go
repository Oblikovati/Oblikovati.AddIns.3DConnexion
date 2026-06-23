// SPDX-License-Identifier: GPL-2.0-only

package device

import (
	"strconv"
	"strings"
)

// SpaceMouse USB identity matching. This is pure (vendor/product in, a bool out) so it is
// unit-tested in CI; the per-OS readers do only the platform I/O around it.
//
// The current 3Dconnexion vendor id (256f) is exclusively theirs, so any product matches.
// The LEGACY id older pucks shipped under is Logitech's (046d), which is shared with every
// Logitech mouse/keyboard/receiver — so it counts only for the 3Dconnexion product range
// (0xc6xx). Matching 046d on vendor alone would wrongly grab e.g. a "Logitech USB
// Receiver" (046d:c548).
const (
	vid3Dconnexion = 0x256f
	vidLogitech    = 0x046d
	logitech3DxLo  = 0xc600 // 3Dconnexion SpaceMouse/SpaceNavigator product range under 046d
	logitech3DxHi  = 0xc6ff
)

// isSpaceMouseVIDPID reports whether a USB vendor/product id pair is a 3Dconnexion
// SpaceMouse. Used directly by the Windows reader (raw VID/PID) and via isSpaceMouseID by
// the Linux reader (sysfs HID_ID string).
func isSpaceMouseVIDPID(vid, pid uint16) bool {
	switch vid {
	case vid3Dconnexion:
		return true
	case vidLogitech:
		return pid >= logitech3DxLo && pid <= logitech3DxHi
	default:
		return false
	}
}

// isSpaceMouseID reports whether a sysfs HID_ID (bus:vendor:product, hex, e.g.
// "0003:0000256F:0000C62E") identifies a 3Dconnexion SpaceMouse.
func isSpaceMouseID(hidID string) bool {
	parts := strings.Split(hidID, ":")
	if len(parts) != 3 {
		return false
	}
	vid, ok1 := parseHex16(parts[1])
	pid, ok2 := parseHex16(parts[2])
	if !ok1 || !ok2 {
		return false
	}
	return isSpaceMouseVIDPID(vid, pid)
}

// parseHex16 parses a hex field to its low-16-bit value (USB ids are 16-bit; the sysfs
// field is zero-padded to 8 hex digits).
func parseHex16(hex string) (uint16, bool) {
	n, err := strconv.ParseUint(strings.TrimSpace(hex), 16, 32)
	if err != nil {
		return 0, false
	}
	return uint16(n), true
}
