// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/fingerprint"
	"chromiumos/tast/remote/firmware/fingerprint/rpcdut"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpBioWash,
		Desc: "Validate bio_wash behavior",
		Contacts: []string{
			"josienordrum@chromium.org", // Test author
			"tomhughes@chromium.org",
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

func FpBioWash(ctx context.Context, s *testing.State) {
	d, err := rpcdut.NewRPCDUT(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect RPCDUT: ", err)
	}
	defer d.Close(ctx)

	servoSpec, ok := s.Var("servo")
	if !ok {
		servoSpec = ""
	}
	// HW wp must be disabled to flash_fp_mcu.
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

	// This test requires a forced flash without entropy
	// initialization to clear entropy.
	testing.ContextLog(ctx, "Force flashing original FP firmware")
	if err := fingerprint.FlashFirmware(ctx, d, t.FirmwareFile().FilePath, t.NeedsRebootAfterFlashing()); err != nil {
		s.Fatal("Failed to flash original FP firmware: ", err)
	}

	// Wait for FPMCU to boot to RW. Fail if it does not.
	testing.ContextLog(ctx, "Waiting for FPMCU to reboot to RW")
	if err := fingerprint.WaitForRunningFirmwareImage(ctx, d.DUT(), fingerprint.ImageTypeRW); err != nil {
		s.Fatal("Failed to boot to RW image: ", err)
	}

	// Rollback should be unset for this test.
	testing.ContextLog(ctx, "Validating initial rollback state")
	if err := fingerprint.CheckRollbackState(ctx, d, fingerprint.RollbackState{
		BlockID: 0, MinVersion: 0, RWVersion: 0}); err != nil {
		s.Fatal("Failed to validate rollback state: ", err)
	}

	// Enable hardware write protect first.
	testing.ContextLog(ctx, "Enabling hardware write protect")
	if err := t.Servo().Servo().SetFWWPState(ctx, servo.FWWPStateOn); err != nil {
		s.Fatal("Failed to ensable hardware write protection: ", err)
	}

	// Enable software write protect.
	testing.ContextLog(ctx, "Enabling software write protect")
	if err := fingerprint.SetSoftwareWriteProtect(ctx, d.DUT(), true); err != nil {
		s.Fatal("Failed to enable software write protect")
	}

	testing.ContextLog(ctx, "Checking that firmware is functional")
	if _, err := fingerprint.CheckFirmwareIsFunctional(ctx, d.DUT()); err != nil {
		s.Fatal("Firmware is not functional after initialization: ", err)
	}

	testing.ContextLog(ctx, "Calling bio_wash with factory_init")
	if err := fingerprint.BioWash(ctx, d, false); err != nil {
		s.Fatal("Failed to call bio_wash with factory_init: ", err)
	}

	testing.ContextLog(ctx, "Validating rollback block ID is 1")
	if err := fingerprint.CheckRollbackState(ctx, d, fingerprint.RollbackState{
		BlockID: 1, MinVersion: 0, RWVersion: 0}); err != nil {
		s.Fatal("Unexpected rollback state: ", err)
	}

	testing.ContextLog(ctx, "Calling bio_wash")
	if err := fingerprint.BioWash(ctx, d, true); err != nil {
		s.Fatal("Failed to call bio_wash: ", err)
	}

	testing.ContextLog(ctx, "Validating Block ID increases by 2, but nothing else")
	if err := fingerprint.CheckRollbackState(ctx, d, fingerprint.RollbackState{
		BlockID: 3, MinVersion: 0, RWVersion: 0}); err != nil {
		s.Fatal("Unexpected rollback state: ", err)
	}
}
