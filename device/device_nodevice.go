// SPDX-License-Identifier: GPL-2.0-only

package device

// The no-op opener: a Device that never produces motion. It builds everywhere so the
// pure layers (navigate, bridge) compile and test with CGO_ENABLED=0 on any runner, and
// it stands in on platforms without a SpaceMouse reader.
//
// Build-tag scheme: while only this fallback exists it carries no constraint (the default
// on every OS). When a per-OS reader lands it gets `//go:build GOOS && !nodevice` and this
// file gains the matching negation (`//go:build nodevice || (!linux && !windows &&
// !darwin)`), so exactly one Open is ever linked and `-tags nodevice` forces the no-op
// (used by the CGO_ENABLED=0 CI jobs that exercise only the pure layers).

// Open returns a device that emits no samples. It never errors: a host with no SpaceMouse
// support compiled in still loads the add-in and its ribbon commands; navigation is just
// inert.
func Open() (Device, error) { return noDevice{}, nil }

type noDevice struct{}

func (noDevice) Samples() <-chan MotionSample {
	ch := make(chan MotionSample)
	close(ch) // already done: ranging over it returns immediately
	return ch
}

func (noDevice) Close() error { return nil }
