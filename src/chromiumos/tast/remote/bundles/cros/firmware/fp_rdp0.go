// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"path/filepath"
	"strings"

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
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
		Vars:         []string{"servo"},
	})
}

func FpRDP0(ctx context.Context, s *testing.State) {
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
	defer func() {
		testing.ContextLog(ctx, "Starting cleanup at the end of test")
		defer pxy.Close(ctx)
		if err := pxy.Servo().SetStringAndCheck(ctx, servo.FWWPState, "force_off"); err != nil {
			s.Fatal("Failed to disable HW write protect: ", err)
		}

		if err := fingerprint.FlashFirmware(ctx, d); err != nil {
			s.Error("Failed to flash original FP firmware: ", err)
		}
		if err := fingerprint.InitializeEntropy(ctx, d); err != nil {
			s.Error("Failed to initiailze entropy: ", err)
		}
		if err := d.Reboot(ctx); err != nil {
			s.Error("Failed to reboot DUT: ", err)
		}

		if err := pxy.Servo().SetStringAndCheck(ctx, servo.FWWPState, "force_on"); err != nil {
			s.Fatal("Failed to enable HW write protect: ", err)
		}
	}()

	// TODO(yichengli): Check the FPMCU is running expected firmware version.

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
	if firmwareCopy != "RW" {
		s.Fatal("Not running RW firmware")
	}

	// Rollback should be unset for this test.
	if err := fingerprint.CheckRollbackState(ctx, d, 0, 0, 0); err != nil {
		s.Fatal("Failed to validate rollback state: ", err)
	}

	if _, err := fingerprint.CheckFirmwareIsFunctional(ctx, d); err != nil {
		s.Fatal("Firmware is not functional after initialization: ", err)
	}

	// Prepare a temporary working directory on DUT.
	tempdir, err := d.Command("mktemp", "-d", "/tmp/fingerprint_rdp0_XXXXXX").Output(ctx)
	if err != nil {
		s.Fatal("Failed to create remote temp directory: ", err)
	}
	tempdirPath := strings.TrimSpace(string(tempdir))
	defer d.Command("rm", "-r", tempdirPath).Run(ctx)

	if err := testRDP0WithoutChangingRDPLevel(ctx, d, buildFwFile, tempdirPath); err != nil {
		s.Fatal("Failed to validate RDP0 without changing RDP level: ", err)
	}

	if err := testRDP0WhileSettingRDPLevel0(ctx, d, buildFwFile, tempdirPath); err != nil {
		s.Fatal("Failed to validate RDP0 while setting RDP to level 0: ", err)
	}
}

// testRDP0WithoutChangingRDPLevel tests RDP0 functionality without chaning RDP level.
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
func testRDP0WithoutChangingRDPLevel(ctx context.Context, d *dut.DUT, buildFwFile, tempdirPath string) error {
	testing.ContextLog(ctx, "Reading firmware without modifying RDP level")
	fileReadFromFlash := filepath.Join(tempdirPath, "test_keep_rdp.bin")
	cmd := d.Command("flash_fp_mcu", "--read", "--noremove_flash_read_protect", fileReadFromFlash)
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

// testRDP0WhileSettingRDPLevel0 tests RDP0 functionality while setting RDP level 0.
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
func testRDP0WhileSettingRDPLevel0(ctx context.Context, d *dut.DUT, buildFwFile, tempdirPath string) error {
	testing.ContextLog(ctx, "Reading firmware while setting RDP to level 0")
	fileReadFromFlash := filepath.Join(tempdirPath, "test_change_rdp.bin")
	cmd := d.Command("flash_fp_mcu", "--read", fileReadFromFlash)
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
