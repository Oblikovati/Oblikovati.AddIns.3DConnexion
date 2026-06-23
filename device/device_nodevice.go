// SPDX-License-Identifier: GPL-2.0-only

//go:build nodevice || !linux

package device

// The no-op opener: a Device that never produces motion. It builds with `-tags nodevice`
// (the CGO_ENABLED=0 CI jobs that exercise only the pure layers) and on any OS without a
// real reader yet. The per-OS readers carry `//go:build GOOS && !nodevice`, so exactly one
// Open is ever linked.
//
// As macOS and Windows readers land (M4) this negation widens to
// `nodevice || (!linux && !windows && !darwin)`.

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
