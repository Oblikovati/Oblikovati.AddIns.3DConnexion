// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"sync"

	"oblikovati.org/api/client"
	"oblikovati.org/spacemouse/device"
)

// HostCaller is the transport the engine talks to the host through — exactly the
// api/client Caller contract, supplied by the cgo shell at Activate (or a fake in tests).
// Keeping it an interface here keeps this package cgo-free and testable.
type HostCaller interface {
	Call(method string, req []byte) ([]byte, error)
}

// Engine drives the viewport camera from a 3Dconnexion SpaceMouse: it opens the device,
// turns its 6-DOF motion into camera frames (package navigate), and pushes them to the
// host over the API. It owns the device read loop; the host owns the camera.
type Engine struct {
	host HostCaller
	api  *client.Client
	open device.Opener

	mu   sync.Mutex
	dev  device.Device // open device, nil when not running
	done chan struct{} // closed to stop the read loop
}

// NewEngine binds the engine to the host transport and the device opener (injected so
// tests can supply a fake device).
func NewEngine(host HostCaller, open device.Opener) *Engine {
	return &Engine{host: host, api: client.New(host), open: open}
}

// Setup performs the one-time host-facing initialisation. It MUST NOT run on the host's
// session goroutine (host calls there deadlock the head) — the cgo shell runs it on its
// own goroutine. Filled in by later milestones (register commands, start the read loop).
func (e *Engine) Setup() error { return nil }

// Notify receives host event bytes (ribbon command triggers, etc.). Filled in by a later
// milestone.
func (e *Engine) Notify(ev []byte) {}

// Stop ends the device read loop and releases the device. Idempotent; called on Deactivate.
func (e *Engine) Stop() {
	e.mu.Lock()
	dev, done := e.dev, e.done
	e.dev, e.done = nil, nil
	e.mu.Unlock()
	if done != nil {
		close(done)
	}
	if dev != nil {
		_ = dev.Close()
	}
}
