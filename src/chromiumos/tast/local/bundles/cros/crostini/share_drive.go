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
		Func:     ShareDrive,
		Desc:     "Test sharing Google Drive with Crostini",
		Contacts: []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"crostini.gaiaUsername", "crostini.gaiaPassword", "crostini.gaiaID", "keepState"},
		Params: []testing.Param{{
			Name:              "artifact_gaia",
			Pre:               crostini.StartedByArtifactWithGaiaLogin(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniStable,
		}, {
			Name:    "download_buster_gaia",
			Pre:     crostini.StartedByDownloadBusterWithGaiaLogin(),
			Timeout: 10 * time.Minute,
		}},
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}
func ShareDrive(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()
	defer crostini.RunCrostiniPostTest(ctx, cont)

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

	if err := shareDriveOK(ctx, sharedFolders, filesApp, tconn); err != nil {
		s.Fatal("Failed to share Google Drive: ", err)
	}

	if err := checkDriveResults(ctx, tconn, cont); err != nil {
		s.Fatal("Failed to verify sharing results: ", err)
	}
}

func shareDriveOK(ctx context.Context, sharedFolders *sharedfolders.SharedFolders, filesApp *filesapp.FilesApp, tconn *chrome.TestConn) error {
	// Share Google Drive.
	scd, err := sharedFolders.ShareDrive(ctx, tconn, filesApp)
	if err != nil {
		return errors.Wrap(err, "failed to share Google Drive")
	}
	defer scd.Release(ctx)

	// Click button OK.
	toast, err := scd.ClickOK(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to click button OK on share confirm dialog")
	}
	defer toast.Release(ctx)

	sharedFolders.AddFolder(sharedfolders.SharedDrive)

	return nil
}

func checkDriveResults(ctx context.Context, tconn *chrome.TestConn, cont *vm.Container) error {
	// Check the shared folders on Settings.
	s, err := settings.OpenLinuxSettings(ctx, tconn, settings.ManageSharedFolders)
	if err != nil {
		return errors.Wrap(err, "failed to find Manage shared folders")
	}
	defer s.Close(ctx)

	sharedFoldersList, err := s.GetSharedFolders(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find the shared folders list")
	}
	if err := listset.CheckListsMatch(sharedFoldersList, sharedfolders.SharedDrive); err != nil {
		return errors.Wrap(err, "failed to verify shared folders list")
	}

	// Check the file list in the container.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		list, err := cont.GetFileList(ctx, sharedfolders.MountPath)
		if err != nil {
			return err
		}
		if err := listset.CheckListsMatch(list, "fonts", sharedfolders.MountFolderGoogleDrive); err != nil {
			return err
		}
		list, err = cont.GetFileList(ctx, sharedfolders.MountPathGoogleDrive)
		if err != nil {
			return err
		}
		if err := listset.CheckListsMatch(list, sharedfolders.MountFolderMyDrive); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to verify file list in container")
	}

	return nil
}
