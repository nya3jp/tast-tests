// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/ui/settingsapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/linuxfiles"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShareMyfilesAlert,
		Desc:     "Test sharing My files to Crostini and clicking Manage on alert container",
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
func ShareMyfilesAlert(ctx context.Context, s *testing.State) {
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

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Share My files and click Manage in the alert container.
	if err := linuxfiles.ShareMyfiles(ctx, tconn, filesApp, linuxfiles.MyfilesMsg, true, true); err != nil {
		s.Fatal("Failed to share My files: ", err)
	}

	// Check shared folders on the Settings page launched by clicking Manage.
	if err := linuxfiles.CheckSharedFoldersInSettings(ctx, tconn, false, linuxfiles.Myfiles); err != nil {
		s.Fatal("Failed to verify shared folders list after clicking Manage on the alert container: ", err)
	}

	// Check the file list in the containers.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := cont.CheckFileListEqual(ctx, linuxfiles.MountPath, "fonts", linuxfiles.MountFolderMyfiles); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to verify file list in container: ", err)
	}

	// Unshare folders.
	if err := settingsapp.UnshareFoldersFromLinux(ctx, tconn, linuxfiles.Myfiles); err != nil {
		s.Fatal("Failed to delete shared folder My files: ", err)
	}

	// Check the file list in the containers.
	// TODO(crbug/1112190): the following code should be uncommented and tested once this bug fixed.
	// if err := testing.Poll(ctx, func(ctx context.Context) error {
	// 	if err := cont.CheckFileListEqual(ctx, linuxfiles.MountPath, "fonts", linuxfiles.MountFolderMyfiles); err != nil {
	// 		return err
	// 	}
	// 	return nil
	// }, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
	// 	s.Fatal("Failed to verify file list in container: ", err)
	// }
}
