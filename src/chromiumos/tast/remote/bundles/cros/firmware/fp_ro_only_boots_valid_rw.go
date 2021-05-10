// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/fingerprint"
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
			fingerprint.GenTestImagesScript,
			fingerprint.Futility,
			fingerprint.BloonchipperDevKey,
			fingerprint.DartmonkeyDevKey,
			fingerprint.NamiFPDevKey,
			fingerprint.NocturneFPDevKey,
		},
	})
}

func testFlashingBadFirmwareVersion(ctx context.Context, d *dut.DUT, fs *dutfs.Client, testImages fingerprint.TestImages, imageType fingerprint.TestImageType) error {
	firmwarePath := testImages[imageType].Path
	testing.ContextLog(ctx, "Flashing bad firmware: ", firmwarePath)
	if err := fingerprint.FlashRWFirmware(ctx, d, fs, firmwarePath); err != nil {
		return errors.Wrapf(err, "failed to flash firmware: %q", firmwarePath)
	}

	// RO version should remain unchanged
	expectedROVersion := testImages[fingerprint.TestImageTypeOriginal].ROVersion
	// RW version should match what was just flashed
	expectedRWVersion := testImages[imageType].RWVersion
	testing.ContextLog(ctx, "Checking for versions: RO: ", expectedROVersion, ", RW: ", expectedRWVersion)
	if err := fingerprint.CheckRunningFirmwareVersionMatches(ctx, d, expectedROVersion, expectedRWVersion); err != nil {
		return errors.Wrap(err, "unexpected firmware version")
	}

	testing.ContextLog(ctx, "Checking that RO firmware is running")
	if err := fingerprint.CheckRunningFirmwareCopy(ctx, d, fingerprint.ImageTypeRO); err != nil {
		return errors.Wrap(err, "not running RO firmware")
	}

	testing.ContextLog(ctx, "Checking that rollback remains unchanged")
	if err := fingerprint.CheckRollbackSetToInitialValue(ctx, d); err != nil {
		return errors.Wrap(err, "rollback not set to initial value")
	}

	return nil
}

func testFlashingGoodFirmwareVersion(ctx context.Context, d *dut.DUT, fs *dutfs.Client, testImages fingerprint.TestImages) error {
	firmwarePath := testImages[fingerprint.TestImageTypeOriginal].Path
	testing.ContextLog(ctx, "Flashing good firmware: ", firmwarePath)
	if err := fingerprint.FlashRWFirmware(ctx, d, fs, firmwarePath); err != nil {
		return errors.Wrapf(err, "failed to flash firmware: %q", firmwarePath)
	}

	expectedROVersion := testImages[fingerprint.TestImageTypeOriginal].ROVersion
	expectedRWVersion := testImages[fingerprint.TestImageTypeOriginal].RWVersion
	testing.ContextLog(ctx, "Checking for versions: RO: ", expectedROVersion, ", RW: ", expectedRWVersion)
	if err := fingerprint.CheckRunningFirmwareVersionMatches(ctx, d, expectedROVersion, expectedRWVersion); err != nil {
		return errors.Wrap(err, "unexpected firmware version")
	}

	testing.ContextLog(ctx, "Checking that RW firmware is running")
	if err := fingerprint.CheckRunningFirmwareCopy(ctx, d, fingerprint.ImageTypeRW); err != nil {
		return errors.Wrap(err, "not running RW firmware")
	}

	testing.ContextLog(ctx, "Checking that rollback remains unchanged")
	if err := fingerprint.CheckRollbackSetToInitialValue(ctx, d); err != nil {
		return errors.Wrap(err, "rollback not set to initial value")
	}

	return nil
}

func FpROOnlyBootsValidRW(ctx context.Context, s *testing.State) {
	t, err := fingerprint.NewFirmwareTest(ctx, s.DUT(), s.RequiredVar("servo"), s.RPCHint(), s.OutDir(), true, true)
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

	d := t.DUT()

	// TODO: check that we're in RW and have hardware write protect enabled

	testImages, err := fingerprint.GenerateTestFirmwareImages(ctx, d, t.DutfsClient(), s.DataPath(fingerprint.DevKeyForFPBoard(t.FPBoard())), t.FPBoard(), t.BuildFwFile(), t.DUTTempDir())
	if err != nil {
		s.Fatal("Failed to generate test images: ", err)
	}

	// Starts with MP-signed firmware. Then successively tries to flash three versions
	// to RW: dev, corrupted first byte, and corrupted last byte. Each of these should
	// flash successfully, but fail to boot (i.e., stay in RO mode). Finally,
	// flash an MP-signed version, which should successfully boot to RW.

	if err := testFlashingBadFirmwareVersion(ctx, d, t.DutfsClient(), testImages, fingerprint.TestImageTypeDev); err != nil {
		s.Fatal("Dev key signed test failed: ", err)
	}

	// Note that the corrupted version has the same version string as the original version
	if err := testFlashingBadFirmwareVersion(ctx, d, t.DutfsClient(), testImages, fingerprint.TestImageTypeCorruptFirstByte); err != nil {
		s.Fatal("Corrupt first byte test failed: ", err)
	}

	if err := testFlashingBadFirmwareVersion(ctx, d, t.DutfsClient(), testImages, fingerprint.TestImageTypeCorruptLastByte); err != nil {
		s.Fatal("Corrupt last byte test failed: ", err)
	}

	if err := testFlashingGoodFirmwareVersion(ctx, d, t.DutfsClient(), testImages); err != nil {
		s.Fatal("Good firmware test failed: ", err)
	}
}
