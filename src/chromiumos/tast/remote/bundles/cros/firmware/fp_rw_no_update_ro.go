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
			fingerprint.GenTestImagesScript,
			fingerprint.Futility,
			fingerprint.BloonchipperDevKey,
			fingerprint.DartmonkeyDevKey,
			fingerprint.NamiFPDevKey,
			fingerprint.NocturneFPDevKey,
		},
	})
}

func FpRWNoUpdateRO(ctx context.Context, s *testing.State) {
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

	testImages, err := fingerprint.GenerateTestFirmwareImages(ctx, d, t.DutfsClient(), s.DataPath(fingerprint.GenTestImagesScript), t.FPBoard(), t.BuildFwFile(), t.DUTTempDir())
	if err != nil {
		s.Fatal("Failed to generate test iamges: ", err)
	}

	testing.ContextLog(ctx, "Flashing RO firmware (expected to fail)")
	flashCmd := []string{"flashrom", "--fast-verify", "-V", "-p", "ec:type=fp", "-i", "EC_RO", "-w", testImages[fingerprint.TestImageTypeDev]}
	testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(flashCmd))
	if output, err := d.Conn().Command(flashCmd[0], flashCmd[1:]...).CombinedOutput(ctx); err == nil {
		s.Fatal("Flashing RO firmware should not succeed, cmd output: ", output)
	}

	testing.ContextLog(ctx, "Flashing failed as expected")
}
