// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/fingerprint"
	"chromiumos/tast/remote/firmware/fingerprint/rpcdut"
	"chromiumos/tast/shutil"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpObeysRollback,
		Desc: "Verify that rollback state is obeyed",
		Contacts: []string{
			"josienordrum@google.com", // Test author
			"tomhughes@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      8 * time.Minute,
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
		ServiceDeps:  []string{"tast.cros.platform.UpstartService", dutfs.ServiceName},
		Vars:         []string{"servo"},
		Data: []string{
			fingerprint.Futility,
			fingerprint.BloonchipperDevKey,
			fingerprint.DartmonkeyDevKey,
			fingerprint.NamiFPDevKey,
			fingerprint.NocturneFPDevKey,
		},
	})
}

type testRollbackParams struct {
	firmwarePath                  string
	expectedROVersion             string
	expectedRWVersion             string
	expectedRunningFirmwareCopy   fingerprint.FWImageType
	expectedFingerprintTaskStatus string
	expectedRollbackState         fingerprint.RollbackState
}

func testFlashingFirmwareRollback(ctx context.Context, d *rpcdut.RPCDUT, params *testRollbackParams) error {
	testing.ContextLog(ctx, "Flashing firmware: ", params.firmwarePath)
	if err := fingerprint.FlashRWFirmware(ctx, d, params.firmwarePath); err != nil {
		return errors.Wrapf(err, "failed to flash firmware: %q", params.firmwarePath)
	}

	testing.ContextLog(ctx, "Checking for versions: RO: ", params.expectedROVersion, ", RW: ", params.expectedRWVersion)
	if err := fingerprint.CheckRunningFirmwareVersionMatches(ctx, d, params.expectedROVersion, params.expectedRWVersion); err != nil {
		return errors.Wrap(err, "unexpected firmware version")
	}

	testing.ContextLog(ctx, "Checking that ", params.expectedRunningFirmwareCopy, " firmware is running")
	if err := fingerprint.CheckRunningFirmwareCopy(ctx, d.DUT(), params.expectedRunningFirmwareCopy); err != nil {
		return errors.Wrap(err, "running unexpected firmware copy")
	}

	testing.ContextLog(ctx, "Checking that Fingerprint Task Status is as expected")
	cmd := []string{"ectool", "--name=cros_fp", "fpinfo"}
	testing.ContextLogf(ctx, "Running command: %s", shutil.EscapeSlice(cmd))
	out, err := d.Conn().CommandContext(ctx, cmd[0], cmd[1:]...).Output(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "error checking fingerprint task ")
	}
	if string(out) != params.expectedFingerprintTaskStatus {
		return errors.New("Unexpected fingerprint task status. Output is: " + string(out))
	}

	testing.ContextLog(ctx, "Checking that rollback meets expected values")
	if err := fingerprint.CheckRollbackState(ctx, d, params.expectedRollbackState); err != nil {
		return errors.Wrap(err, "rollback not set to initial value")
	}

	return nil
}

// FpObeysRollback flashes new RW firmware with a rollback ID of '1' and verifies that all
// rollback state is set correctly. Then attempts to flash RW firmware with
// rollback ID of '0' and verifies that the RO version of firmware is running
// (i.e., not running older version). Finally, flashes RW firmware with rollback
// ID of '9' and validates that the RW version of '9' is running.
func FpObeysRollback(ctx context.Context, s *testing.State) {
	d, err := rpcdut.NewRPCDUT(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect RPCDUT: ", err)
	}
	defer d.Close(ctx)

	servoSpec, ok := s.Var("servo")
	if !ok {
		servoSpec = ""
	}
	fpBoard, err := fingerprint.Board(ctx, d)
	if err != nil {
		s.Fatal("Failed to get fingerprint board: ", err)
	}
	buildFwFile, err := fingerprint.FirmwarePath(ctx, d, fpBoard)
	if err != nil {
		s.Fatal("Failed to get build firmware file path: ", err)
	}

	// Genereate test images to flash to RW.
	testImages, err := fingerprint.GenerateTestFirmwareImages(ctx, d, s.DataPath(fingerprint.Futility), s.DataPath(fingerprint.DevKeyForFPBoard(fpBoard)), fpBoard, buildFwFile, s.OutDir())
	if err != nil {
		s.Fatal("Failed to generate test images: ", err)
	}

	if err := fsutil.CopyFile(testImages[fingerprint.TestImageTypeDev].Path, s.OutDir()); err != nil {
		s.Fatal("Failed to copy test image file")
	}

	// Set both HW write protect and SW write protect true.
	t, err := fingerprint.NewFirmwareTest(ctx, d, servoSpec, s.OutDir(), testImages[fingerprint.TestImageTypeDev].Path, true /*HW protect*/, true /*SW protect*/)
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

	testing.ContextLog(ctx, "Flashing RW firmware with rollback ID of '1'")
	if err := testFlashingFirmwareRollback(ctx, d,
		&testRollbackParams{
			firmwarePath: testImages[fingerprint.TestImageTypeDevRollbackOne].Path,
			// RO version should remain unchanged.
			expectedROVersion: testImages[fingerprint.TestImageTypeOriginal].ROVersion,
			// RW version should match what we requested to be flashed.
			expectedRWVersion: testImages[fingerprint.TestImageTypeDevRollbackOne].RWVersion,
			// Signature check will pass, so we should be running RW.
			expectedRunningFirmwareCopy: fingerprint.ImageTypeRW,
			// Fingerprint task should be running.
			expectedFingerprintTaskStatus: "F",
			// Expected rollback state.
			expectedRollbackState: fingerprint.RollbackState{
				BlockID: 2, MinVersion: 1, RWVersion: 1},
		}); err != nil {
		s.Fatal("Rollback ID 1 test failed: ", err)
	}

	testing.ContextLog(ctx, "Flashing RW firmware with rollback ID of '0'")
	if err := testFlashingFirmwareRollback(ctx, d,
		&testRollbackParams{
			firmwarePath: testImages[fingerprint.TestImageTypeDevRollbackZero].Path,
			// RO version should remain unchanged.
			expectedROVersion: testImages[fingerprint.TestImageTypeOriginal].ROVersion,
			// RW version should match what we requested to be flashed.
			expectedRWVersion: testImages[fingerprint.TestImageTypeDevRollbackZero].RWVersion,
			// Signature check will fail, so we should be running RO.
			expectedRunningFirmwareCopy: fingerprint.ImageTypeRO,
			// Fingerprint task should be running.
			expectedFingerprintTaskStatus: "",
			// Expected rollback state.
			expectedRollbackState: fingerprint.RollbackState{
				BlockID: 2, MinVersion: 1, RWVersion: 0},
		}); err != nil {
		s.Fatal("Rollback ID 0 test failed: ", err)
	}

	testing.ContextLog(ctx, "Flashing RW firmware with rollback ID of '9'")
	if err := testFlashingFirmwareRollback(ctx, d,
		&testRollbackParams{
			firmwarePath: testImages[fingerprint.TestImageTypeDevRollbackNine].Path,
			// RO version should remain unchanged.
			expectedROVersion: testImages[fingerprint.TestImageTypeOriginal].ROVersion,
			// RW version should match what we requested to be flashed.
			expectedRWVersion: testImages[fingerprint.TestImageTypeDevRollbackNine].RWVersion,
			// Signature check will pass, so we should be running RW.
			expectedRunningFirmwareCopy: fingerprint.ImageTypeRW,
			// Fingerprint task should be running.
			expectedFingerprintTaskStatus: "F",
			// Expected rollback state.
			expectedRollbackState: fingerprint.RollbackState{
				BlockID: 3, MinVersion: 9, RWVersion: 9},
		}); err != nil {
		s.Fatal("Rollback ID 9 test failed: ", err)
	}

}
