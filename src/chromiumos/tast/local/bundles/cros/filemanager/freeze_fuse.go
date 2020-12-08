// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"time"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/testexec"
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
			"drivefs",
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
			"filemanager.drive_credentials",
		},
	})
}

func FreezeFUSE(ctx context.Context, s *testing.State) {
	// Attempt to suspend/resume 5 times while mounting a zip file from Drive.
	const suspendAttempts = 5
	for i := 0; i < suspendAttempts; i++ {
		testMountZipAndSuspend(ctx, s)
	}
}

func testMountZipAndSuspend(ctx context.Context, s *testing.State) {
	// Create a new Chrome instance since |tconn| doesn't survice suspend/resume.
	cr, err := chrome.New(
		ctx,
		chrome.GAIALogin(),
		chrome.Auth(s.RequiredVar("filemanager.user"), s.RequiredVar("filemanager.password"), "gaia-id"),
		chrome.ARCDisabled())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create TestAPIConn for Chrome: ", err)
	}

	// Copy the zip file to DriveFS.
	zipFile := "100000_files_in_one_folder.zip"
	zipPath := path.Join(filesapp.DownloadPath, zipFile)
	if err := fsutil.CopyFile(s.DataPath(zipFile), zipPath); err != nil {
		s.Fatalf("Error copying ZIP file to %s: %s", zipPath, err)
	}
	defer os.Remove(zipPath)

	s.Log("Zip file copied to: ", zipPath)

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Could not launch the Files App: ", err)
	}
	defer files.Close(ctx)

	if err := files.OpenDownloads(ctx); err != nil {
		s.Fatal("Could not open Downloads folder: ", err)
	}

	// Wait for the zip file to show up in the UI.
	if err := files.WaitForFile(ctx, zipFile, 3*time.Minute); err != nil {
		s.Fatal("Waiting for test ZIP file failed: ", err)
	}

	s.Log("Starting zip archive mount")
	if err := files.OpenFile(ctx, zipFile); err != nil {
		s.Fatal("Mounting zip file failed: ", err)
	}

	params := ui.FindParams{
		Name: "Files - " + zipFile,
		Role: ui.RoleTypeRootWebArea,
	}

	if err := files.Root.WaitUntilDescendantExists(ctx, params, time.Minute); err != nil {
		s.Fatalf("Mounting zip file, %s, failed: %v", zipFile, err)
	}
	s.Log("Zip archive mount complete")

	// This command quickly reproduces freeze timeouts with archives.
	cmd := testexec.CommandContext(
		ctx,
		"sh",
		"-c",
		"while true; do find /media/archive -type f | xargs cat &> /dev/null; done")
	if err := cmd.Start(); err != nil {
		s.Fatal("Unable to start archive stress script: ", err)
	}
	defer cmd.Kill()

	// Put the script in the ui (Chrome) cgroup to simulate the Files App accessing the ZIP archive.
	if err := ioutil.WriteFile(
		"/sys/fs/cgroup/freezer/ui/cgroup.procs",
		[]byte(strconv.Itoa(cmd.Process.Pid)),
		0644); err != nil {
		s.Log("Freezer cgroup ui does not exist: ", err)
	}

	s.Log("Attempting suspend")
	if err := testexec.CommandContext(ctx, "powerd_dbus_suspend", "--timeout=10", "--wakeup_timeout=30").Run(); err != nil {
		// Restart powerd, otherwise we may get stuck in suspend.
		if err := testexec.CommandContext(ctx, "restart", "powerd").Run(); err != nil {
			s.Fatal("Failed to restart powerd after failed suspend attempt. DUT may get stuck after retry suspend: ", err)
		}
		s.Fatal("powerd_dbus_suspend failed to properly suspend: ", err)
	}
}
