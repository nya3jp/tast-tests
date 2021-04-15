// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	deviceID = "f8887c5d-6236-50cd-bbc4-b188a4a3b198"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FwupdGetDevicesPS175,
		Desc: "Checks that fwupd can detect device",
		Contacts: []string{
			"campello@chromium.org",        // Test Author
			"chromeos-fwupd@google.com",    // CrOS FWUPD
			"chromeos-firmware@google.com", // CrOS Firmware Developers
		},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.Platform("puff")),
	})
}

// verifyDeviceIDLspcon verifys the log file contains proper device ID.
func verifyDeviceIDLspcon(path string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	matched, err := regexp.Match(deviceID, data)
	if err != nil {
		return err
	}

	if !matched {
		return errors.Errorf("failed to find device: %q", data)
	}

	return nil
}

// FwupdGetDevicesPS175 runs the fwupdtool utility update and verifies that the PS175
// device is recognized as expected.
func FwupdGetDevicesPS175(ctx context.Context, s *testing.State) {

	f, err := os.Create(filepath.Join(s.OutDir(), "fwupdtool-lspcon.txt"))
	if err != nil {
		s.Error("Failed to create fwupdtool output: ", err)
	}
	defer f.Close()

	cmd := testexec.CommandContext(ctx, "/usr/bin/fwupdtool", "get-devices", "--plugins=flashrom")

	cmd.Stdout = f
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}

	if err := verifyDeviceIDLspcon(filepath.Join(s.OutDir(), "fwupdtool-lspcon.txt")); err != nil {
		s.Fatal("Failed to verify: ", err)
	}
}
