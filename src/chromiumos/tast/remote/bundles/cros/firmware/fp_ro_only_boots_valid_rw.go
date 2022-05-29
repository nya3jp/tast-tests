// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
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
		Func: FpROOnlyBootsValidRW,
		Desc: "Verify the RO fingerprint firmware only boots valid RW firmware",
		Contacts: []string{
			"tomhughes@chromium.org", // Test author
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      7 * time.Minute,
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

type testParams struct {
	firmwarePath                string
	expectedROVersion           string
	expectedRWVersion           string
	expectedRunningFirmwareCopy fingerprint.FWImageType
}

func testFlashingFirmwareVersion(ctx context.Context, d *rpcdut.RPCDUT, params *testParams) error {
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
		return errors.Wrap(err, "not running RO firmware")
	}

	testing.ContextLog(ctx, "Checking that rollback remains unchanged")
	if err := fingerprint.CheckRollbackSetToInitialValue(ctx, d); err != nil {
		return errors.Wrap(err, "rollback not set to initial value")
	}

	return nil
}

func FpROOnlyBootsValidRW(ctx context.Context, s *testing.State) {
	d, err := rpcdut.NewRPCDUT(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect RPCDUT: ", err)
	}
	defer d.Close(ctx)

	servoSpec, ok := s.Var("servo")
	if !ok {
		servoSpec = ""
	}
	t, err := fingerprint.NewFirmwareTest(ctx, d, servoSpec, s.OutDir(), true, true)
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

	testImages, err := fingerprint.GenerateTestFirmwareImages(ctx, d, s.DataPath(fingerprint.Futility), s.DataPath(fingerprint.DevKeyForFPBoard(t.FPBoard())), t.FPBoard(), t.BuildFwFile(), t.DUTTempDir())
	if err != nil {
		s.Fatal("Failed to generate test images: ", err)
	}

	if err := fingerprint.CheckRunningFirmwareCopy(ctx, d.DUT(), fingerprint.ImageTypeRW); err != nil {
		s.Fatal("Test expects RW firmware copy to be running")
	}

	// Hardware write protect must be enabled for the test to work correctly.
	if err := fingerprint.CheckWriteProtectStateCorrect(ctx, d.DUT(), true, true); err != nil {
		s.Fatal("Failed to validate write protect settings: ", err)
	}

	// Starts with MP-signed firmware. Then successively tries to flash three versions
	// to RW: dev, corrupted first byte, and corrupted last byte. Each of these should
	// flash successfully, but fail to boot (i.e., stay in RO mode). Finally,
	// flash an MP-signed version, which should successfully boot to RW.

	if err := testFlashingFirmwareVersion(ctx, d,
		&testParams{
			firmwarePath: testImages[fingerprint.TestImageTypeDev].Path,
			// RO version should remain unchanged.
			expectedROVersion: testImages[fingerprint.TestImageTypeOriginal].ROVersion,
			// RW version should match what we requested to be flashed.
			expectedRWVersion: testImages[fingerprint.TestImageTypeDev].RWVersion,
			// Signature check will fail, so we should be running RO.
			expectedRunningFirmwareCopy: fingerprint.ImageTypeRO,
		}); err != nil {
		s.Fatal("Dev key signed test failed: ", err)
	}

	// Note that the corrupted version has the same version string as the original version.
	if err := testFlashingFirmwareVersion(ctx, d,
		&testParams{
			firmwarePath: testImages[fingerprint.TestImageTypeCorruptFirstByte].Path,
			// RO version should remain unchanged.
			expectedROVersion: testImages[fingerprint.TestImageTypeOriginal].ROVersion,
			// RW version should match what we requested to be flashed.
			expectedRWVersion: testImages[fingerprint.TestImageTypeCorruptFirstByte].RWVersion,
			// Signature check will fail, so we should be running RO.
			expectedRunningFirmwareCopy: fingerprint.ImageTypeRO,
		}); err != nil {
		s.Fatal("Corrupt first byte test failed: ", err)
	}

	if err := testFlashingFirmwareVersion(ctx, d,
		&testParams{
			firmwarePath: testImages[fingerprint.TestImageTypeCorruptLastByte].Path,
			// RO version should remain unchanged.
			expectedROVersion: testImages[fingerprint.TestImageTypeOriginal].ROVersion,
			// RW version should match what we requested to be flashed.
			expectedRWVersion: testImages[fingerprint.TestImageTypeCorruptLastByte].RWVersion,
			// Signature check will fail, so we should be running RO.
			expectedRunningFirmwareCopy: fingerprint.ImageTypeRO,
		}); err != nil {
		s.Fatal("Corrupt last byte test failed: ", err)
	}

	if err := testFlashingFirmwareVersion(ctx, d,
		&testParams{
			firmwarePath: testImages[fingerprint.TestImageTypeOriginal].Path,
			// RO version should remain unchanged.
			expectedROVersion: testImages[fingerprint.TestImageTypeOriginal].ROVersion,
			// RW version should match what we requested to be flashed.
			expectedRWVersion: testImages[fingerprint.TestImageTypeOriginal].RWVersion,
			// Signature check will succeed, so we should be running RW.
			expectedRunningFirmwareCopy: fingerprint.ImageTypeRW,
		}); err != nil {
		s.Fatal("Good firmware test failed: ", err)
	}
}
