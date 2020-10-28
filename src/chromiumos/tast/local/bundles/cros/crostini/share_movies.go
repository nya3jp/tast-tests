// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"reflect"
	"time"

	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/local/crostini/ui/sharedfolders"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShareMovies,
		Desc:         "Test sharing Play files > Movies with Crostini",
		Contacts:     []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Vars:         []string{"keepState", "crostini.gaiaUsername", "crostini.gaiaPassword", "crostini.gaiaID"},
		Params: []testing.Param{
			{
				Name:              "stretch_amd64_stable_arc",
				ExtraData:         []string{"crostini_vm_amd64.zip", "crostini_test_container_metadata_stretch_amd64.tar.xz", "crostini_test_container_rootfs_stretch_amd64.tar.xz"},
				ExtraSoftwareDeps: []string{"amd64"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Pre:               crostini.StartedArtifactStretchARCEnabledGaia(),
				Timeout:           7 * time.Minute,
			}, {
				Name:              "buster_amd64_stable_arc",
				ExtraData:         []string{"crostini_vm_amd64.zip", "crostini_test_container_metadata_buster_amd64.tar.xz", "crostini_test_container_rootfs_buster_amd64.tar.xz"},
				ExtraSoftwareDeps: []string{"amd64"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Pre:               crostini.StartedArtifactBusterARCEnabledGaia(),
				Timeout:           7 * time.Minute,
			},
		},
	})
}

func ShareMovies(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container
	cr := s.PreValue().(crostini.PreData).Chrome

	defer crostini.RunCrostiniPostTest(ctx, s.PreValue().(crostini.PreData))

	// Show Play files.
	// It is necessary to call optin.Perform and optin.WaitForPlayStoreShown to make sure that Play files is shown.
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}
	if err := optin.FindOptInExtensionPageAndAcceptTerms(ctx, cr, false); err != nil {
		s.Fatal("Failed to find optin extension page: ", err)
	}
	if err := optin.WaitForPlayStoreShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}

	sharedFolders := sharedfolders.NewSharedFolders()
	// Unshare shared folders in the end.
	defer func() {
		if err := sharedFolders.UnshareAll(ctx, tconn, cont); err != nil {
			s.Error("Failed to unshare all folders: ", err)
		}
	}()

	// Open Files app.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}
	defer filesApp.Close(ctx)

	if err := filesApp.OpenDir(ctx, filesapp.Playfiles, "Files - "+filesapp.Playfiles); err != nil {
		s.Fatal("Failed to open Play files: ", err)
	}

	const Movies = "Movies"
	if err = filesApp.SelectContextMenu(ctx, Movies, sharedfolders.ShareWithLinux); err != nil {
		s.Fatal("Failed to share Movies with Crostini: ", err)
	}
	sharedFolders.AddFolder(filesapp.Playfiles + " › " + Movies)

	// Verify on Settings.
	st, err := settings.OpenLinuxSettings(ctx, tconn, settings.ManageSharedFolders)
	if err != nil {
		s.Fatal("Failed to open Manage shared folders: ", err)
	}
	defer st.Close(ctx)
	shared, err := st.GetSharedFolders(ctx)
	if err != nil {
		s.Fatal("Failed to find the shared folders list: ", err)
	}
	if want := []string{filesapp.Playfiles + " › " + Movies}; !reflect.DeepEqual(shared, want) {
		s.Fatalf("Failed to verify shared folders list, got %s, want %s", shared, want)
	}

	// Verify inside Crostini.
	if list, err := cont.GetFileList(ctx, sharedfolders.MountPath); err != nil {
		s.Fatalf("Failed to get file list of %s: %s", sharedfolders.MountPath, err)
	} else if want := []string{"fonts", sharedfolders.MountFolderPlay}; !reflect.DeepEqual(list, want) {
		s.Fatalf("Failed to verify file list in /mnt/chromeos, got %s, want %s", list, want)
	}

	if list, err := cont.GetFileList(ctx, sharedfolders.MountPathPlay); err != nil {
		s.Fatalf("Failed to get file list of %s: %s", sharedfolders.MountPathPlay, err)
	} else if want := []string{Movies}; !reflect.DeepEqual(list, want) {
		s.Fatalf("Failed to verify file list in /mnt/chromeos/PlayFiles, got %s, want %s", list, want)
	}
}
