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

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	fmpb "chromiumos/tast/services/cros/filemanager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			fmpb.RegisterFreezeFUSEServiceServer(srv, &FreezeFUSEService{s})
		},
	})
}

type FreezeFUSEService struct {
	s *testing.ServiceState
}

func (f *FreezeFUSEService) TestMountZipAndSuspend(ctx context.Context, request *fmpb.TestMountZipAndSuspendRequest) (emp *empty.Empty, lastErr error) {
	// Use a shortened context to allow time for required cleanup steps.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	// Create a new Chrome instance since |tconn| doesn't survive suspend/resume.
	// TODO(crbug.com/1168360): Don't restart Chrome after tconn survives suspend/resume.
	cr, err := chrome.New(
		ctx,
		chrome.GAIALogin(chrome.Creds{User: request.GetUser(), Pass: request.GetPassword()}),
		chrome.ARCDisabled())
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create TestAPIConn for Chrome")
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
	if err := fsutil.CopyFile(request.GetZipDataPath(), zipPath); err != nil {
		return nil, errors.Wrapf(err, "error copying ZIP file to %q", zipPath)
	}
	defer func() {
		if err := os.Remove(zipPath); err != nil {
			lastErr = errors.Wrapf(err, "failed to remove ZIP file %q", zipPath)
			// Log the error now, because this may not the last error.
			testing.ContextLog(cleanupCtx, lastErr)
		}
		if err := cmd.Kill(); err != nil {
			lastErr = errors.Wrap(err, "failed to kill testing script")
			// Log the error now, because this may not the last error.
			testing.ContextLog(cleanupCtx, lastErr)
		}
		cmd.Wait()
		// Restart powerd, otherwise we may get stuck in suspend.
		if err := testexec.CommandContext(cleanupCtx, "restart", "powerd").Run(); err != nil {
			lastErr = errors.Wrap(err, "failed to restart powerd after failed suspend attempt. DUT may get stuck after retry suspend")
		}
	}()

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "could not launch the Files App")
	}
	defer files.Close(cleanupCtx)

	if err := files.OpenDownloads(ctx); err != nil {
		return nil, errors.Wrap(err, "could not open Downloads folder")
	}

	// Wait for the zip file to show up in the UI.
	if err := files.WaitForFile(ctx, zipFile, 3*time.Minute); err != nil {
		return nil, errors.Wrap(err, "Waiting for test ZIP file failed")
	}

	if err := files.OpenFile(ctx, zipFile); err != nil {
		return nil, errors.Wrap(err, "Opening ZIP file failed")
	}

	params := ui.FindParams{
		Name: "Files - " + zipFile,
		Role: ui.RoleTypeRootWebArea,
	}

	if err := files.Root.WaitUntilDescendantExists(ctx, params, time.Minute); err != nil {
		return nil, errors.Wrapf(err, "Mounting ZIP file %q failed", zipFile)
	}

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "Unable to start archive stress script")
	}

	// Read wakeup count here to prevent suspend retries, which happen without user input.
	wakeupCount, err := ioutil.ReadFile("/sys/power/wakeup_count")
	if err != nil {
		return nil, errors.Wrap(err, "failed to read wakeup count before suspend")
	}

	// Suspend for 45 seconds since the stress script slows us down.
	// This gives freeze during suspend enough time to timeout in 20s.
	testing.ContextLog(ctx, "Attempting suspend")
	if err := testexec.CommandContext(
		ctx,
		"powerd_dbus_suspend",
		fmt.Sprintf("--wakeup_count=%s", strings.Trim(string(wakeupCount), "\n")),
		"--timeout=30",
		"--suspend_for_sec=45").Run(); err != nil {
		return nil, errors.Wrap(err, "powerd_dbus_suspend failed to properly suspend")
	}
	return &empty.Empty{}, lastErr
}
