// Copyright 2021 The ChromiumOS Authors
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
	deviceInfoPattern = `.*PS175:
.*Device ID:          [0-9a-f]+
.*Current version:    \d+\.\d+
.*Vendor:             Parade Technologies \(PCI:0x1AF8, OUI:001CF8\)
.*GUID:               c146ccc9-58b6-517c-97f6-9c55a0bd39d3 \? I2C\\NAME_1AF80175:00
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
			hwdep.DisplayPortConverter("PS175"),
		),
	})
}

// verifyPS175Detected verifies the fwupdmgr output shows a PS175 was detected.
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

// FwupdDetectPS175 runs the fwupdmgr utility and verifies that the PS175
// device is recognized.
func FwupdDetectPS175(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", "get-devices")

	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}

	if err := verifyPS175Detected(output); err != nil {
		s.Fatal("fwupdmgr failed to detect PS175: ", err)
	}
}
