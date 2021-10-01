// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"path/filepath"
	"regexp"
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
		Func: FpRDP1,
		Desc: "Validate read protection (RDP) level 1 of the fingerprint firmware works as expected",
		Contacts: []string{
			"josienordrum@google.com", // Test author
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

func FpRDP1(ctx context.Context, s *testing.State) {
	d, err := rpcdut.NewRPCDUT(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect RPCDUT: ", err)
	}
	defer d.Close(ctx)

	// Set both HW write protect false and SW write protect true to get RDP1 state.
	servoSpec, _ := s.Var("servo")
	t, err := fingerprint.NewFirmwareTest(ctx, d, servoSpec, s.OutDir(), false, true)
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

	// Test RDP1 without modiying RDP level
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
	if err := testRDP1(ctx, d, t.BuildFwFile(), false, t.NeedsRebootAfterFlashing()); err != nil {
		s.Fatal("Failed to validate RDP1  without changing RDP level: ", err)
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
	if err := testRDP1(ctx, d, t.BuildFwFile(), true, t.NeedsRebootAfterFlashing()); err != nil {
		s.Fatal("Failed to validate RDP1 while setting RDP to level 0: ", err)
	}
}

// testRDP1 tests RDP1 functionality by trying to read from flash.
func testRDP1(ctx context.Context, d *rpcdut.RPCDUT, buildFwFile string, removeFlashReadProtect, needsReboot bool) (e error) {
	var fileReadFromFlash string
	var args []string

	fs := dutfs.NewClient(d.RPC().Conn)

	tempdirPath, err := fs.TempDir(ctx, "", "fingerprint_rdp1_*")
	if err != nil {
		return errors.Wrap(err, "failed to create remote temp directory")
	}
	defer func() {
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
	}()

	if removeFlashReadProtect {
		// Use different file name to avoid errors in removing file.
		fileReadFromFlash = filepath.Join(tempdirPath, "test_rdp1.bin")
		args = []string{"--noservices", "--read", fileReadFromFlash}
	} else {
		fileReadFromFlash = filepath.Join(tempdirPath, "test_rdp1_noremove.bin")
		args = []string{"--noservices", "--read",
			"--noremove_flash_read_protect", fileReadFromFlash}
	}
	cmd := d.Conn().CommandContext(ctx, "flash_fp_mcu", args...)
	out, err := cmd.CombinedOutput()
	testing.ContextLog(ctx, "flash_fp_mcu output:", "\n", string(out))
	if err == nil {
		return errors.Wrap(err, "should not be able to successfully flash")
	}

	if removeFlashReadProtect {
		testing.ContextLog(ctx, "Checking that file_read_from_flash is made entirely of 0xFF bytes")
		cmd = d.Conn().CommandContext(ctx, "stat", "--printf", "%s", fileReadFromFlash)
		fsize, err := cmd.CombinedOutput()
		if err != nil {
			return errors.Wrap(err, "failed to get size of test file")
		}
		cmd = d.Conn().CommandContext(ctx, "stat", "--printf", "%s", buildFwFile)
		origsize, err := cmd.CombinedOutput()
		if err != nil {
			return errors.Wrap(err, "failed to get size of original file")
		}
		if string(fsize) != string(origsize) {
			return errors.New("file read from flash doesn't match original file size")
		}
		rx := regexp.MustCompile(`0000000 ffff ffff ffff ffff ffff ffff ffff ffff\n\*\n[0-9]+\n$`)
		cmd = d.Conn().CommandContext(ctx, "hexdump", buildFwFile)
		modfile, err := cmd.CombinedOutput()
		if err != nil {
			return errors.Wrap(err, "failed to get hexdump of test file")
		}
		if rx.Find(modfile) != nil {
			return errors.New("test file does not contain all 0xFF bytes")
		}
	} else {
		testing.ContextLog(ctx, "Checking file_read_from_flash is empty")
		cmd = d.Conn().CommandContext(ctx, "stat", "--printf", "%s", fileReadFromFlash)
		out, err = cmd.CombinedOutput()
		if err != nil {
			return errors.Wrap(err, "failed to get size of test file")
		}
		if string(out) != "0" {
			return errors.New("file read from flash is not empty")
		}
	}

	if needsReboot {
		testing.ContextLog(ctx, "Rebooting")
		if err := d.Reboot(ctx); err != nil {
			return errors.Wrap(err, "failed to reboot DUT")
		}
	}

	_, err = fingerprint.CheckFirmwareIsFunctional(ctx, d.DUT())

	if removeFlashReadProtect {
		if err == nil {
			return errors.New("firmware should not be functional")
		}
	} else {
		if err != nil {
			return errors.Wrap(err, "firmware is not functional after reading flash")
		}
	}
	return nil
}
