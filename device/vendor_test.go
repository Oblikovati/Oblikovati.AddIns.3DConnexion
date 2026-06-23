// SPDX-License-Identifier: GPL-2.0-only

package device

import "testing"

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
