// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package guestos

import (
	"context"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/graphics/trace"
	"chromiumos/tast/local/graphics/trace/comm"
	"chromiumos/tast/testing"
)

// TraceReplayCommon replays a graphics trace inside a crostini container.
func TraceReplayCommon(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	config := s.Param().(comm.TestGroupConfig)
	defer crostini.RunCrostiniPostTest(ctx, s.PreValue().(crostini.PreData))
	guest := CrostiniGuestOS{
		VMInstance: pre.Container,
	}

	var pTestVars *comm.TestVars
	if config.ExtendedDuration > 0 {
		pTestVars = &comm.TestVars{PowerTestVars: comm.GetPowerTestVars(s)}
	}

	if err := trace.RunTraceReplayTest(ctx, s.OutDir(), s.CloudStorage(), &guest, &config, pTestVars); err != nil {
		s.Fatal("Trace replay test failed: ", err)
	}
}
