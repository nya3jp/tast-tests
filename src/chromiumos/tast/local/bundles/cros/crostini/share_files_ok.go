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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShareFilesOK,
		Desc:     "Test sharing My files with Crostini and clicking OK on the confirm dialog",
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
func ShareFilesOK(ctx context.Context, s *testing.State) {
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

	if err := shareMyFilesOK(ctx, sharedFolders, filesApp, tconn); err != nil {
		s.Fatal("Failed to share My files: ", err)
	}

	// Check shared folders on the Settings app.
	st, err := settings.OpenLinuxSettings(ctx, tconn, settings.ManageSharedFolders)
	if err != nil {
		s.Fatal("Failed to open Manage shared folders: ", err)
	}
	defer st.Close(ctx)

	sharedFoldersList, err := st.GetSharedFolders(ctx)
	if err != nil {
		s.Fatal("Failed to find the shared folders list: ", err)
	}
	if err := listset.CheckListsMatch(sharedFoldersList, sharedfolders.MyFiles); err != nil {
		s.Fatal("Failed to verify shared folders list: ", err)
	}
}

func shareMyFilesOK(ctx context.Context, sharedFolders *sharedfolders.SharedFolders, filesApp *filesapp.FilesApp, tconn *chrome.TestConn) error {
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

	return nil
}
