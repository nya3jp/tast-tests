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
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/remote/servo"
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
		Vars:         []string{"servo"},
	})
}

func FpRDP0(ctx context.Context, s *testing.State) {
	// TODO(b/183151135): Move common preparation logic to a Precondition.
	d := s.DUT()

	fpBoard, err := fingerprint.Board(ctx, d)
	if err != nil {
		s.Fatal("Failed to get fingerprint board: ", err)
	}
	buildFwFile, err := fingerprint.FirmwarePath(ctx, d, fpBoard)
	if err != nil {
		s.Fatal("Failed to get build firmware file path: ", err)
	}
	if err := fingerprint.ValidateBuildFwFile(ctx, d, fpBoard, buildFwFile); err != nil {
		s.Fatal("Failed to validate build firmware file: ", err)
	}

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()
	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Tearing down")
		defer pxy.Close(ctx)
		fingerprint.ReimageFPMCU(ctx, d, pxy)
	}(cleanupCtx)

	// TODO(b/182596510): Check the FPMCU is running expected firmware version.

	// Set both HW and SW write protect to false to get RDP0 state.
	if err := fingerprint.InitializeHWAndSWWriteProtect(ctx, d, pxy, false, false); err != nil {
		s.Fatal("Initialization failed: ", err)
	}

	// This test requires a forced flash without entropy
	// initialization to ensure that we get an exact match between
	// the original firmware that was flashed and the value that is
	// read.
	if err := fingerprint.FlashFirmware(ctx, d); err != nil {
		s.Fatal("Failed to flash original FP firmware: ", err)
	}

	firmwareCopy, err := fingerprint.RunningFirmwareCopy(ctx, d)
	if err != nil {
		s.Fatal("Failed to query running firmware copy: ", err)
	}
	if firmwareCopy != fingerprint.ImageTypeRW {
		s.Fatal("Want RW firmware; actual: ", firmwareCopy)
	}

	// Rollback should be unset for this test.
	if err := fingerprint.CheckRollbackState(ctx, d, fingerprint.RollbackState{
		BlockID: 0, MinVersion: 0, RWVersion: 0}); err != nil {
		s.Fatal("Failed to validate rollback state: ", err)
	}

	if _, err := fingerprint.CheckFirmwareIsFunctional(ctx, d); err != nil {
		s.Fatal("Firmware is not functional after initialization: ", err)
	}

	// TODO(chromium:1189908): Use library function once it's there.
	// Prepare a temporary working directory on DUT.
	tempdir, err := d.Command("mktemp", "-d", "/tmp/fingerprint_rdp0_XXXXXX").Output(ctx)
	if err != nil {
		s.Fatal("Failed to create remote temp directory: ", err)
	}
	tempdirPath := strings.TrimSpace(string(tempdir))
	removeTempdirCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	defer d.Command("rm", "-r", tempdirPath).Run(removeTempdirCtx)

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
	if err := testRDP0(ctx, d, buildFwFile, tempdirPath, false); err != nil {
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
	if err := testRDP0(ctx, d, buildFwFile, tempdirPath, true); err != nil {
		s.Fatal("Failed to validate RDP0 while setting RDP to level 0: ", err)
	}
}

// testRDP0 tests RDP0 functionality by trying to read from flash.
func testRDP0(ctx context.Context, d *dut.DUT, buildFwFile, tempdirPath string, removeFlashReadProtect bool) error {
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
	cmd := d.Command("flash_fp_mcu", args...)
	if err := cmd.Run(ctx); err != nil {
		return errors.Wrap(err, "failed to read from flash")
	}

	testing.ContextLog(ctx, "Checking that value read matches the flashed version")
	cmd = d.Command("cmp", buildFwFile, fileReadFromFlash)
	if err := cmd.Run(ctx); err != nil {
		return errors.Wrap(err, "file read from flash does not match original fw file")
	}

	hostBoard, err := reporters.New(d).Board(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to query host board")
	}
	// On zork, an AP reboot is needed after using flash_fp_mcu.
	if hostBoard == "zork" {
		if err := d.Reboot(ctx); err != nil {
			return errors.Wrap(err, "failed to reboot DUT")
		}
	}
	if _, err := fingerprint.CheckFirmwareIsFunctional(ctx, d); err != nil {
		return errors.Wrap(err, "firmware is not functional after reading flash")
	}
	return nil
}
