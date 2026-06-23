// SPDX-License-Identifier: GPL-2.0-only

// Package bridge wires the SpaceMouse to the host: it opens the device, turns its motion
// into camera frames (package navigate), and pushes them to the viewport over the API. It
// is cgo-free — the host transport is an injected [HostCaller] and the device an injected
// [device.Opener] — so it is fully unit-testable with a fake host and a fake device.
package bridge

import (
	"log/slog"
	"sync"

	"oblikovati.org/api/client"
	"oblikovati.org/api/wire"
	"oblikovati.org/spacemouse/device"
	"oblikovati.org/spacemouse/navigate"
)

// HostCaller is the transport the engine talks to the host through — exactly the
// api/client Caller contract, supplied by the cgo shell at Activate (or a fake in tests).
type HostCaller interface {
	Call(method string, req []byte) ([]byte, error)
}

// Engine drives the viewport camera from a 3Dconnexion SpaceMouse. It owns the device read
// loop; the host owns the camera. State guarded by mu is shared between the read-loop
// goroutine and the host's Notify goroutine.
type Engine struct {
	host HostCaller
	api  *client.Client
	open device.Opener

	mu      sync.Mutex
	enabled bool            // navigation on/off (the Toggle command)
	sensIdx int             // index into sensitivityPresets
	cfg     navigate.Config // current tuning (sensitivity preset applied)
	dev     device.Device   // open device, nil when not running
	done    chan struct{}   // closed to stop the read loop

	// cur caches the camera during one gesture so the engine reads view.getCamera once
	// per gesture and integrates locally; reset to nil at rest. Touched only by the read
	// loop goroutine, so it needs no lock.
	cur *navigate.Frame
}

// NewEngine binds the engine to the host transport and the device opener (injected so
// tests can supply a fake device). Navigation starts enabled at medium sensitivity.
func NewEngine(host HostCaller, open device.Opener) *Engine {
	return &Engine{
		host:    host,
		api:     client.New(host),
		open:    open,
		enabled: true,
		sensIdx: defaultSensitivity,
		cfg:     configForSensitivity(defaultSensitivity),
	}
}

// Setup registers the ribbon commands and starts the device read loop. It MUST NOT run on
// the host's session goroutine (host calls there deadlock the head) — the cgo shell runs
// it on its own goroutine.
func (e *Engine) Setup() error {
	if err := e.registerCommands(); err != nil {
		return err
	}
	e.startDevice()
	return nil
}

// startDevice opens the SpaceMouse and runs its read loop. A missing device is not an
// error: the add-in stays loaded with its commands, navigation is just inert until a
// device appears (and the user re-activates).
func (e *Engine) startDevice() {
	dev, err := e.open()
	if err != nil {
		slog.Warn("spacemouse: device unavailable", "err", err)
		return
	}
	done := make(chan struct{})
	e.mu.Lock()
	e.dev, e.done = dev, done
	e.mu.Unlock()
	go e.run(dev, done)
}

// run consumes the device's motion stream until it closes or Stop signals done.
func (e *Engine) run(dev device.Device, done chan struct{}) {
	samples := dev.Samples()
	for {
		select {
		case <-done:
			return
		case s, ok := <-samples:
			if !ok {
				return
			}
			e.onSample(s)
		}
	}
}

// onSample applies one motion sample to the camera. A rest sample (or disabled
// navigation) ends the current gesture so the next motion re-reads the live camera.
func (e *Engine) onSample(s device.MotionSample) {
	e.mu.Lock()
	enabled, cfg := e.enabled, e.cfg
	e.mu.Unlock()
	if !enabled || s.IsZero() {
		e.cur = nil
		return
	}
	frame, ok := e.gestureFrame()
	if !ok {
		return
	}
	next := navigate.Apply(frame, s, cfg)
	e.cur = &next
	e.pushCamera(next)
}

// gestureFrame returns the camera to integrate from: the cached frame mid-gesture, or a
// fresh read from the host at the start of one (so the gesture builds on whatever the user
// last left the view at).
func (e *Engine) gestureFrame() (navigate.Frame, bool) {
	if e.cur != nil {
		return *e.cur, true
	}
	cam, err := e.api.View().Camera()
	if err != nil {
		slog.Warn("spacemouse: read camera failed", "err", err)
		return navigate.Frame{}, false
	}
	return frameFromWire(cam), true
}

// pushCamera applies a frame to the host's active-view camera.
func (e *Engine) pushCamera(f navigate.Frame) {
	if _, err := e.api.View().SetCamera(wire.SetCameraArgs{
		Eye: f.Eye, Target: f.Target, Up: f.Up, FOV: f.FOV,
	}); err != nil {
		slog.Warn("spacemouse: set camera failed", "err", err)
	}
}

// frameFromWire converts the host's camera DTO into the navigate frame (identical fields;
// the conversion keeps navigate free of the wire types).
func frameFromWire(c wire.CameraView) navigate.Frame {
	return navigate.Frame{Eye: c.Eye, Target: c.Target, Up: c.Up, FOV: c.FOV}
}

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
