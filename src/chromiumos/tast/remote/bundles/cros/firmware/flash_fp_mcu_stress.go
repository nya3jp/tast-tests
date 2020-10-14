// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FlashFpMcuStress,
		Desc: "Repeatedly run flash_fp_mcu, rebooting each time after flashing",
		Contacts: []string{
			"yichengli@chromium.org", // Test author
			"tomhughes@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
	})
}

func flashFpMcu(ctx context.Context, d *dut.DUT, fpFirmwarePath string) error {
	flashCmd := []string{"flash_fp_mcu", fpFirmwarePath}
	testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(flashCmd))
	if err := d.Command(flashCmd[0], flashCmd[1:]...).Run(ctx); err != nil {
		d.Reboot(ctx)
		return errors.Wrap(err, "flash_fp_mcu failed")
	}
	testing.ContextLog(ctx, "flash_fp_mcu completed, now rebooting to rebind cros-ec-uart on zork")
	d.Reboot(ctx)
	return nil
}

func FlashFpMcuStress(ctx context.Context, s *testing.State) {
	d := s.DUT()
	fpBoard, err := getFpBoard(ctx, d)
	if err != nil {
		s.Fatalf("Failed to get fp board: %v", err)
	}
	testing.ContextLogf(ctx, "fp board name: %q", fpBoard)

	fpFirmwarePath, err := getFpFirmwarePath(ctx, d, fpBoard)
	if err != nil {
		s.Fatalf("Failed to get fp firmware path: %v", err)
	}

	for i := 0; i < 10; i++ {
		if err := flashFpMcu(ctx, d, fpFirmwarePath); err != nil {
			s.Fatalf("Failed to flash FP firmware: %v", err)
		}

		// Check version and see if the FPMCU is in a good state.
		versionCmd := []string{"ectool", "--name=cros_fp", "version"}
		testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(versionCmd))
		err := d.Command(versionCmd[0], versionCmd[1:]...).Run(ctx)
		if err != nil {
			s.Fatalf("Failed to query FPMCU version after flashing (error: %v).", err)
		}
	}
}
