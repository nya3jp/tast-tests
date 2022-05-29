// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/fingerprint"
	"chromiumos/tast/remote/firmware/fingerprint/rpcdut"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpRWNoUpdateRO,
		Desc: "Enables hardware write protect, attempts to flash the RO fingerprint firmware, and verifies that the flashing fails",
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

func FpRWNoUpdateRO(ctx context.Context, s *testing.State) {
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

	s.Log("Flashing RO firmware (expected to fail)")
	flashCmd := []string{
		"flashrom",
		"--noverify-all",   // only verify included regions
		"-V",               // verbose
		"-p", "ec:type=fp", // use "programmer" for fingerprint "EC"
		"-i", "EC_RO", // target image is RO
		"-w", testImages[fingerprint.TestImageTypeDev].Path, // write specified file
	}
	s.Log("Running command: ", shutil.EscapeSlice(flashCmd))
	if output, err := d.Conn().CommandContext(ctx, flashCmd[0], flashCmd[1:]...).CombinedOutput(); err == nil {
		s.Fatal("Flashing RO firmware should not succeed, cmd output: ", output)
	}

	s.Log("Flashing failed as expected")
}
