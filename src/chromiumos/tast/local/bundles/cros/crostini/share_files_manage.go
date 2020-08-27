// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"reflect"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/local/crostini/ui/sharedfolders"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShareFilesManage,
		Desc:     "Test sharing My files with Crostini and manage it by selecting contenxt menu 'Manage Linux Sharing'",
		Contacts: []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"keepState"},
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

func ShareFilesManage(ctx context.Context, s *testing.State) {
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
	defer filesApp.Close(cleanupCtx)

	sharedFolders := sharedfolders.NewSharedFolders()
	// Clean up shared folders in the end.
	defer func() {
		if err := sharedFolders.UnshareAll(cleanupCtx, tconn, cont); err != nil {
			s.Error("Failed to unshare all folders: ", err)
		}
	}()

	if err := sharedFolders.ShareMyFilesOK(ctx, filesApp, tconn); err != nil {
		s.Fatal("Failed to share My files: ", err)
	}

	// This is necessary otherwise the next step will fail because a toast notification appears.
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for location on the desktop: ", err)
	}

	// Right click Manage Linux sharing on My files to open Manage shared folders page.
	if err = filesApp.SelectDirectoryContextMenuItem(ctx, sharedfolders.MyFiles, sharedfolders.ManageLinuxSharing); err != nil {
		s.Fatalf("Failed to click %q on My files: %s", sharedfolders.ManageLinuxSharing, err)
	}

	st, err := settings.FindSettingsPage(ctx, tconn, settings.PageNameMSF)
	if err != nil {
		s.Fatal("Failed to find Manage shared folders: ", err)
	}
	defer st.Close(ctx)

	if shared, err := st.GetSharedFolders(ctx); err != nil {
		s.Fatal("Failed to find the shared folders list: ", err)
	} else if want := []string{sharedfolders.MyFiles}; !reflect.DeepEqual(shared, want) {
		s.Fatalf("Failed to verify shared folders list, got %s, want %s", shared, want)
	}
}
