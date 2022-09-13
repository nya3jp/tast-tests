// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"path/filepath"

	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// Screendiffer contains the required fields for screenshot testing.
type Screendiffer struct {
	differ *screenshot.Differ // Differ for screenshot testing.
	state  *screenDiffState   // Private states for crostini tests.
}

// TODO(b/235164130) remove this struct once FixtTestState.Var is available and
// use testing.FixtState is sufficient.
type screenDiffState struct {
	testState *testing.FixtTestState
	fixtState *testing.FixtState
}

func (state *screenDiffState) Var(name string) (string, bool) {
	return state.fixtState.Var(name)
}

func (state *screenDiffState) TestName() string {
	// TODO(b/235164130) use FixtTestState.TestName once it is available.
	return filepath.Base(state.testState.OutDir())
}

func (state *screenDiffState) Fatal(args ...interface{}) {
	state.testState.Fatal(args...)
}
