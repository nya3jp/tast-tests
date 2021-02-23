// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/rpc"
	fmpb "chromiumos/tast/services/cros/filemanager"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		// TODO(b/177494589): Add additional test cases for different FUSE instances.
		Func: FreezeFUSE,
		Desc: "Verify that freeze on suspend works with FUSE",
		Contacts: []string{
			"dbasehore@google.com",
			"cros-telemetry@google.com",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
		},
		Attr: []string{
			"group:mainline",
			"informational",
		},
		Data:    []string{"100000_files_in_one_folder.zip"},
		Timeout: 15 * time.Minute,
		Vars: []string{
			"filemanager.user",
			"filemanager.password",
		},
		ServiceDeps: []string{"tast.cros.filemanager.FreezeFUSEService"},
	})
}

func FreezeFUSE(ctx context.Context, s *testing.State) {
	d := s.DUT()

	// Connect to the gPRC service
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)
	fc := fmpb.NewFreezeFUSEServiceClient(cl.Conn)

	tempdir, err := d.Conn().Command("mktemp", "-d", "/tmp/nearby_share_XXXXXX").Output(ctx)
	if err != nil {
		s.Fatal("Failed to create remote data path directory: ", err)
	}
	dataPath := strings.TrimSpace(string(tempdir))
	defer d.Conn().Command("rm", "-r", dataPath).Run(ctx)

	zipFile := "100000_files_in_one_folder.zip"

	remoteZipPath := filepath.Join(dataPath, zipFile)

	if _, err := linuxssh.PutFiles(ctx, d.Conn(), map[string]string{
		s.DataPath(zipFile): remoteZipPath,
	}, linuxssh.DereferenceSymlinks); err != nil {
		s.Fatalf("Failed to send data to remote data path %v: %v", dataPath, err)
	}

	// Attempt to suspend/resume 5 times while mounting a zip file.
	// Without the freeze ordering patches, suspend is more likely to fail than
	// not, so attempt 5 times to balance reproducing the bug with test runtime
	// (about 1 minute 15 seconds per attempt).
	const suspendAttempts = 5
	for i := 0; i < suspendAttempts; i++ {
		if _, err := fc.TestMountZipAndSuspend(ctx, &fmpb.TestMountZipAndSuspendRequest{
			User:        s.RequiredVar("filemanager.user"),
			Password:    s.RequiredVar("filemanager.password"),
			ZipDataPath: remoteZipPath,
		}); err != nil {
			s.Error("Failed to TestMountZipAndSuspend: ", err)
		}
	}

	if err := s.DUT().Reboot(ctx); err != nil {
		s.Error("Failed to reboot: ", err)
	}
}
