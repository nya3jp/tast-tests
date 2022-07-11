// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"

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
		Func: FpRDP1,
		Desc: "Validate read protection (RDP) level 1 of the fingerprint firmware works as expected",
		Contacts: []string{
			"josienordrum@google.com", // Test author
			"tomhughes@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      9 * time.Minute,
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
		ServiceDeps:  []string{"tast.cros.platform.UpstartService", dutfs.ServiceName},
		Vars:         []string{"servo"},
	})
}

// FpRDP1 test setup enables RDP level 1 by enabling hardware write protect and then
// enabling software write protect (with reboot of the EC); the test setup then
// disables hardware write protect, so that we can perform reads and change RDP
// levels through the bootloader (only accessible when HW write protect is
// disabled).
//
// When the test script starts, a read through the bootloader is done without
// disabling flash protection (changing RDP state). We verify that we are unable
// to read any data.
//
// Next a read through the bootloader is done, while also disabling flash
// protection (changing to RDP level 0), which triggers a mass erase. We verify
// that the bytes in the output are all 0xFF and that the firmware is no longer
// functional.
func FpRDP1(ctx context.Context, s *testing.State) {
	d, err := rpcdut.NewRPCDUT(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect RPCDUT: ", err)
	}
	defer d.Close(ctx)

	servoSpec, ok := s.Var("servo")
	if !ok {
		servoSpec = ""
	}
	// Set both HW write protect false and SW write protect true to get RDP1 state.
	t, err := fingerprint.NewFirmwareTest(ctx, d, servoSpec, s.OutDir(), false /*HW protect*/, true /*SW protect*/)
	if err != nil {
		s.Fatal("Failed to create new firmware test: ", err)
	}
	cleanupCtx := ctx
	defer func() {
		if err := t.Close(cleanupCtx); err != nil {
			s.Fatal("Failed to clean up: ", err)
		}
	}()
	ctx, cancel := ctxutil.Shorten(ctx, t.CleanupTime())
	defer cancel()

	// Test RDP1 without modifying RDP level
	// Given:
	// * Hardware write protect is disabled
	//		(so we can use bootloader to read and change RDP level)
	// * Software write protect is enabled
	// * RDP is at level 1
	//
	// Then:
	// * Reading from flash without changing the RDP level should fail (and we should not have read any bytes from flash).
	// * The firmware should still be functional because mass erase is NOT triggered since we are NOT changing the RDP level.
	testing.ContextLog(ctx, "Reading firmware without modifying RDP level")
	if err := testRDP1(ctx, s.OutDir(), d, t.BuildFwFile(), false /*preserve RDP level*/, t.NeedsRebootAfterFlashing()); err != nil {
		s.Fatal("Failed to validate RDP1 without changing RDP level: ", err)
	}

	// Test RDP1 while setting RDP0
	// Given:
	// * Hardware write protect is disabled
	// * Software write protect is disabled
	// * RDP is at level 0
	//
	// Then:
	// * Setting the RDP level to 0 (after being at level 1) should trigger a mass erase.
	// * A mass erase sets all flash bytes to 0xFF, so all bytes read from flash should have that value.
	testing.ContextLog(ctx, "Reading firmware while setting RDP to level 0")
	if err := testRDP1(ctx, s.OutDir(), d, t.BuildFwFile(), true /*remove Flash Read Protect*/, t.NeedsRebootAfterFlashing()); err != nil {
		s.Fatal("Failed to validate RDP1 while setting RDP to level 0: ", err)
	}
}

// testRDP1 tests RDP1 functionality by trying to read from flash.
func testRDP1(ctx context.Context, outdir string, d *rpcdut.RPCDUT, buildFwFile string, removeFlashReadProtect, needsReboot bool) (retErr error) {
	fs := dutfs.NewClient(d.RPC().Conn)

	tempdirPath, err := fs.TempDir(ctx, "", "fingerprint_rdp1_*")
	if err != nil {
		return errors.Wrap(err, "failed to create remote temp directory")
	}
	defer func() {
		if fs != nil {
			err := fs.RemoveAll(ctx, tempdirPath)
			if os.IsNotExist(err) {
				retErr = errors.Wrapf(err, "failed to remove temp directory: %q", tempdirPath)
			}
		} else {
			testing.ContextLog(ctx, "DUTFS connection not available. Skip removing the temp directory")
		}
	}()

	fileReadFromFlash := filepath.Join(tempdirPath, "test_rdp1.bin")
	args := []string{"--noservices", "--read"}
	if !removeFlashReadProtect {
		// Use different file name to avoid errors in removing file.
		fileReadFromFlash = filepath.Join(tempdirPath, "test_rdp1_noremove.bin")
		args = append(args, "--noremove_flash_read_protect")
	}
	args = append(args, fileReadFromFlash)
	cmd := d.Conn().CommandContext(ctx, "flash_fp_mcu", args...)
	logFile, err := os.Create(filepath.Join(outdir, "flash_fpmcu_removeReadProtect_"+strconv.FormatBool(removeFlashReadProtect)+"_output.txt"))
	if err != nil {
		return errors.Wrap(err, "failed to create file for logging output")
	}
	defer logFile.Close()
	cmd.Stdout = logFile
	err = cmd.Run()
	// Schedule DUT reboot if needed. This is intended to reboot DUT on SSH
	// or file checking failures. AlreadyRebooted variable is used to
	// prevent from unnecessary reboot if DUT was already rebooted after
	// checking flash contents.
	alreadyRebooted := false
	if needsReboot {
		defer func() {
			if !alreadyRebooted {
				testing.ContextLog(ctx, "Rebooting")
				// Reboot invalidates RPC client handle saved in
				// fs. Assign nil to fs to mark it explicitly as
				// invalid.
				fs = nil
				if err := d.Reboot(ctx); err != nil {
					testing.ContextLog(ctx, "Failed to reboot DUT")
				} else {
					fs = dutfs.NewClient(d.RPC().Conn)
				}
			}
		}()
	}

	exitStatus := 0
	if err != nil {
		errcode, ok := err.(*ssh.ExitError)
		if !ok {
			return errors.New("failed to return Exit Error")
		}
		if errcode.Signal() != "" {
			return errors.New("flash fpmcu terminated from signal: " + errcode.Signal())
		}
		exitStatus = errcode.ExitStatus()
	}

	// Flash_fp_mcu in RDP1 run with removeReadProtect set to false should
	// fail and not read any bytes from flash.
	// Flash_fp_mcu in RDP1 run with removeReadProtect set to true should
	// trigger mass erase which sets all bytes to 0xFF. It will return zero
	// exit status on boards that require reboot (e.g. zork) or non-zero on
	// boards that don't require reboot (because flash_fp_mcu fails to check
	// if firmware is functional).
	testing.ContextLogf(ctx, "flash_fp_mcu exited with status %d", exitStatus)
	expectFlashFpMcuSuccess := false
	if removeFlashReadProtect && needsReboot {
		expectFlashFpMcuSuccess = true
	}
	if expectFlashFpMcuSuccess && exitStatus != 0 {
		return errors.New("flash_fp_mcu failed, but should have succeeded")
	} else if !expectFlashFpMcuSuccess && exitStatus == 0 {
		return errors.New("flash_fp_mcu succeeded, but should have failed")
	}

	if removeFlashReadProtect {
		testing.ContextLog(ctx, "Checking that fileReadFromFlash is the same size as build file")
		testinfo, err := fs.Stat(ctx, fileReadFromFlash)
		if err != nil {
			return errors.Wrap(err, "failed to get info for test file")
		}
		buildinfo, err := fs.Stat(ctx, buildFwFile)
		if err != nil {
			return errors.Wrap(err, "failed to get info for build file")
		}
		if testinfo.Size() != buildinfo.Size() {
			return errors.New("file read from flash doesn't match original file size")
		}

		testing.ContextLog(ctx, "Checking that test file is made entirely of 0xFF bytes")
		testfile, err := fs.ReadFile(ctx, fileReadFromFlash)
		if err != nil {
			return errors.Wrap(err, "failed to read test file")
		}
		for _, b := range testfile {
			if b != byte(0xFF) {
				return errors.New("test file does not contain all 0xFF bytes")
			}
		}
	} else {
		testing.ContextLog(ctx, "Checking fileReadFromFlash is empty")
		testinfo, err := fs.Stat(ctx, fileReadFromFlash)
		if err != nil {
			return errors.Wrap(err, "failed to get size of test file")
		}
		if testinfo.Size() != 0 {
			return errors.New("file read from flash is not empty")
		}
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
		alreadyRebooted = true
	}

	_, err = fingerprint.CheckFirmwareIsFunctional(ctx, d.DUT())

	if removeFlashReadProtect {
		errcode, ok := err.(*ssh.ExitError)
		if !ok {
			return errors.New("failed to return Exit Error")
		}
		if errcode.Signal() != "" {
			return errors.New("firmware check terminated from signal: " + errcode.Signal())
		}
		if errcode.ExitStatus() == 0 {
			return errors.New("firmware check should not complete successfully")
		}
	} else {
		if err != nil {
			return errors.Wrap(err, "firmware is not functional after reading flash")
		}
	}
	return nil
}
