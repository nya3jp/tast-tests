// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/fingerprint"
	"chromiumos/tast/remote/firmware/fingerprint/rpcdut"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpRebootPowerCycle,
		Desc: "Validates that AP firmware performs FPMCU power cycle on reboot",
		Contacts: []string{
			"patrykd@google.com", // Test author
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      7 * time.Minute,
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
		ServiceDeps:  []string{"tast.cros.platform.UpstartService", dutfs.ServiceName},
		Vars:         []string{"servo"},
	})
}

func FpRebootPowerCycle(ctx context.Context, s *testing.State) {
	d, err := rpcdut.NewRPCDUT(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect RPCDUT: ", err)
	}
	defer d.Close(ctx)

	servoSpec, ok := s.Var("servo")
	if !ok {
		servoSpec = ""
	}
	firmwareFile, err := fingerprint.NewMPFirmwareFile(ctx, d)
	if err != nil {
		s.Fatal("Failed to create MP firmwareFile: ", err)
	}
	t, err := fingerprint.NewFirmwareTest(ctx, d, servoSpec, s.OutDir(), firmwareFile, false, false)
	if err != nil {
		s.Fatal("Failed to create new firmware test: ", err)
	}
	ctxForCleanup := ctx
	defer func() {
		if err := t.Close(ctxForCleanup); err != nil {
			s.Fatal("Failed to clean up: ", err)
		}
	}()
	ctx, cancel := ctxutil.Shorten(ctx, t.CleanupTime())
	defer cancel()

	testing.ContextLog(ctx, "Rebooting FPMCU to clear power-on reset flag")
	if err := fingerprint.RebootFpmcu(ctx, d.DUT(), fingerprint.ImageTypeRW); err != nil {
		s.Fatal("Failed to reboot FPMCU: ", err)
	}

	// Confirm that FPMCU doesn't report power-on reset flag.
	flags, err := fingerprint.GetResetFlags(ctx, d.DUT())
	if err != nil {
		s.Fatal("Failed to get FPMCU reset flags: ", err)
	}
	testing.ContextLogf(ctx, "FPMCU reset flags: %#08x", flags)

	if flags.IsSet(fingerprint.ResetFlagPowerOn) {
		s.Fatal("Reset flags contain power-on flag after FPMCU reset")
	}

	testing.ContextLog(ctx, "Rebooting DUT")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	// Confirm that after DUT reboot FPMCU reports power-on reset flag.
	flags, err = fingerprint.GetResetFlags(ctx, d.DUT())
	if err != nil {
		s.Fatal("Failed to get FPMCU reset flags: ", err)
	}
	testing.ContextLogf(ctx, "FPMCU reset flags: %#08x", flags)

	if !flags.IsSet(fingerprint.ResetFlagPowerOn) {
		s.Fatal("Reset flags don't contain power-on flag")
	}
}
