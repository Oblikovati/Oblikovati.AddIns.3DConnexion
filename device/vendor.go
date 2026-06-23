// SPDX-License-Identifier: GPL-2.0-only

package device

import "strings"

// SpaceMouse USB identity matching. This is pure (a HID_ID string in, a bool out) so it is
// unit-tested in CI; the Linux reader does only the sysfs I/O around it.
//
// The current 3Dconnexion vendor id (256f) is exclusively theirs, so any product matches.
// The LEGACY id older pucks shipped under is Logitech's (046d), which is shared with every
// Logitech mouse/keyboard/receiver — so it counts only for the 3Dconnexion product range
// (0xc6xx). Matching 046d on vendor alone would wrongly grab e.g. a "Logitech USB
// Receiver" (046d:c548).
const (
	vendor3Dconnexion = "256f"
	vendorLogitech    = "046d"
	logitech3DxPrefix = "c6" // 3Dconnexion SpaceMouse/SpaceNavigator product range under 046d
)

// isSpaceMouseID reports whether a sysfs HID_ID (bus:vendor:product, hex, e.g.
// "0003:0000256F:0000C62E") identifies a 3Dconnexion SpaceMouse.
func isSpaceMouseID(hidID string) bool {
	parts := strings.Split(hidID, ":")
	if len(parts) != 3 {
		return false
	}
	vendor := low16(parts[1])
	product := low16(parts[2])
	switch vendor {
	case vendor3Dconnexion:
		return true
	case vendorLogitech:
		return strings.HasPrefix(product, logitech3DxPrefix)
	default:
		return false
	}
}

// low16 normalizes a hex field to its lowercase low-16-bit (4-digit) form.
func low16(hex string) string {
	h := strings.ToLower(strings.TrimSpace(hex))
	if len(h) > 4 {
		h = h[len(h)-4:]
	}
	return h
}
