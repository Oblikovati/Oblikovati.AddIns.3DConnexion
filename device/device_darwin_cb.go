// SPDX-License-Identifier: GPL-2.0-only

//go:build darwin && !nodevice

package device

/*
#include <stdint.h>
*/
import "C"

// goSpaceMouseSample is the cgo callback the framework's C message handler (sm_message in
// device_darwin.go) invokes with one device state. It normalizes the raw axes and feeds the
// active device's stream. It lives in its own file because a cgo file with an //export may
// not also define C functions in its preamble (the C definitions are in device_darwin.go).
//
//export goSpaceMouseSample
func goSpaceMouseSample(tx, ty, tz, rx, ry, rz C.int16_t, buttons C.uint32_t) {
	activeMu.Lock()
	d := active
	activeMu.Unlock()
	if d == nil {
		return
	}
	d.emit(MotionSample{
		Tx: normAxis(int16(tx)), Ty: normAxis(int16(ty)), Tz: normAxis(int16(tz)),
		Rx: normAxis(int16(rx)), Ry: normAxis(int16(ry)), Rz: normAxis(int16(rz)),
		Buttons: uint32(buttons),
	})
}
