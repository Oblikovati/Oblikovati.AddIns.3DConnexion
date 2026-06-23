// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
	"oblikovati.org/spacemouse/device"
)

// fakeHost records the method calls the engine makes and replies with canned JSON, so the
// tests assert what the engine sends the host without a live session.
type fakeHost struct {
	mu    sync.Mutex
	calls []hostCall
	reply map[string][]byte
}

type hostCall struct {
	method string
	req    []byte
}

func (h *fakeHost) Call(method string, req []byte) ([]byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.calls = append(h.calls, hostCall{method, append([]byte(nil), req...)})
	if r, ok := h.reply[method]; ok {
		return r, nil
	}
	return []byte("{}"), nil
}

func (h *fakeHost) countOf(method string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, c := range h.calls {
		if c.method == method {
			n++
		}
	}
	return n
}

func (h *fakeHost) lastReq(method string) []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := len(h.calls) - 1; i >= 0; i-- {
		if h.calls[i].method == method {
			return h.calls[i].req
		}
	}
	return nil
}

// cameraReply is a view.getCamera response 10cm down +X looking at the origin, +Z up.
func cameraReply() []byte {
	b, _ := json.Marshal(wire.CameraView{
		Eye: types.NewPoint(10, 0, 0), Target: types.NewPoint(0, 0, 0),
		Up: types.NewVector(0, 0, 1), FOV: 0.78,
	})
	return b
}

// fakeDevice replays a fixed list of samples then closes.
type fakeDevice struct {
	ch     chan device.MotionSample
	closed bool
}

func newFakeDevice(samples ...device.MotionSample) *fakeDevice {
	ch := make(chan device.MotionSample, len(samples))
	for _, s := range samples {
		ch <- s
	}
	close(ch)
	return &fakeDevice{ch: ch}
}

func (d *fakeDevice) Samples() <-chan device.MotionSample { return d.ch }
func (d *fakeDevice) Close() error                        { d.closed = true; return nil }

func newEngine(t *testing.T) (*Engine, *fakeHost) {
	t.Helper()
	host := &fakeHost{reply: map[string][]byte{wire.MethodViewGetCamera: cameraReply()}}
	eng := NewEngine(host, func() (device.Device, error) { return newFakeDevice(), nil })
	return eng, host
}

func TestRegisterCommandsPlacesAll(t *testing.T) {
	eng, host := newEngine(t)
	if err := eng.Setup(); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if n := host.countOf(wire.MethodCommandsCreate); n != len(commands) {
		t.Fatalf("created %d commands, want %d", n, len(commands))
	}
	// Every created command lands on the SpaceMouse tab's Navigation panel of the part ribbon.
	for _, c := range host.calls {
		if c.method != wire.MethodCommandsCreate {
			continue
		}
		var a wire.CreateCommandArgs
		if err := json.Unmarshal(c.req, &a); err != nil {
			t.Fatal(err)
		}
		if a.Ribbon != types.PartRibbon || a.Tab != ribbonTab || a.Category != ribbonPanel {
			t.Errorf("command %q placed at ribbon=%q tab=%q panel=%q", a.ID, a.Ribbon, a.Tab, a.Category)
		}
		if a.IconSVG == "" {
			t.Errorf("command %q has no icon", a.ID)
		}
	}
	if host.countOf(wire.MethodCommandsSetState) == 0 {
		t.Error("initial toggle state was not set")
	}
}

func TestOnSampleDrivesCamera(t *testing.T) {
	eng, host := newEngine(t)
	eng.onSample(device.MotionSample{Rz: 1, Period: time.Second})

	if host.countOf(wire.MethodViewGetCamera) != 1 {
		t.Fatalf("expected one camera read, got %d", host.countOf(wire.MethodViewGetCamera))
	}
	if host.countOf(wire.MethodViewSetCamera) != 1 {
		t.Fatalf("expected one camera write, got %d", host.countOf(wire.MethodViewSetCamera))
	}
	var got wire.SetCameraArgs
	if err := json.Unmarshal(host.lastReq(wire.MethodViewSetCamera), &got); err != nil {
		t.Fatal(err)
	}
	// A yaw orbit must move the eye off the start (10,0,0) while keeping distance ≈ 10.
	if got.Eye.X == 10 && got.Eye.Y == 0 {
		t.Fatalf("camera did not move: %+v", got.Eye)
	}
	if d := got.Eye.DistanceTo(got.Target); d < 9.9 || d > 10.1 {
		t.Fatalf("orbit changed distance to %v", d)
	}
}

func TestDisabledDoesNotDriveCamera(t *testing.T) {
	eng, host := newEngine(t)
	eng.enabled = false
	eng.onSample(device.MotionSample{Tx: 1, Period: time.Second})
	if host.countOf(wire.MethodViewSetCamera) != 0 {
		t.Fatal("disabled engine moved the camera")
	}
}

func TestZeroSampleDoesNotDriveCamera(t *testing.T) {
	eng, host := newEngine(t)
	eng.onSample(device.MotionSample{Period: time.Second}) // rest
	if host.countOf(wire.MethodViewSetCamera) != 0 {
		t.Fatal("rest sample moved the camera")
	}
}

func TestGestureCachesCameraRead(t *testing.T) {
	eng, host := newEngine(t)
	// Two motion samples in one gesture: the camera is read once, then integrated locally.
	eng.onSample(device.MotionSample{Rz: 1, Period: time.Second})
	eng.onSample(device.MotionSample{Rz: 1, Period: time.Second})
	if n := host.countOf(wire.MethodViewGetCamera); n != 1 {
		t.Fatalf("gesture read the camera %d times, want 1", n)
	}
	if n := host.countOf(wire.MethodViewSetCamera); n != 2 {
		t.Fatalf("gesture wrote the camera %d times, want 2", n)
	}
}

func TestRestEndsGesture(t *testing.T) {
	eng, host := newEngine(t)
	eng.onSample(device.MotionSample{Rz: 1, Period: time.Second}) // gesture 1
	eng.onSample(device.MotionSample{Period: time.Second})        // rest
	eng.onSample(device.MotionSample{Rz: 1, Period: time.Second}) // gesture 2
	if n := host.countOf(wire.MethodViewGetCamera); n != 2 {
		t.Fatalf("expected a fresh read per gesture (2), got %d", n)
	}
}

func TestToggleEnabled(t *testing.T) {
	eng, host := newEngine(t)
	if !eng.enabled {
		t.Fatal("engine should start enabled")
	}
	eng.dispatchCommand(ToggleCommandID)
	if eng.enabled {
		t.Fatal("toggle did not disable")
	}
	var a wire.SetCommandStateArgs
	_ = json.Unmarshal(host.lastReq(wire.MethodCommandsSetState), &a)
	if a.ID != ToggleCommandID || a.Active {
		t.Fatalf("toggle state not reflected: %+v", a)
	}
}

func TestSensitivityCycles(t *testing.T) {
	eng, host := newEngine(t)
	start := eng.sensIdx
	eng.dispatchCommand(SensitivityCommandID)
	if eng.sensIdx != (start+1)%len(sensitivityPresets) {
		t.Fatalf("sensitivity did not advance: %d -> %d", start, eng.sensIdx)
	}
	// The faster preset must raise the configured orbit speed.
	if eng.cfg.OrbitSpeed <= configForSensitivity(start).OrbitSpeed {
		t.Fatalf("config not updated for new sensitivity: %v", eng.cfg.OrbitSpeed)
	}
	var a wire.SetCommandStateArgs
	_ = json.Unmarshal(host.lastReq(wire.MethodCommandsSetState), &a)
	if a.DisplayName == "" {
		t.Fatal("sensitivity label not updated")
	}
}

func TestHomeSetsOrientation(t *testing.T) {
	eng, host := newEngine(t)
	eng.dispatchCommand(HomeCommandID)
	var a wire.SetOrientationArgs
	if err := json.Unmarshal(host.lastReq(wire.MethodViewSetOrientation), &a); err != nil {
		t.Fatal(err)
	}
	if a.Orientation != types.IsoTopRightViewOrientation || !a.Fit {
		t.Fatalf("home did not request a fitted iso view: %+v", a)
	}
}

func TestNotifyRoutesCommandStarted(t *testing.T) {
	eng, _ := newEngine(t)
	ev, _ := json.Marshal(wire.CommandStartedEvent{Type: wire.EventCommandStarted, Command: ToggleCommandID})
	eng.Notify(ev)
	if eng.enabled {
		t.Fatal("Notify did not dispatch the toggle command")
	}
	// An unrelated event must be ignored.
	eng.Notify([]byte(`{"type":"something.else"}`))
	eng.Notify([]byte(`not json`))
}

func TestRunLoopStops(t *testing.T) {
	host := &fakeHost{reply: map[string][]byte{wire.MethodViewGetCamera: cameraReply()}}
	dev := newFakeDevice(device.MotionSample{Rz: 1, Period: time.Second})
	eng := NewEngine(host, func() (device.Device, error) { return dev, nil })

	if err := eng.Setup(); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	// The fake device's channel closes after its samples, so the loop drains and exits;
	// Stop must then be safe and close the device.
	waitFor(t, func() bool { return host.countOf(wire.MethodViewSetCamera) >= 1 })
	eng.Stop()
	if !dev.closed {
		t.Fatal("Stop did not close the device")
	}
	eng.Stop() // idempotent
}

func TestStartDeviceToleratesOpenError(t *testing.T) {
	host := &fakeHost{}
	eng := NewEngine(host, func() (device.Device, error) { return nil, errOpen })
	if err := eng.Setup(); err != nil {
		t.Fatalf("Setup should tolerate a missing device: %v", err)
	}
}

type openErr struct{}

func (openErr) Error() string { return "no device" }

var errOpen = openErr{}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition not met before deadline")
}
