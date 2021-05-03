// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package guestos

import (
	"context"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/local/graphics/trace"
	"chromiumos/tast/local/graphics/trace/comm"
	"chromiumos/tast/testing"
)

// TraceReplayCrostiniSetup replays a graphics trace inside a crostini container. The VM disk will be resized for resizeDiskGB > 0.
func TraceReplayCrostiniSetup(ctx context.Context, s *testing.State, resizeDiskGB uint64) {
	pre := s.PreValue().(crostini.PreData)
	defer crostini.RunCrostiniPostTest(ctx, s.PreValue().(crostini.PreData))

	func() { // give defer statements block-scope to close settingsApp before continuing
		settingsApp, err := settings.OpenLinuxSettings(ctx, pre.TestAPIConn, pre.Chrome)
		if err != nil {
			s.Fatal("Failed to open Linux settings: ", err)
		}
		defer settingsApp.Close(ctx)

		if (resizeDiskGB > 0) {
			testing.ContextLogf(ctx, "Resizing VM disk to %d GB", resizeDiskGB)
			if err := settingsApp.ResizeDisk(ctx, pre.Keyboard, resizeDiskGB * settings.SizeGB, true); err != nil {
				s.Fatal("Failed to resize VM disk: ", err)
			}
		} else {
			defaultDiskSize, err := settingsApp.GetDiskSize(ctx)
			if (err != nil) {
				s.Fatal("Failed to query default VM disk size: ", err)
			}
			testing.ContextLogf(ctx, "VM disk configured with default size of %s", defaultDiskSize)
		}
	}()

	config := s.Param().(comm.TestGroupConfig)

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
