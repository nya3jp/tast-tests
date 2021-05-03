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

// resizeDisk resizes the VM's disk for size > 0 and reports the actual VM size.
func resizeDisk(ctx context.Context, s *testing.State, pre *crostini.PreData, sizeBytes uint64) error {
	settingsApp, err := settings.OpenLinuxSettings(ctx, pre.TestAPIConn, pre.Chrome)
	if err != nil {
		msg := "Failed to open Linux settings: "
		if sizeBytes > 0 {
			s.Fatal(msg, err)
		}
		s.Error(msg, err)
		return err
	}
	defer settingsApp.Close(ctx)

	if sizeBytes > 0 {
		testing.ContextLogf(ctx, "Resizing VM disk to %d bytes", sizeBytes)
		if err := settingsApp.ResizeDisk(ctx, pre.Keyboard, sizeBytes, true); err != nil {
			s.Fatal("Failed to resize VM disk: ", err)
		}
	}

	diskSizeString, err := settingsApp.GetDiskSize(ctx)
	if err != nil {
		s.Error("Failed to query VM disk size: ", err)
		return err
	}
	testing.ContextLogf(ctx, "VM disk configured with size: %s", diskSizeString)

	return err
}

// TraceReplayCrostiniSetup replays a graphics trace inside a crostini container. The VM disk will be resized for resizeDiskBytes > 0.
func TraceReplayCrostiniSetup(ctx context.Context, s *testing.State, resizeDiskBytes uint64) {
	pre := s.PreValue().(crostini.PreData)
	defer crostini.RunCrostiniPostTest(ctx, s.PreValue().(crostini.PreData))

	resizeDisk(ctx, s, &pre, resizeDiskBytes)

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
