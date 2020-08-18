// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BackupRestore,
		Desc:         "Checks crostini backup and restore",
		Contacts:     []string{"joelhockey@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params: []testing.Param{
			{
				Name:              "artifact",
				Pre:               crostini.StartedByArtifact(),
				Timeout:           7 * time.Minute,
				ExtraData:         []string{crostini.ImageArtifact},
				ExtraHardwareDeps: crostini.CrostiniStable,
				ExtraAttr:         []string{"informational"},
			},
			{
				Name:              "artifact_unstable",
				Pre:               crostini.StartedByArtifact(),
				Timeout:           7 * time.Minute,
				ExtraData:         []string{crostini.ImageArtifact},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				ExtraAttr:         []string{"informational"},
			},
			{
				Name:      "download_stretch",
				Pre:       crostini.StartedByDownloadStretch(),
				Timeout:   10 * time.Minute,
				ExtraAttr: []string{"informational"},
			},
			{
				Name:      "download_buster",
				Pre:       crostini.StartedByDownloadBuster(),
				Timeout:   10 * time.Minute,
				ExtraAttr: []string{"informational"},
			},
		},
	})
}

func BackupRestore(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	cr := pre.Chrome
	tconn := pre.TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container

	ownerID, err := cryptohome.UserHash(ctx, cr.User())
	if err != nil {
		s.Fatal("Failed to get user hash: ", err)
	}

	const (
		testFileName    = "BackupRestore.txt"
		testFileContent = "BackupRestore"
	)

	if err := cont.WriteFile(ctx, testFileName, testFileContent); err != nil {
		s.Fatalf("Failed to write file %v in container: %v", testFileName, err)
	}
	if err := vm.ShrinkDefaultContainer(ctx, ownerID); err != nil {
		s.Fatal("Failed to shrink container for backup: ", err)
	}

	s.Log("Waiting for crostini to backup (typically ~ 2 mins)")
	if err := tconn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
		   chrome.autotestPrivate.exportCrostini('backup.tar.gz', () => {
		     if (chrome.runtime.lastError === undefined) {
		       resolve();
		     } else {
		       reject(new Error(chrome.runtime.lastError.message));
		     }
		   });
		 })`, nil); err != nil {
		s.Fatal("Running autotestPrivate.exportCrostini failed: ", err)
	}
	defer os.Remove(filepath.Join("/home/user", ownerID, "Downloads/backup.tar.gz"))

	// Delete the test file in the container.
	if err := crostini.RemoveContainerFile(ctx, cont, testFileName); err != nil {
		s.Fatalf("Failed to delete file %v in container: %v", testFileName, err)
	}
	if err := crostini.VerifyFileNotInContainer(ctx, cont, testFileName); err != nil {
		s.Errorf("File %v unexpectedly exists", testFileName)
	}

	// Restore the container and verify that the file is back.
	s.Log("Waiting for crostini to restore (typically ~ 1 min)")
	if err := tconn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
		   chrome.autotestPrivate.importCrostini('backup.tar.gz', () => {
		     if (chrome.runtime.lastError === undefined) {
		       resolve();
		     } else {
		       reject(new Error(chrome.runtime.lastError.message));
		     }
		   });
		 })`, nil); err != nil {
		s.Fatal("Running autotestPrivate.importCrostini failed: ", err)
	}

	if err := cont.CheckFileContent(ctx, testFileName, testFileContent); err != nil {
		s.Fatalf("Wrong file content for %v: %v", testFileContent, err)
	}
}
