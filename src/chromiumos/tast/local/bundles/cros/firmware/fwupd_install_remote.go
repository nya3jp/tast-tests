// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"path/filepath"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/shutil"
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
		Data:         []string{"fwupd-tests.conf", "fwupd-tests.xml"},
	})
}

// FwupdInstallRemote runs the fwupdtool utility and verifies that it
// can update a device in the system using a remote repository.
func FwupdInstallRemote(ctx context.Context, s *testing.State) {
	const (
		configFile   = "/etc/fwupd/daemon.conf"
		remoteDir    = "/etc/fwupd/remotes.d/"
		remoteFile   = "fwupd-tests.conf"
		manifestDir  = "/usr/share/fwupd/remotes.d/vendor/"
		manifestFile = "fwupd-tests.xml"
	)

	// Enable test plugins
	cmd := testexec.CommandContext(ctx, "/bin/sed", "-e", "/^DisabledPlugins=/s/^/#/", "-i", configFile)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
	if err := fsutil.CopyFile(s.DataPath(remoteFile), filepath.Join(remoteDir, remoteFile)); err != nil {
		s.Fatalf("Failed to copy %s: %v", remoteFile, err)
	}
	if err := fsutil.CopyFile(s.DataPath(manifestFile), filepath.Join(manifestDir, manifestFile)); err != nil {
		s.Fatalf("Failed to copy %s: %v", manifestFile, err)
	}

	cmd = testexec.CommandContext(ctx, "/usr/bin/fwupdtool", "update", "-v", "b585990a-003e-5270-89d5-3705a17f9a43")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
}
