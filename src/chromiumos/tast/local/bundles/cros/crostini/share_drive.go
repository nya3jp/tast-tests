// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"reflect"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
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
	defer crostini.RunCrostiniPostTest(cleanupCtx, cont)

	// Open Files app.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}
	// Close the Files app in the end.
	defer func() {
		if err := filesApp.Close(cleanupCtx); err != nil {
			s.Error("Failed to close the Files app: ", err)
		}
	}()

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
	scd, err := sharedFolders.ShareDrive(ctx, tconn, filesApp, true)
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

	return nil
}

func checkDriveResults(ctx context.Context, tconn *chrome.TestConn, cont *vm.Container) error {
	// Check the shared folders on Settings.
	s, err := settings.OpenLinuxSettings(ctx, tconn, settings.ManageSharedFolders)
	if err != nil {
		return errors.Wrap(err, "failed to find Manage shared folders")
	}
	defer s.Close(ctx)

	if shared, err := s.GetSharedFolders(ctx); err != nil {
		return errors.Wrap(err, "failed to find the shared folders list")
	} else if want := []string{sharedfolders.SharedDrive}; !reflect.DeepEqual(shared, want) {
		return errors.Errorf("failed to verify shared folders list, got %s, want %s", shared, want)
	}

	// Check the file list in the container.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if list, err := cont.GetFileList(ctx, sharedfolders.MountPath); err != nil {
			return err
		} else if want := []string{"fonts", sharedfolders.MountFolderGoogleDrive}; !reflect.DeepEqual(list, want) {
			return errors.Errorf("failed to verify file list in /mnt/chromeos, got %s, want %s", list, want)
		}

		if list, err := cont.GetFileList(ctx, sharedfolders.MountPathGoogleDrive); err != nil {
			return err
		} else if want := []string{sharedfolders.MountFolderMyDrive}; !reflect.DeepEqual(list, want) {
			return errors.Errorf("failed to verify file list in /mnt/chromeos/GoogleDrive, got %s, want %s", list, want)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to verify file list in container")
	}

	return nil
}
