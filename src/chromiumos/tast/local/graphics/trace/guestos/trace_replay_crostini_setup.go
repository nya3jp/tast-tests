// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package guestos

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/local/graphics/trace"
	"chromiumos/tast/local/graphics/trace/comm"
	"chromiumos/tast/testing"
)

// resizeDisk resizes the VM's disk for size > 0 and reports the actual VM size.
func resizeDisk(ctx context.Context, pre *crostini.PreData, sizeBytes uint64) error {
	settingsApp, err := settings.OpenLinuxSettings(ctx, pre.TestAPIConn, pre.Chrome)
	if err != nil {
		return errors.Wrap(err, "failed to open Linux settings")
	}
	defer settingsApp.Close(ctx)

	if sizeBytes > 0 {
		testing.ContextLogf(ctx, "Resizing VM disk to %d bytes", sizeBytes)
		if _, _, err := settingsApp.Resize(ctx, sizeBytes); err != nil {
			return errors.Wrap(err, "failed to resize VM disk")
		}
	}

	diskSizeString, err := settingsApp.GetDiskSize(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to query VM disk size")
	}
	testing.ContextLogf(ctx, "VM disk configured with size: %s", diskSizeString)

	return nil
}

// TraceReplayCrostiniSetup replays a graphics trace inside a crostini container. The VM disk will be resized for resizeDiskBytes > 0.
func TraceReplayCrostiniSetup(ctx context.Context, s *testing.State, resizeDiskBytes uint64) {
	pre := s.PreValue().(crostini.PreData)
	defer crostini.RunCrostiniPostTest(ctx, s.PreValue().(crostini.PreData))

	if err := resizeDisk(ctx, &pre, resizeDiskBytes); err != nil {
		if resizeDiskBytes > 0 {
			s.Fatal("Failed to resize: ", err)
		}
		s.Error("Failed to resize: ", err)
	}

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
