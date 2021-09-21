// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/bundles/cros/firmware/fwupd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FwupdInstallRemote,
		Desc: "Checks that fwupd can install using a remote repository",
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
	uri, err := fwupd.ReleaseURI(ctx)
	if err != nil {
		s.Fatal("Failed to get release URI: ", err)
	}

	cmd := testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", "install", "--allow-reinstall", "-v", uri)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("%q failed: %v", cmd.Args, err)
	}
}
