// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
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
		Func:     ShareMultipleFolders,
		Desc:     "Test sharing multiple folders with Crostini",
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

func ShareMultipleFolders(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()
	defer crostini.RunCrostiniPostTest(ctx, cont)

	sharedFolders := sharedfolders.NewSharedFolders()
	// Clean up shared folders in the end.
	defer func() {
		if err := sharedFolders.UnshareAll(cleanupCtx, tconn, cont); err != nil {
			s.Error("Failed to unshare all folders: ", err)
		}
		files, err := ioutil.ReadDir(filesapp.DownloadPath)
		if err != nil {
			s.Error("Failed to read files in Downloads: ", err)
		}
		for _, f := range files {
			os.RemoveAll(filepath.Join(filesapp.DownloadPath, f.Name()))
		}
	}()

	// Create three folders in Downloads.
	const (
		folder1 = "testShareFolder1"
		folder2 = "testShareFolder2"
		folder3 = "testShareFolder3"

		sharedFolder1 = sharedfolders.SharedDownloads + " › " + folder1
		sharedFolder2 = sharedfolders.SharedDownloads + " › " + folder2
		sharedFolder3 = sharedfolders.SharedDownloads + " › " + folder3
	)
	for _, folder := range []string{folder1, folder2, folder3} {
		if err := os.MkdirAll(filepath.Join(filesapp.DownloadPath, folder), 0755); err != nil {
			s.Fatalf("Failed to create %s in Downloads: %q", folder, err)
		}
	}

	// Open Files app.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}
	defer filesApp.Close(cleanupCtx)

	if err := filesApp.OpenDownloads(ctx); err != nil {
		s.Fatal("Failed to open Downloads: ", err)
	}

	// Right click two folders and select Share with Linux.
	if err = filesApp.SelectContextMenu(ctx, folder1, sharedfolders.ShareWithLinux); err != nil {
		s.Fatalf("Failed to share %s with Crostini: %s", folder1, err)
	}
	sharedFolders.AddFolder(sharedFolder1)
	if err = filesApp.SelectContextMenu(ctx, folder2, sharedfolders.ShareWithLinux); err != nil {
		s.Fatalf("Failed to share %s with Crostini: %s", folder2, err)
	}
	sharedFolders.AddFolder(sharedFolder2)

	st, err := settings.OpenLinuxSettings(ctx, tconn, settings.ManageSharedFolders)
	if err != nil {
		s.Fatal("Failed to open Manage shared folders: ", err)
	}
	defer st.Close(ctx)

	// Check results after sharing two folders.
	if err := checkShareFoldersResults(ctx, tconn, cont, st, []string{folder1, folder2}, []string{sharedFolder1, sharedFolder2}); err != nil {
		s.Fatal("Failed to check share results after sharing two folders: ", err)
	}

	// Unshare folder1.
	if err := st.UnshareFolder(ctx, sharedFolder1); err != nil {
		s.Fatalf("Failed to unshare %s: %s", sharedFolder1, err)
	}

	// Check results after unsharing one folder.
	if err := checkShareFoldersResults(ctx, tconn, cont, st, []string{folder2}, []string{sharedFolder2}); err != nil {
		s.Fatal("Failed to check share results after unshare one folder: ", err)
	}
}

func checkShareFoldersResults(ctx context.Context, tconn *chrome.TestConn, cont *vm.Container, s *settings.Settings, testFolders, sharedFolders []string) error {
	// Check shared folders on the Settings app.
	sharedFoldersList, err := s.GetSharedFolders(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find the shared folders list")
	}
	if err := listset.CheckListsMatch(sharedFoldersList, sharedFolders...); err != nil {
		return errors.Wrap(err, "failed to verify shared folders list")
	}

	// Check the file list in the container.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		list, err := cont.GetFileList(ctx, sharedfolders.MountPathDownloads)
		if err != nil {
			return err
		}
		if err := listset.CheckListsMatch(list, testFolders...); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to verify file list in container")
	}

	return nil
}
