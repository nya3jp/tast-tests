// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/fingerprint"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpRDP0,
		Desc: "Validate read protection (RDP) level 0 of the fingerprint firmware works as expected",
		Contacts: []string{
			"yichengli@chromium.org", // Test author
			"tomhughes@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      6 * time.Minute,
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
		ServiceDeps:  []string{"tast.cros.platform.UpstartService"},
		Vars:         []string{"servo"},
	})
}

func FpRDP0(ctx context.Context, s *testing.State) {
	// Set both HW and SW write protect to false to get RDP0 state.
	t, err := fingerprint.NewFirmwareTest(ctx, s.DUT(), s.RequiredVar("servo"), s.RPCHint(), s.OutDir(), false, false)
	if err != nil {
		s.Fatal("Failed to create new firmware test: ", err)
	}
	ctxForCleanup := ctx
	defer func() {
		if err := t.Close(ctxForCleanup); err != nil {
			s.Fatal("Failed to clean up")
		}
	}()
	ctx, cancel := ctxutil.Shorten(ctx, fingerprint.TimeForCleanup)
	defer cancel()

	d := t.DUT()

	// This test requires a forced flash without entropy
	// initialization to ensure that we get an exact match between
	// the original firmware that was flashed and the value that is
	// read.
	testing.ContextLog(ctx, "Force flashing original FP firmware")
	if err := fingerprint.FlashFirmware(ctx, d); err != nil {
		s.Fatal("Failed to flash original FP firmware: ", err)
	}

	// Wait for FPMCU to boot to RW. Fail if it does not.
	testing.ContextLog(ctx, "Waiting for FPMCU to reboot to RW")
	if err := fingerprint.WaitForRunningFirmwareImage(ctx, d, fingerprint.ImageTypeRW); err != nil {
		s.Fatal("Failed to boot to RW image")
	}

	// Rollback should be unset for this test.
	testing.ContextLog(ctx, "Validating initial rollback state")
	if err := fingerprint.CheckRollbackState(ctx, d, fingerprint.RollbackState{
		BlockID: 0, MinVersion: 0, RWVersion: 0}); err != nil {
		s.Fatal("Failed to validate rollback state: ", err)
	}

	testing.ContextLog(ctx, "Checking that firmware is functional")
	if _, err := fingerprint.CheckFirmwareIsFunctional(ctx, d); err != nil {
		s.Fatal("Firmware is not functional after initialization: ", err)
	}

	// TODO(chromium:1189908): Use library function once it's there.
	// Prepare a temporary working directory on DUT.
	tempdir, err := d.Conn().Command("mktemp", "-d", "/tmp/fingerprint_rdp0_XXXXXX").Output(ctx, ssh.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to create remote temp directory: ", err)
	}
	tempdirPath := strings.TrimSpace(string(tempdir))
	removeTempdirCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	defer d.Conn().Command("rm", "-r", tempdirPath).Run(removeTempdirCtx, ssh.DumpLogOnError)

	// Given:
	// * Hardware write protect is disabled
	// * Software write protect is disabled
	// * RDP is at level 0
	//
	// Then:
	// * Reading from flash without changing the RDP level should succeed
	//   (we're already at level 0). Thus we should be able to read the
	//   entire firmware out of flash and it should exactly match the
	//   firmware that we flashed for testing.
	testing.ContextLog(ctx, "Reading firmware without modifying RDP level")
	if err := testRDP0(ctx, d, t.BuildFwFile(), tempdirPath, false, t.NeedsRebootAfterFlashing()); err != nil {
		s.Fatal("Failed to validate RDP0 without changing RDP level: ", err)
	}

	// Given:
	// * Hardware write protect is disabled
	// * Software write protect is disabled
	// * RDP is at level 0
	//
	// Then:
	// * Changing the RDP level to 0 should have no effect
	//   (we're already at level 0). Thus we should be able to read the
	//   entire firmware out of flash and it should exactly match the
	//   firmware that we flashed for testing.
	testing.ContextLog(ctx, "Reading firmware while setting RDP to level 0")
	if err := testRDP0(ctx, d, t.BuildFwFile(), tempdirPath, true, t.NeedsRebootAfterFlashing()); err != nil {
		s.Fatal("Failed to validate RDP0 while setting RDP to level 0: ", err)
	}
}

// testRDP0 tests RDP0 functionality by trying to read from flash.
func testRDP0(ctx context.Context, d *dut.DUT, buildFwFile, tempdirPath string, removeFlashReadProtect, needsReboot bool) error {
	var fileReadFromFlash string
	var args []string
	if removeFlashReadProtect {
		// Use different file name to avoid errors in removing file.
		fileReadFromFlash = filepath.Join(tempdirPath, "test_rdp0.bin")
		args = []string{"--read", fileReadFromFlash}
	} else {
		fileReadFromFlash = filepath.Join(tempdirPath, "test_rdp0_noremove.bin")
		args = []string{"--read", "--noremove_flash_read_protect", fileReadFromFlash}
	}
	cmd := d.Conn().Command("flash_fp_mcu", args...)
	if err := cmd.Run(ctx, ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to read from flash")
	}

	testing.ContextLog(ctx, "Checking that value read matches the flashed version")
	cmd = d.Conn().Command("cmp", buildFwFile, fileReadFromFlash)
	if err := cmd.Run(ctx); err != nil {
		return errors.Wrap(err, "file read from flash does not match original fw file")
	}

	if needsReboot {
		testing.ContextLog(ctx, "Rebooting")
		if err := d.Reboot(ctx); err != nil {
			return errors.Wrap(err, "failed to reboot DUT")
		}
	}

	if _, err := fingerprint.CheckFirmwareIsFunctional(ctx, d); err != nil {
		return errors.Wrap(err, "firmware is not functional after reading flash")
	}
	return nil
}
