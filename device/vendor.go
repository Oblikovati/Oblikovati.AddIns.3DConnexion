// SPDX-License-Identifier: GPL-2.0-only

package device

import (
	"strconv"
	"strings"
)

// SpaceMouse USB identity matching. This is pure (vendor/product in, a bool out) so it is
// unit-tested in CI; the per-OS readers do only the platform I/O around it.
//
// Matching is an EXPLICIT allowlist of the actual 6-DOF devices, not a vendor (or vendor
// range) heuristic — because neither vendor id is safe to match broadly:
//   - 0x046d is Logitech's, shared with every Logitech mouse/keyboard/receiver (the
//     pre-2013 3Dconnexion pucks were Logitech-branded and used it);
//   - 0x256f is 3Dconnexion's own, but it also covers their CadMouse line, which are
//     ordinary 2D mice — not 6-DOF pucks — and must NOT be driven as a camera.
// So we list the known SpaceMouse / SpaceNavigator / SpaceExplorer / SpacePilot /
// SpaceBall product ids only. Source: the spacenavd / libspnav device tables.

// id packs a vendor+product into one comparable key.
func id(vid, pid uint16) uint32 { return uint32(vid)<<16 | uint32(pid) }

// spaceMouseDevices is the allowlist of 6-DOF 3Dconnexion devices. CadMouse (256f:c650,
// 256f:c651) is intentionally absent — it is a 2D mouse.
var spaceMouseDevices = map[uint32]struct{}{
	// Legacy, Logitech-branded (vendor 0x046d).
	id(0x046d, 0xc603): {}, // SpaceMouse Plus XT
	id(0x046d, 0xc605): {}, // CADman
	id(0x046d, 0xc606): {}, // SpaceMouse Classic
	id(0x046d, 0xc621): {}, // SpaceBall 5000
	id(0x046d, 0xc623): {}, // SpaceTraveler
	id(0x046d, 0xc625): {}, // SpacePilot
	id(0x046d, 0xc626): {}, // SpaceNavigator
	id(0x046d, 0xc627): {}, // SpaceExplorer
	id(0x046d, 0xc628): {}, // SpaceNavigator for Notebooks
	id(0x046d, 0xc629): {}, // SpacePilot Pro
	id(0x046d, 0xc62b): {}, // SpaceMouse Pro
	id(0x046d, 0xc640): {}, // nulooq

	// Current, 3Dconnexion-branded (vendor 0x256f).
	id(0x256f, 0xc62e): {}, // SpaceMouse Wireless (cabled)
	id(0x256f, 0xc62f): {}, // SpaceMouse Wireless receiver
	id(0x256f, 0xc631): {}, // SpaceMouse Pro Wireless
	id(0x256f, 0xc632): {}, // SpaceMouse Pro Wireless receiver
	id(0x256f, 0xc633): {}, // SpaceMouse Enterprise
	id(0x256f, 0xc635): {}, // SpaceMouse Compact
	id(0x256f, 0xc636): {}, // SpaceMouse Module
	id(0x256f, 0xc638): {}, // SpaceMouse Pro 2
	id(0x256f, 0xc652): {}, // 3Dconnexion Universal Receiver
}

// isSpaceMouseVIDPID reports whether a USB vendor/product id pair is a known 6-DOF
// SpaceMouse. Used directly by the Windows reader (raw VID/PID) and via isSpaceMouseID by
// the Linux reader (sysfs HID_ID string).
func isSpaceMouseVIDPID(vid, pid uint16) bool {
	_, ok := spaceMouseDevices[id(vid, pid)]
	return ok
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
