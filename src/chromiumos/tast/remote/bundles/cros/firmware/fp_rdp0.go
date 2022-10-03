// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/fingerprint"
	"chromiumos/tast/remote/firmware/fingerprint/rpcdut"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpRDP0,
		Desc: "Validate read protection (RDP) level 0 of the fingerprint firmware works as expected",
		Contacts: []string{
			"tomhughes@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      15 * time.Minute,
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
		ServiceDeps:  []string{"tast.cros.platform.UpstartService", dutfs.ServiceName},
		Vars:         []string{"servo"},
	})
}

func FpRDP0(ctx context.Context, s *testing.State) {
	d, err := rpcdut.NewRPCDUT(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect RPCDUT: ", err)
	}
	defer d.Close(ctx)

	// Set both HW and SW write protect to false to get RDP0 state.
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

	// This test requires a forced flash without entropy
	// initialization to ensure that we get an exact match between
	// the original firmware that was flashed and the value that is
	// read.
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

	testing.ContextLog(ctx, "Checking that firmware is functional")
	if _, err := fingerprint.CheckFirmwareIsFunctional(ctx, d.DUT()); err != nil {
		s.Fatal("Firmware is not functional after initialization: ", err)
	}

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
	if err := testRDP0(ctx, d, t.FirmwareFile().FilePath, false, t.NeedsRebootAfterFlashing()); err != nil {
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
	if err := testRDP0(ctx, d, t.FirmwareFile().FilePath, true, t.NeedsRebootAfterFlashing()); err != nil {
		s.Fatal("Failed to validate RDP0 while setting RDP to level 0: ", err)
	}
}

// testRDP0 tests RDP0 functionality by trying to read from flash.
func testRDP0(ctx context.Context, d *rpcdut.RPCDUT, buildFwFile string, removeFlashReadProtect, needsReboot bool) (e error) {
	var fileReadFromFlash string
	var args []string

	fs := dutfs.NewClient(d.RPC().Conn)

	tempdirPath, err := fs.TempDir(ctx, "", "fingerprint_rdp0_*")
	if err != nil {
		return errors.Wrap(err, "failed to create remote temp directory")
	}
	defer func() {
		if fs != nil {
			tempDirExists, err := fs.Exists(ctx, tempdirPath)
			if err != nil {
				e = errors.Wrapf(err, "failed to check existence of temp directory: %q", tempdirPath)
				return
			}

			if !tempDirExists {
				// If we rebooted, the directory may no longer exist.
				return
			}

			if err := fs.RemoveAll(ctx, tempdirPath); err != nil {
				e = errors.Wrapf(err, "failed to remove temp directory: %q", tempdirPath)
			}
		} else {
			testing.ContextLog(ctx, "DUTFS connection not available. Skip removing the temp directory")
		}
	}()

	if removeFlashReadProtect {
		// Use different file name to avoid errors in removing file.
		fileReadFromFlash = filepath.Join(tempdirPath, "test_rdp0.bin")
		args = []string{"--noservices", "--read", fileReadFromFlash}
	} else {
		fileReadFromFlash = filepath.Join(tempdirPath, "test_rdp0_noremove.bin")
		args = []string{"--noservices", "--read",
			"--noremove_flash_read_protect", fileReadFromFlash}
	}
	cmd := d.Conn().CommandContext(ctx, "flash_fp_mcu", args...)
	out, err := cmd.CombinedOutput()
	testing.ContextLog(ctx, "flash_fp_mcu output:", "\n", string(out))
	if err != nil {
		return errors.Wrap(err, "failed to read from flash")
	}

	testing.ContextLog(ctx, "Checking that value read matches the flashed version")
	cmd = d.Conn().CommandContext(ctx, "cmp", buildFwFile, fileReadFromFlash)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "file read from flash does not match original fw file")
	}

	if needsReboot {
		testing.ContextLog(ctx, "Rebooting")
		// Reboot invalidates RPC client handle saved in fs. Assign nil
		// to fs to mark it explicitly as invalid.
		fs = nil
		if err := d.Reboot(ctx); err != nil {
			return errors.Wrap(err, "failed to reboot DUT")
		}
		fs = dutfs.NewClient(d.RPC().Conn)
	}

	if _, err := fingerprint.CheckFirmwareIsFunctional(ctx, d.DUT()); err != nil {
		return errors.Wrap(err, "firmware is not functional after reading flash")
	}
	return nil
}
