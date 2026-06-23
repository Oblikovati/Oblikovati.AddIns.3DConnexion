// SPDX-License-Identifier: GPL-2.0-only

package device

import "testing"

func TestIsSpaceMouseVIDPID(t *testing.T) {
	cases := []struct {
		vid, pid uint16
		want     bool
	}{
		{0x256f, 0xc62e, true},  // current vendor, SpaceMouse Compact
		{0x256f, 0x0000, true},  // current vendor, any product
		{0x046d, 0xc623, true},  // legacy vendor, SpaceNavigator (3Dx range)
		{0x046d, 0xc6ff, true},  // legacy vendor, top of 3Dx range
		{0x046d, 0xc548, false}, // legacy vendor, USB receiver (NOT 3Dx range)
		{0x046d, 0xc52b, false}, // legacy vendor, unifying receiver
		{0x1234, 0xc62e, false}, // unrelated vendor
	}
	for _, c := range cases {
		if got := isSpaceMouseVIDPID(c.vid, c.pid); got != c.want {
			t.Errorf("isSpaceMouseVIDPID(%#04x, %#04x) = %v, want %v", c.vid, c.pid, got, c.want)
		}
	}
}

func TestIsSpaceMouseID(t *testing.T) {
	cases := []struct {
		name  string
		hidID string
		want  bool
	}{
		{"current vendor, any product", "0003:0000256F:0000C62E", true},
		{"current vendor lowercase", "0003:0000256f:0000c635", true},
		{"legacy Logitech vendor, 3Dx product range", "0003:0000046D:0000C623", true},
		{"legacy Logitech vendor, NON-3Dx product (USB receiver)", "0003:0000046D:0000C548", false},
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
