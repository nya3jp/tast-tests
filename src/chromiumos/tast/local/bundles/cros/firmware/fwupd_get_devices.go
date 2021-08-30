// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FwupdGetDevices,
		Desc: "Checks that fwupd can detect device",
		Contacts: []string{
			"campello@chromium.org",     // Test Author
			"chromeos-fwupd@google.com", // CrOS FWUPD
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"fwupd"},
	})
}

// FwupdGetDevices runs the fwupdmgr utility and verifies that it
// detects devices in the system.
func FwupdGetDevices(ctx context.Context, s *testing.State) {
	if err := upstart.EnsureJobRunning(ctx, "fwupd"); err != nil {
		s.Error("Failed to ensure fwupd is running: ", err)
	}

	cmd := testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", "get-devices", "--show-all")

	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "fwupdmgr.txt"), output, 0644); err != nil {
		s.Error("Failed dumping fwupdmgr output: ", err)
	}
}
