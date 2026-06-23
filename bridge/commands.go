// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"embed"
	"encoding/json"

	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
	"oblikovati.org/spacemouse/navigate"
)

// Command ids the add-in owns. The host fires command.started with one of these when its
// ribbon button is clicked; Notify dispatches on it.
const (
	ToggleCommandID      = "SpaceMouse.Toggle"      // enable/disable navigation
	HomeCommandID        = "SpaceMouse.Home"        // recenter: iso view, fit to model
	SensitivityCommandID = "SpaceMouse.Sensitivity" // cycle Low/Medium/High speed
)

// ribbonTab/ribbonPanel place the controls. The host creates the tab on the part ribbon
// if it does not exist (same as the CAM add-in's dedicated tab).
const (
	ribbonTab   = "SpaceMouse"
	ribbonPanel = "Navigation"
)

// sensitivityPresets scale the default speeds. defaultSensitivity (Medium) is the 1.0
// preset, i.e. navigate.DefaultConfig unchanged.
var sensitivityPresets = []struct {
	name string
	mul  float64
}{
	{"Low", 0.5},
	{"Medium", 1.0},
	{"High", 2.0},
}

const defaultSensitivity = 1 // index of "Medium"

// configForSensitivity returns the default tuning with every speed scaled by the preset.
func configForSensitivity(idx int) navigate.Config {
	c := navigate.DefaultConfig()
	m := sensitivityPresets[idx].mul
	c.PanSpeed *= m
	c.ZoomSpeed *= m
	c.OrbitSpeed *= m
	c.RollSpeed *= m
	return c
}

// command describes one ribbon button.
type command struct {
	id    string
	name  string
	tip   string
	kind  types.ControlKind
	style types.ButtonStyle
	icon  string // key into icons/<icon>.svg
}

var commands = []command{
	{ToggleCommandID, "SpaceMouse", "Enable or disable SpaceMouse camera navigation.",
		types.ToggleControl, types.LargeIconButton, "spacemouse"},
	{HomeCommandID, "Home View", "Recenter: isometric view, fit the model to the window.",
		types.ButtonControl, types.CompactIconButton, "home"},
	{SensitivityCommandID, "Sensitivity: Medium", "Cycle navigation speed: Low, Medium, High.",
		types.ButtonControl, types.CompactIconButton, "sensitivity"},
}

// registerCommands creates the ribbon buttons and reflects the initial Toggle state (on).
func (e *Engine) registerCommands() error {
	for _, c := range commands {
		if _, err := e.api.Commands().Create(wire.CreateCommandArgs{
			ID: c.id, DisplayName: c.name, Tooltip: c.tip,
			Ribbon: types.PartRibbon, Tab: ribbonTab, Category: ribbonPanel,
			Kind: c.kind, ButtonStyle: c.style, IconSVG: iconSVG(c.icon),
		}); err != nil {
			return err
		}
	}
	_, _ = e.api.Commands().SetState(wire.SetCommandStateArgs{ID: ToggleCommandID, Active: e.enabled})
	return nil
}

// Notify receives host event bytes; it acts only on a command.started for a button this
// add-in owns. It runs on the host's session goroutine, so the handlers only mutate engine
// state and make at most one short host call (SetState / SetOrientation) — never a blocking
// read loop.
func (e *Engine) Notify(ev []byte) {
	var hdr struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(ev, &hdr) != nil || hdr.Type != wire.EventCommandStarted {
		return
	}
	var c wire.CommandStartedEvent
	if json.Unmarshal(ev, &c) == nil {
		e.dispatchCommand(c.Command)
	}
}

// dispatchCommand routes a fired command id to its handler, ignoring commands the add-in
// does not own.
func (e *Engine) dispatchCommand(id string) {
	switch id {
	case ToggleCommandID:
		e.toggleEnabled()
	case HomeCommandID:
		e.homeView()
	case SensitivityCommandID:
		e.cycleSensitivity()
	}
}

// toggleEnabled flips navigation on/off and reflects it on the toggle button.
func (e *Engine) toggleEnabled() {
	e.mu.Lock()
	e.enabled = !e.enabled
	on := e.enabled
	e.mu.Unlock()
	_, _ = e.api.Commands().SetState(wire.SetCommandStateArgs{ID: ToggleCommandID, Active: on})
}

// homeView jumps to the top-right isometric and fits the model to the window.
func (e *Engine) homeView() {
	_, _ = e.api.View().SetOrientation(wire.SetOrientationArgs{
		Orientation: types.IsoTopRightViewOrientation, Fit: true,
	})
}

// cycleSensitivity advances to the next speed preset and updates the button label.
func (e *Engine) cycleSensitivity() {
	e.mu.Lock()
	e.sensIdx = (e.sensIdx + 1) % len(sensitivityPresets)
	e.cfg = configForSensitivity(e.sensIdx)
	name := sensitivityPresets[e.sensIdx].name
	e.mu.Unlock()
	_, _ = e.api.Commands().SetState(wire.SetCommandStateArgs{
		ID: SensitivityCommandID, DisplayName: "Sensitivity: " + name,
	})
}

// iconFS bundles the ribbon glyphs. Each is "icons/<key>.svg" in the Oblikovati glyph
// convention (a 24×24 viewBox; the sentinel paints the host recolours per theme: a green
// fill tile, a black outline, red action accents).
//
//go:embed icons/*.svg
var iconFS embed.FS

// iconSVG returns the inline SVG markup for a button glyph, or "" when none is bundled
// (the host then falls back to a text button).
func iconSVG(key string) string {
	b, err := iconFS.ReadFile("icons/" + key + ".svg")
	if err != nil {
		return ""
	}
	return string(b)
}
