// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
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
		Func: FpROCanUpdateRW,
		Desc: "Verify that RO can update RW",
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

type testRWFlashParams struct {
	firmwarePath                string
	expectedROVersion           string
	expectedRWVersion           string
	expectedRunningFirmwareCopy fingerprint.FWImageType
}

func testFlashingRWFirmware(ctx context.Context, d *rpcdut.RPCDUT, params *testRWFlashParams) error {
	testing.ContextLog(ctx, "Flashing RW firmware: ", params.firmwarePath)
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
	testing.ContextLog(ctx, "Checking that rollback meets expected values")
	if err := fingerprint.CheckRollbackSetToInitialValue(ctx, d); err != nil {
		return errors.Wrap(err, "rollback not set to initial value")
	}

	return nil
}

// FpROCanUpdateRW flashes RW firmware with a version string that ends in '.rb0'
// (has rollback ID '0') and validates that it is running. Then flashes RW
// firmware with version string that ends in '.dev' (also has rollback ID '0')
// and validates that it is running.
func FpROCanUpdateRW(ctx context.Context, s *testing.State) {
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
	folderpath := filepath.Join("/", "mnt", "stateful_partition", fmt.Sprintf("fpimages_%d", time.Now().Unix()))
	err = dutfs.NewClient(d.RPC().Conn).MkDir(ctx, folderpath, 0755)
	if err != nil {
		s.Fatal("Failed to create remote working directory: ", err)
	}
	testing.ContextLog(ctx, "Created non-temporary fptast directory")
	// Generate test images to flash to RW.
	testImages, err := fingerprint.GenerateTestFirmwareImages(ctx, d, s.DataPath(fingerprint.Futility), s.DataPath(fingerprint.DevKeyForFPBoard(fpBoard)), fpBoard, buildFwFile, folderpath)
	if err != nil {
		s.Fatal("Failed to generate test images: ", err)
	}
	firmwareFile := fingerprint.NewFirmwareFile(testImages[fingerprint.TestImageTypeDev].Path, fingerprint.KeyTypeDev, testImages[fingerprint.TestImageTypeDev].ROVersion, testImages[fingerprint.TestImageTypeDev].RWVersion)
	// Set both HW write protect and SW write protect true.
	t, err := fingerprint.NewFirmwareTest(ctx, d, servoSpec, s.OutDir(), firmwareFile, true /*HW protect*/, true /*SW protect*/)
	if err != nil {
		s.Fatal("Failed to create new firmware test: ", err)
	}
	cleanupCtx := ctx
	defer func() {
		s.Log("Delete fptast directory and contained files from DUT")
		dutfs.NewClient(d.RPC().Conn).RemoveAll(ctx, folderpath)
		if err != nil {
			s.Fatal("Failed to delete dir: ", folderpath, err)
		}
		if err := t.Close(cleanupCtx); err != nil {
			s.Fatal("Failed to clean up: ", err)
		}
	}()
	ctx, cancel := ctxutil.Shorten(ctx, t.CleanupTime())
	defer cancel()

	testing.ContextLog(ctx, "Flashing RW firmware with rollback ID of '0'")
	if err := testFlashingRWFirmware(ctx, d,
		&testRWFlashParams{
			firmwarePath: testImages[fingerprint.TestImageTypeDevRollbackZero].Path,
			// RO version should remain unchanged.
			expectedROVersion: testImages[fingerprint.TestImageTypeDev].ROVersion,
			// RW version should match what we requested to be flashed.
			expectedRWVersion: testImages[fingerprint.TestImageTypeDevRollbackZero].RWVersion,
			// Signature check will pass, so we should be running RW.
			expectedRunningFirmwareCopy: fingerprint.ImageTypeRW,
		}); err != nil {
		s.Fatal("Rollback ID 0 test failed: ", err)
	}
	testing.ContextLog(ctx, "Flashing RW with dev firmware")
	if err := testFlashingRWFirmware(ctx, d,
		&testRWFlashParams{
			firmwarePath: testImages[fingerprint.TestImageTypeDev].Path,
			// RO version should remain unchanged.
			expectedROVersion: testImages[fingerprint.TestImageTypeDev].ROVersion,
			// RW version should match what we requested to be flashed.
			expectedRWVersion: testImages[fingerprint.TestImageTypeDev].RWVersion,
			// Signature check will pass, so we should be running RW.
			expectedRunningFirmwareCopy: fingerprint.ImageTypeRW,
		}); err != nil {
		s.Fatal("Dev firmware test failed: ", err)
	}
}
