// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Fwupd,
		Desc: "Checks that fwupd can detect device and update the firmware",
		Contacts: []string{
			"sshiyu@chromium.org",       // Test Author
			"chromeos-firmware@google.com", // CrOS Firmware Developers
		},
		Attr: []string{"informational"},
		SoftwareDeps: []string{"fwupd", "flashrom"},
	})
}

// FwupdInstall runs the fwupdtool utility update and verifies the device are recoginzed
// and get updated as expected.
func FwupdInstallPs175(ctx context.Context, s *testing.State) {
	// TODO(sshiyu): Add a script called flashrom so that the flash won't happen.
	fw_file_dir := "somewhere"
	device_id := "ef43eb9fd629d16aa4a1b86c30b9752a995f2a54"
	
	f, err := os.Create(filepath.Join(s.OutDir(), "fwupdtool-lspcon.txt"))
	if err!= nil {
		s.Error("Failed to create fwupdtool output: ", err)
	}

	defer f.Close()

	cmd := testexec.CommandContext(ctx, "flashrom_tester", "/usr/bin/fwupdtool", "install", fw_file_dir, device_id)
	cmd.Stdout = f
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
}
