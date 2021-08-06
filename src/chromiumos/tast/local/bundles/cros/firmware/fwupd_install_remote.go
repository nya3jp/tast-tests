// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FwupdInstallRemote,
		Desc: "Checks that fwcmd can install using a remote repository",
		Contacts: []string{
			"campello@chromium.org",     // Test Author
			"chromeos-fwupd@google.com", // CrOS FWUPD
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"fwupd"},
	})
}

// FwupdInstallRemote runs the fwupdtool utility and verifies that it
// can update a device in the system using a remote repository.
func FwupdInstallRemote(ctx context.Context, s *testing.State) {
	// b585990a-003e-5270-89d5-3705a17f9a43 is the GUID for a fake device.
	updtype := updateChecker(ctx)

	cmd := testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", updtype, "-v", "b585990a-003e-5270-89d5-3705a17f9a43", "--ignore-power")
	err := cmd.Run(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("%s failed: %v", cmd.Args, err)
	}
}
