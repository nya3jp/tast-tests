// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/crostini/util"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/local/crostini/ui/sharedfolders"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShareFilesToast,
		Desc:     "Test sharing My files to Crostini and clicking Manage on toast nofication",
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

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Open Files app.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}
	defer filesApp.Close(cleanupCtx)

	sharedFolders := &sharedfolders.SharedFolders{Folders: make(map[string]struct{})}
	// Clean up shared folders in the end.
	defer sharedFolders.UnshareAll(cleanupCtx, tconn)

	// Share My files and click Manage in the toast nofication.
	if err := sharedFolders.ShareMyfiles(ctx, tconn, filesApp, sharedfolders.MyFilesMsg, true, true); err != nil {
		s.Fatal("Failed to share My files: ", err)
	}

	shareFoldersList, err := settings.GetLinuxSharedFolders(ctx, tconn, settings.SettingsMSF)
	if err != nil {
		s.Fatal("Failed to find the shared folders list: ", err)
	}

	if err := util.CheckTwoListsMatch(shareFoldersList, sharedfolders.MyFiles); err != nil {
		s.Fatal("Failed to verify shared folders list: ", err)
	}

	// Check the file list in the containers.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		list, err := cont.GetFileList(ctx, sharedfolders.MountPath)
		if err != nil {
			return err
		}
		if err := util.CheckTwoListsMatch(list, "fonts", sharedfolders.MountFolderMyFiles); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to verify file list in container: ", err)
	}

	// Unshare folders. This is part of the test, different from clean up in line 59.
	if err := sharedFolders.Unshare(ctx, tconn, sharedfolders.MyFiles); err != nil {
		s.Fatal("Failed to delete shared folder My files: ", err)
	}

	// Check the file list in the containers.
	// TODO(crbug/1112190): the following code should be uncommented and tested once this bug fixed.
	// if err := testing.Poll(ctx, func(ctx context.Context) error {
	// 	if err := util.CheckTwoListsMatch(ctx, sharedfolders.MountPath, "fonts"); err != nil {
	// 		return err
	// 	}
	// 	return nil
	// }, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
	// 	s.Fatal("Failed to verify file list in container: ", err)
	// }
}
