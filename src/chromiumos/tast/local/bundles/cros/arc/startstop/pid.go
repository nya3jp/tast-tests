// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package startstop

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

// TestPID runs inside arc.StartStop.
type TestPID struct {
	oldPID int32
}

// PreStart implements Fixture.PreStart().
func (*TestPID) PreStart(ctx context.Context, s *testing.State) {
	// Do nothing.
}

// PostStart implements Fixture.PostStart(). It remembers the current ARC's
// init PID, which is used in PostStop().
func (t *TestPID) PostStart(ctx context.Context, s *testing.State) {
	pid, err := arc.InitPID()
	if err != nil {
		s.Error("Failed to find PID for init: ", err)
		return
	}
	t.oldPID = pid
}

// PostStop implements Fixture.PostStop(). It checks the PID for ARC is changed
// on Chrome logout (i.e. on ARC shutdown).
func (t *TestPID) PostStop(ctx context.Context, s *testing.State) {
	if t.oldPID == 0 {
		// The error is already reported in PostStart.
		return
	}

	// If err != nil, it means ARC is not running, so it's an expected case.
	newPID, err := arc.InitPID()
	if err == nil && newPID == t.oldPID {
		s.Error("ARC was not relaunched. Got PID: ", newPID)
	}
}
