// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
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
	})
}

func FreezeFUSE(ctx context.Context, s *testing.State) {
	// Attempt to suspend/resume 5 times while mounting a zip file.
	// Without the freeze ordering patches, suspend is more likely to fail than
	// not, so attempt 5 times to balance reproducing the bug with test runtime
	// (about 1 minute 15 seconds per attempt).
	const suspendAttempts = 5
	for i := 0; i < suspendAttempts; i++ {
		if !s.Run(ctx, fmt.Sprintf("FUSE Suspend Attempt %d", i), testMountZipAndSuspend) {
			return
		}
	}
}

func testMountZipAndSuspend(ctx context.Context, s *testing.State) {
	// Use a shortened context to allow time for required cleanup steps.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	// Create a new Chrome instance since |tconn| doesn't survive suspend/resume.
	// TODO(crbug.com/1168360): Don't restart Chrome after tconn survives suspend/resume.
	cr, err := chrome.New(
		ctx,
		chrome.GAIALogin(),
		chrome.Auth(s.RequiredVar("filemanager.user"), s.RequiredVar("filemanager.password"), "gaia-id"),
		chrome.ARCDisabled(),
		chrome.EnableFeatures("FilesZipMount"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create TestAPIConn for Chrome: ", err)
	}

	// This command quickly reproduces freeze timeouts with archives.
	// The PID is assigned to the ui cgroup here to avoid race conditions where
	// find/cat are forked before writing the PID to cgroup.procs.
	// Sync is run before the while loop to speed up the kernel's sync before
	// the stress script starts hammering the filesystem.
	script := "echo $$ > /sys/fs/cgroup/freezer/ui/cgroup.procs;" +
		"sync;" +
		"while true; do find /media/archive -type f | xargs cat &> /dev/null; done"

	cmd := testexec.CommandContext(
		ctx,
		"sh",
		"-c",
		script)

	// Copy the zip file to Downloads folder.
	zipFile := "100000_files_in_one_folder.zip"
	zipPath := path.Join(filesapp.DownloadPath, zipFile)
	if err := fsutil.CopyFile(s.DataPath(zipFile), zipPath); err != nil {
		s.Fatalf("Error copying ZIP file to %s: %s", zipPath, err)
	}
	defer func() {
		if err := os.Remove(zipPath); err != nil {
			s.Errorf("Failed to remove ZIP file %s: %v", zipPath, err)
		}
		if err := cmd.Kill(); err != nil {
			s.Error("Failed to kill testing script: ", err)
		}
		cmd.Wait()
		// Restart powerd, otherwise we may get stuck in suspend.
		if err := testexec.CommandContext(cleanupCtx, "restart", "powerd").Run(); err != nil {
			s.Error("Failed to restart powerd after failed suspend attempt. DUT may get stuck after retry suspend: ", err)
		}
	}()

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Could not launch the Files App: ", err)
	}
	defer files.Close(cleanupCtx)

	if err := files.OpenDownloads(ctx); err != nil {
		s.Fatal("Could not open Downloads folder: ", err)
	}

	// Wait for the zip file to show up in the UI.
	if err := files.WaitForFile(ctx, zipFile, 3*time.Minute); err != nil {
		s.Fatal("Waiting for test ZIP file failed: ", err)
	}

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

	if err := cmd.Start(); err != nil {
		s.Fatal("Unable to start archive stress script: ", err)
	}

	// Read wakeup count here to prevent suspend retries, which happen without user input.
	wakeupCount, err := ioutil.ReadFile("/sys/power/wakeup_count")
	if err != nil {
		s.Fatal("Failed to read wakeup count before suspend: ", err)
	}

	// Suspend for 45 seconds since the stress script slows us down.
	// This gives freeze during suspend enough time to timeout in 20s.
	s.Log("Attempting suspend")
	if err := testexec.CommandContext(
		ctx,
		"powerd_dbus_suspend",
		fmt.Sprintf("--wakeup_count=%s", strings.Trim(string(wakeupCount), "\n")),
		"--timeout=30",
		"--suspend_for_sec=45").Run(); err != nil {
		s.Fatal("powerd_dbus_suspend failed to properly suspend: ", err)
	}
}
