// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/common/testexec"
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

// FwupdGetDevices runs the fwupdtool utility and verifies that it
// detects devices in the system.
func FwupdGetDevices(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx, "/usr/bin/fwupdtool", "get-devices", "--show-all")

	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
}
