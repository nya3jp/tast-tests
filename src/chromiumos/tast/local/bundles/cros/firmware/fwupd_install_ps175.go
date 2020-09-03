// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FwupdInstallPs175,
		Desc: "Checks that fwupd can detect device and update the firmware",
		Contacts: []string{
			"sshiyu@chromium.org",          // Test Author
			"chromeos-firmware@google.com", // CrOS Firmware Developers
		},
		Attr:         []string{"group:fwupd"},
		SoftwareDeps: []string{"fwupd"},
		Data:         []string{"fwupd_install_dummy_ps175_V99.99.cab", "flashrom"},
	})
}

// FwupdInstallPs175 runs the fwupdtool utility update and verifies that the PS175
// device are recoginzed and get updated as expected.
func FwupdInstallPs175(ctx context.Context, s *testing.State) {
	const deviceID = "ef43eb9fd629d16aa4a1b86c30b9752a995f2a54"

	f, err := os.Create(filepath.Join(s.OutDir(), "fwupdtool-lspcon.txt"))
	if err != nil {
		s.Error("Failed to create fwupdtool output: ", err)
	}

	defer f.Close()

	cmd := testexec.CommandContext(ctx, "/usr/bin/fwupdtool", "install", s.DataPath("fwupd_install_dummy_ps175_V99.99.cab"), deviceID)
	cmd.Stdout = f
	cmd.Env = []string{fmt.Sprintf("PATH=%s", filepath.Dir(s.DataPath("flashrom")))}
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
}
