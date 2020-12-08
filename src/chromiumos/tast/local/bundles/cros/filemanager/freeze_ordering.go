// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	//"fmt"
	"os"
	"path"
	"time"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/filemanager/pre"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const zipFile = "100000_files_in_one_folder.zip"

func init() {
	testing.AddTest(&testing.Test{
		Func: FreezeOrdering,
		Desc: "Verify that freeze on suspend works with the current ordering",
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
		Data: []string{zipFile},
		Timeout: 10 * time.Minute,
		Pre:     pre.DriveFsStarted,
		Vars: []string{
			"filemanager.user",
			"filemanager.password",
			"filemanager.drive_credentials",
		},
	})
}

func FreezeOrdering(ctx context.Context, s *testing.State) {
	// Copy the zip file to DriveFS
	drivefsPath := path.Join(s.PreValue().(drivefs.PreData).MountPath, "root")
	zipPath := path.Join(drivefsPath, zipFile)
	if err := fsutil.CopyFile(s.DataPath(zipFile), zipPath); err != nil {
		s.Fatalf("Error copying ZIP file to %s: %s", zipPath, err)
	}
	defer os.Remove(zipPath)

	// Attempt to suspend/resume 15 times while mounting a zip file from Drive.
	const suspendAttempts = 15
	for i := 0; i < suspendAttempts; i++ {
		testMountZipAndSuspend(ctx, s)
	}
}

func testMountZipAndSuspend(ctx context.Context, s *testing.State,) {
	tconn := s.PreValue().(drivefs.PreData).TestAPIConn
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Could not launch the Files App: ", err)
	}
	defer files.Close(ctx)

	if err := files.OpenDrive(ctx); err != nil {
		s.Fatal("Could not open Google Drive folder: ", err)
	}

	// Wait for the zip file to show up in the UI
	if err := files.WaitForFile(ctx, zipFile, 3*time.Minute); err != nil {
		s.Fatal("Waiting for test zip file failed: ", err)
	}

	if err := files.OpenFile(ctx, zipFile); err != nil {
		s.Fatal("Mounting zip file failed", err)
	}

	s.Log("Attempting suspend")
	if err := testexec.CommandContext(ctx, "powerd_dbus_suspend", "--timeout=10", "--wakeup_timeout=15").Run(); err != nil {
		s.Fatal("powerd_dbus_suspend failed to properly suspend: ", err)
	}

	params := ui.FindParams{
		Name: "Files - " + zipFile,
		Role: ui.RoleTypeRootWebArea,
	}

	if err := files.Root.WaitUntilDescendantExists(ctx, params, 3*time.Minute); err != nil {
		// This can happen if DriveFS is unmounted on suspend
		s.Logf("Mounting zip file, %s, failed: ", zipFile, err)
		return
	}

	params = ui.FindParams{
		Name: zipFile,
		Role: ui.RoleTypeTreeItem,
	}

	treeItem, err := files.Root.DescendantWithTimeout(ctx, params, 5*time.Second)
	if err != nil {
		s.Fatalf("Cannot find tree item for %s: %v", zipFile, err)
	}
	defer treeItem.Release(ctx)

	params = ui.FindParams{
		Name: "Eject device",
		Role: ui.RoleTypeButton,
	}

	ejectButton, err := treeItem.DescendantWithTimeout(ctx, params, 5*time.Second)
	if err != nil {
		s.Fatal("Cannot find eject button: ", err)
		return
	}
	defer ejectButton.Release(ctx)

	if err := ejectButton.LeftClick(ctx); err != nil {
		s.Fatal("Cannot click eject button: ", err)
		return
	}

	params = ui.FindParams{
		Name: zipFile,
		Role: ui.RoleTypeTreeItem,
	}

	if err = files.Root.WaitUntilDescendantGone(ctx, params, 5*time.Second); err != nil {
		s.Errorf("%s is still mounted: %v", zipFile, err)
	}
}
