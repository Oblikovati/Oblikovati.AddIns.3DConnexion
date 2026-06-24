// SPDX-License-Identifier: GPL-2.0-only

package device

import "testing"

func TestIsSpaceMouseVIDPID(t *testing.T) {
	cases := []struct {
		name     string
		vid, pid uint16
		want     bool
	}{
		// Real 6-DOF pucks — must match.
		{"SpaceNavigator (legacy 046d)", 0x046d, 0xc626, true},
		{"SpacePilot Pro (legacy 046d)", 0x046d, 0xc629, true},
		{"SpaceMouse Compact (256f)", 0x256f, 0xc635, true},
		{"SpaceMouse Wireless (256f)", 0x256f, 0xc62e, true},
		{"Universal Receiver (256f)", 0x256f, 0xc652, true},

		// NOT 6-DOF pucks — must be rejected, even on a 3Dconnexion vendor id.
		{"3Dconnexion CadMouse is a 2D mouse", 0x256f, 0xc650, false},
		{"3Dconnexion CadMouse Wireless", 0x256f, 0xc651, false},

		// Ordinary devices that happen to share a vendor id — must be rejected. These are
		// the exact ids on the maintainer's machine (no SpaceMouse attached).
		{"Logitech USB Receiver (user's mouse)", 0x046d, 0xc548, false},
		{"Logitech BRIO webcam", 0x046d, 0x085e, false},
		{"Logitech Unifying receiver", 0x046d, 0xc52b, false},
		{"unrelated vendor in c6xx range", 0x1234, 0xc626, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isSpaceMouseVIDPID(c.vid, c.pid); got != c.want {
				t.Fatalf("isSpaceMouseVIDPID(%#04x, %#04x) = %v, want %v", c.vid, c.pid, got, c.want)
			}
		})
	}
}

func TestIsSpaceMouseID(t *testing.T) {
	cases := []struct {
		name  string
		hidID string
		want  bool
	}{
		{"SpaceMouse Compact", "0003:0000256F:0000C635", true},
		{"SpaceMouse lowercase", "0003:0000256f:0000c62e", true},
		{"SpaceNavigator (legacy)", "0003:0000046D:0000C626", true},
		{"user's Logitech receiver (real sysfs string)", "0003:0000046D:0000C548", false},
		{"CadMouse (2D mouse on 3Dx vendor)", "0003:0000256F:0000C650", false},
		{"unrelated vendor", "0003:00001234:00005678", false},
		{"malformed (two fields)", "0003:0000256F", false},
		{"empty", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isSpaceMouseID(c.hidID); got != c.want {
				t.Fatalf("isSpaceMouseID(%q) = %v, want %v", c.hidID, got, c.want)
			}
		})
	}
}
