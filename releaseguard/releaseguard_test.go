// SPDX-License-Identifier: GPL-2.0-only

// Package releaseguard holds guards on the add-in's release identity.
// It is test-only: the guards run in the ordinary `go test ./...` sweep.
package releaseguard_test

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"
)

// addInIDPattern pulls the literal out of `const addInID = "..."` in the root export.go.
// export.go is read as text rather than imported because package main is cgo, and the CI
// test legs run with CGO_ENABLED=0.
var addInIDPattern = regexp.MustCompile(`(?m)^const addInID = "([^"]+)"`)

// TestAddInIDMatchesManifest fails when the compiled-in add-in id drifts from manifest.json.
//
// The two are separate literals (manifest.json is embedded for ObkAddInManifest, addInID is
// a Go constant for ObkAddInId) and nothing but this test ties them together. Drift is not
// cosmetic: scripts/publish-catalogue.sh posts the manifest id as the publish NAME, and
// addins.oblikovati.org both authorizes by that name and rejects a bundle whose manifest id
// disagrees with it. The add-in shipped for a month publishing an id the catalogue had no
// token for, and every hourly release run 401'd.
func TestAddInIDMatchesManifest(t *testing.T) {
	if got, want := manifestID(t), constantID(t); got != want {
		t.Errorf("manifest.json id = %q but export.go addInID = %q — they must be identical, "+
			"and must match the name registered in the catalogue's token map, or publishing 401s.", got, want)
	}
}

// manifestID reads the "id" field of the embedded-at-build manifest.
func manifestID(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("../manifest.json")
	if err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}
	var m struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse manifest.json: %v", err)
	}
	if m.ID == "" {
		t.Fatal("manifest.json has no \"id\" — the catalogue publishes and authorizes by it")
	}
	return m.ID
}

// constantID extracts the addInID literal compiled into the C-ABI entry point.
func constantID(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("../export.go")
	if err != nil {
		t.Fatalf("read export.go: %v", err)
	}
	m := addInIDPattern.FindSubmatch(raw)
	if m == nil {
		t.Fatal("export.go no longer declares `const addInID = \"...\"` — update this guard " +
			"to follow it, do not delete it")
	}
	return string(m[1])
}
