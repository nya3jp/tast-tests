// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShareFiles,
		Desc:         "Checks crostini files sharing",
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

func ShareFiles(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	cr := pre.Chrome
	cont := s.PreValue().(crostini.PreData).Container

	ownerID, err := cryptohome.UserHash(ctx, cr.User())
	if err != nil {
		s.Fatal("Failed to get user hash: ", err)
	}

	// Create MyFiles/hello.txt and MyFiles/shared/hello.txt.
	const (
		testFileName    = "hello.txt"
		testFileContent = "hello"
	)
	// TODO(crbug.com/911718): Using files under /MyFiles/Downloads, rather than
	// directly in /MyFiles since unshare in root of /MyFiles is broken.
	myfilesCros := filepath.Join("/home/user", ownerID, "MyFiles/Downloads")
	const myfilesCont = "/mnt/chromeos/MyFiles/Downloads"
	sharedCros := filepath.Join(myfilesCros, "shared")
	sharedCont := filepath.Join(myfilesCont, "shared")
	if err := os.MkdirAll(sharedCros, 0755); err != nil {
		s.Fatalf("Failed to create dir %v: %v", sharedCros, err)
	}

	myfilesCrosFileName := filepath.Join(myfilesCros, testFileName)
	myfilesContFileName := filepath.Join(myfilesCont, testFileName)
	myfilesFileContent := testFileContent + ":" + myfilesCrosFileName
	if err := ioutil.WriteFile(myfilesCrosFileName, []byte(myfilesFileContent), 0644); err != nil {
		s.Fatalf("Failed writing file %v: %v", myfilesCrosFileName, err)
	}

	sharedCrosFileName := filepath.Join(sharedCros, testFileName)
	sharedContFileName := filepath.Join(sharedCont, testFileName)
	sharedFileContent := testFileContent + ":" + sharedCrosFileName
	if err := ioutil.WriteFile(sharedCrosFileName, []byte(sharedFileContent), 0644); err != nil {
		s.Fatalf("Failed writing file %v: %v", sharedCrosFileName, err)
	}

	// Share paths.
	const filesAppExtID = "hhaomjibdihmijegdhdafkllkbggdgoj"
	bgURL := chrome.ExtensionBackgroundPageURL(filesAppExtID)
	fconn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		s.Fatalf("Failed to find %v: %v", bgURL, err)
	}
	const readyExpr = "'fileManagerPrivate' in chrome && 'background' in window"
	if err := fconn.WaitForExpr(ctx, readyExpr); err != nil {
		s.Fatalf("Failed waiting for %q: %v", readyExpr, err)
	}
	defer fconn.Close()

	// Share '/shared' dir, verify appropriate files are visible in the container.
	const localVolumeType = "downloads"
	sharePath(ctx, s, fconn, localVolumeType, "/Downloads/shared")
	if err := cont.CheckFileContent(ctx, sharedContFileName, sharedFileContent); err != nil {
		s.Fatalf("Wrong file content for %v: %v", sharedContFileName, err)
	}
	if err := crostini.VerifyFileNotInContainer(ctx, cont, myfilesContFileName); err != nil {
		s.Errorf("File %v unexpectedly exists", myfilesContFileName)
	}

	// Share root path, verify all files are now visible in the container.
	sharePath(ctx, s, fconn, localVolumeType, "/Downloads")
	if err := cont.CheckFileContent(ctx, sharedContFileName, sharedFileContent); err != nil {
		s.Fatalf("Wrong file content for %v: %v", sharedContFileName, err)
	}
	if err := cont.CheckFileContent(ctx, myfilesContFileName, myfilesFileContent); err != nil {
		s.Fatalf("Wrong file content for %v: %v", myfilesContFileName, err)
	}

	// Create dir and write file from container and verify it exists in the host.
	contWriteCrosFileName := filepath.Join(myfilesCros, "contwrite/"+testFileName)
	contWriteContDir := filepath.Join(myfilesCont, "contwrite")
	contWriteContFileName := filepath.Join(contWriteContDir, testFileName)
	contWriteFileContent := testFileContent + ":" + contWriteCrosFileName
	cmd := cont.Command(ctx, "mkdir", "-p", contWriteContDir)
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatalf("Failed to create dir %v in container: %v", contWriteContDir, err)
	}
	if err := crostini.CreateFileInContainer(ctx, cont, contWriteContFileName, contWriteFileContent); err != nil {
		s.Fatalf("Failed to write file %v in container: %v", contWriteContFileName, err)
	}
	if out, err := ioutil.ReadFile(contWriteCrosFileName); err != nil {
		cmd.DumpLog(ctx)
		s.Fatalf("Failed to read %v: %v", contWriteCrosFileName, err)
	} else if string(out) != contWriteFileContent {
		s.Errorf("%v contains %q; want %q", contWriteCrosFileName, out, contWriteFileContent)
	}

	// Unshare and verify files are no longer visible in container.
	unsharePath(ctx, s, fconn, localVolumeType, "/Downloads")
	if err := crostini.VerifyFileNotInContainer(ctx, cont, sharedContFileName); err != nil {
		s.Errorf("File %v unexpectedly exists", sharedContFileName)
	}
	if err := crostini.VerifyFileNotInContainer(ctx, cont, myfilesContFileName); err != nil {
		s.Errorf("File %v unexpectedly exists", myfilesContFileName)
	}
	if err := crostini.VerifyFileNotInContainer(ctx, cont, contWriteContFileName); err != nil {
		s.Errorf("File %v unexpectedly exists", contWriteContFileName)
	}
}

// sharePath calls FilesApp chrome.fileManagerPrivate API to share the specified path within the given volume with the container.
// Param volume must be a valid FilesApp VolumeManagerCommon.VolumeType.
func sharePath(ctx context.Context, s *testing.State, fconn *chrome.Conn, volume, path string) {
	js := fmt.Sprintf(
		`volumeManagerFactory.getInstance().then(vmgr => {
		    return util.getEntries(vmgr.getCurrentProfileVolumeInfo('%s'));
		 }).then(entries => {
		   const path = entries['%s'];
		   return new Promise((resolve, reject) => {
		     chrome.fileManagerPrivate.sharePathsWithCrostini('termina', [path], false, () => {
		       if (chrome.runtime.lastError !== undefined) {
		         return reject(new Error(chrome.runtime.lastError.message));
		       }
		       resolve();
		     });
		   });
		 })`, volume, path)
	if err := fconn.EvalPromise(ctx, js, nil); err != nil {
		s.Fatal("Running fileManagerPrivate.sharePathsWithCrostini failed: ", err)
	}
}

// unsharePath calls FilesApp chrome.fileManagerPrivate API to unshare the specified path within the given volume with the container.
// Param volume must be a valid FilesApp VolumeManagerCommon.VolumeType.
func unsharePath(ctx context.Context, s *testing.State, fconn *chrome.Conn, volume, path string) {
	js := fmt.Sprintf(
		`volumeManagerFactory.getInstance().then(vmgr => {
		   return util.getEntries(vmgr.getCurrentProfileVolumeInfo('%s'));
		 }).then(entries => {
		   const path = entries['%s'];
		   return new Promise((resolve, reject) => {
		     chrome.fileManagerPrivate.unsharePathWithCrostini('termina', path, () => {
		       if (chrome.runtime.lastError !== undefined) {
		         return reject(new Error(chrome.runtime.lastError.message));
		       }
		       resolve();
		     });
		   });
		 })`, volume, path)
	if err := fconn.EvalPromise(ctx, js, nil); err != nil {
		s.Fatal("Running fileManagerPrivate.unsharePathWithCrostini failed: ", err)
	}
}
