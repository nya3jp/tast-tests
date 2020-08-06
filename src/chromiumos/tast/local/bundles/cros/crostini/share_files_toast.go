// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/crostini/listset"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/local/crostini/ui/sharedfolders"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShareFilesToast,
		Desc:     "Test sharing My files with Crostini and clicking Manage on toast nofication",
		Contacts: []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:              "artifact",
			Pre:               crostini.StartedByArtifact(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniStable,
		}, {
			Name:              "artifact_unstable",
			Pre:               crostini.StartedByArtifact(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniUnstable,
		}, {
			Name:    "download_stretch",
			Pre:     crostini.StartedByDownloadStretch(),
			Timeout: 10 * time.Minute,
		}, {
			Name:    "download_buster",
			Pre:     crostini.StartedByDownloadBuster(),
			Timeout: 10 * time.Minute,
		}},
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}
func ShareFilesToast(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	// Open Files app.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}
	defer filesApp.Close(cleanupCtx)

	sharedFolders := sharedfolders.NewSharedFolders()
	// Clean up shared folders in the end.
	defer func() {
		if err := sharedFolders.UnshareAll(cleanupCtx, tconn, cont); err != nil {
			s.Error("Failed to unshare all folders: ", err)
		}
	}()

	if err := shareMyFilesOKManage(ctx, sharedFolders, filesApp, tconn); err != nil {
		s.Fatal("Failed to share My files: ", err)
	}

	if err := checkShareResults(ctx, tconn, cont); err != nil {
		s.Fatal("Failed to check result after sharing My files: ", err)
	}

	// Unshare My files. This is part of the test, different from clean up in line 72.
	if err := unshareMyFiles(ctx, tconn, cont, sharedFolders); err != nil {
		s.Fatal("Failed to unshare My files: ", err)
	}
}

func shareMyFilesOKManage(ctx context.Context, sharedFolders *sharedfolders.SharedFolders, filesApp *filesapp.FilesApp, tconn *chrome.TestConn) error {
	// Share My files, click OK on the confirm dialog, click Manage on the toast nofication.
	scd, err := sharedFolders.ShareMyFiles(ctx, tconn, filesApp, sharedfolders.MyFilesMsg)
	if err != nil {
		return errors.Wrap(err, "failed to share My files")
	}
	defer scd.Release(ctx)

	// Click button OK.
	toast, err := scd.ClickOK(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to click button OK on share confirm dialog")
	}
	defer toast.Release(ctx)

	sharedFolders.AddFolder(sharedfolders.MyFiles)

	// Click button Manage.
	if err := toast.ClickManage(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to click button Manage on toast notification")
	}

	return nil
}

func checkShareResults(ctx context.Context, tconn *chrome.TestConn, cont *vm.Container) error {
	// Check the shared folders on Settings.
	sharedFoldersList, err := settings.GetLinuxSharedFolders(ctx, tconn, settings.PageNameMSF)
	if err != nil {
		return errors.Wrap(err, "failed to find the shared folders list")
	}
	if err := listset.CheckListsMatch(sharedFoldersList, sharedfolders.MyFiles); err != nil {
		return errors.Wrap(err, "failed to verify shared folders list")
	}

	// Check the file list in the container.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		list, err := cont.GetFileList(ctx, sharedfolders.MountPath)
		if err != nil {
			return err
		}
		if err := listset.CheckListsMatch(list, "fonts", sharedfolders.MountFolderMyFiles); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to verify file list in container")
	}

	return nil
}

func unshareMyFiles(ctx context.Context, tconn *chrome.TestConn, cont *vm.Container, sharedFolders *sharedfolders.SharedFolders) error {
	if err := sharedFolders.Unshare(ctx, tconn, sharedfolders.MyFiles); err != nil {
		return errors.Wrap(err, "failed to delete shared folder My files")
	}

	if err := sharedFolders.CheckNoSharedFolders(ctx, tconn, cont); err != nil {
		return errors.Wrap(err, "failed to verify shared folder list after unshare My files")
	}

	return nil
}
