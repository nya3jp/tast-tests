// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	deviceInfoPattern = `PS175:
      Device ID:          [0-9a-f]+
      Current version:    \d+\.\d+
      Vendor:             Parade \(I2C:1AF8\)
      GUID:               f8887c5d-6236-50cd-bbc4-b188a4a3b198 \? FLASHROM-LSPCON-I2C-SPI\\VEN_1AF8&DEV_0175
`
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FwupdDetectPS175,
		Desc: "Checks that fwupd can detect device",
		Contacts: []string{
			"pmarheine@chromium.org",    // Test Author
			"chromeos-fwupd@google.com", // CrOS FWUPD
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"fwupd"},
		HardwareDeps: hwdep.D(
			// TODO(https://crbug.com/1198060): replace with PS175 feature
			hwdep.Platform("puff"),
			// Dooly doesn't have an LSPCON
			hwdep.SkipOnModel("dooly"),
		),
	})
}

// verifyPS175Detected verifies the fwupdtool output shows a PS175 was detected.
func verifyPS175Detected(output []byte) error {
	matched, err := regexp.Match(deviceInfoPattern, output)
	if err != nil {
		return err
	}

	if !matched {
		return errors.Errorf("get-devices output didn't match expected format: %q", output)
	}

	return nil
}

// FwupdDetectPS175 runs the fwupdtool utility and verifies that the PS175
// device is recognized.
func FwupdDetectPS175(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx, "/usr/bin/fwupdtool", "get-devices", "--plugins", "flashrom")

	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}

	if err := verifyPS175Detected(output); err != nil {
		s.Fatal("fwupdtool failed to detect PS175: ", err)
	}
}
