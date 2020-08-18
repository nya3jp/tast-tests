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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/testexec"
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

	// Create MyFiles/ShareFiles.txt and MyFiles/shared/ShareFiles.txt.
	// We will share dir 'MyFiles/shared' and validate that the file inside
	// it is visible to the container, but not the one outside MyFiles/shared.
	const (
		testFileName    = "ShareFiles.txt"
		testFileContent = "ShareFiles"
	)
	// TODO(crbug.com/911718): Using files under /MyFiles/Downloads, rather than
	// directly in /MyFiles since unshare in root of /MyFiles is broken.
	myFilesCros := filepath.Join("/home/user", ownerID, "MyFiles/Downloads")
	const myFilesCont = "/mnt/chromeos/MyFiles/Downloads"
	sharedCros := filepath.Join(myFilesCros, "shared")
	sharedCont := filepath.Join(myFilesCont, "shared")
	if err := os.MkdirAll(sharedCros, 0755); err != nil {
		s.Fatalf("Failed to create dir %v: %v", sharedCros, err)
	}
	defer os.RemoveAll(sharedCros)

	myFilesCrosFileName := filepath.Join(myFilesCros, testFileName)
	myFilesContFileName := filepath.Join(myFilesCont, testFileName)
	myfilesFileContent := testFileContent + ":" + myFilesCrosFileName
	if err := ioutil.WriteFile(myFilesCrosFileName, []byte(myfilesFileContent), 0644); err != nil {
		s.Fatalf("Failed writing file %v: %v", myFilesCrosFileName, err)
	}
	defer os.Remove(myFilesCrosFileName)
	if err := crostini.VerifyFileNotInContainer(ctx, cont, myFilesContFileName); err != nil {
		s.Errorf("File %v unexpectedly exists", myFilesContFileName)
	}

	sharedCrosFileName := filepath.Join(sharedCros, testFileName)
	sharedContFileName := filepath.Join(sharedCont, testFileName)
	sharedFileContent := testFileContent + ":" + sharedCrosFileName
	if err := ioutil.WriteFile(sharedCrosFileName, []byte(sharedFileContent), 0644); err != nil {
		s.Fatalf("Failed writing file %v: %v", sharedCrosFileName, err)
	}
	defer os.Remove(sharedCrosFileName)
	if err := crostini.VerifyFileNotInContainer(ctx, cont, sharedContFileName); err != nil {
		s.Errorf("File %v unexpectedly exists", sharedContFileName)
	}

	// Share paths.
	const filesAppExtID = "hhaomjibdihmijegdhdafkllkbggdgoj"
	bgURL := chrome.ExtensionBackgroundPageURL(filesAppExtID)
	fconn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		s.Fatalf("Failed to find %v: %v", bgURL, err)
	}
	defer fconn.Close()
	const readyExpr = "'fileManagerPrivate' in chrome && 'background' in window"
	if err := fconn.WaitForExpr(ctx, readyExpr); err != nil {
		s.Fatalf("Failed waiting for %q: %v", readyExpr, err)
	}

	// Share '/shared' dir, verify appropriate files are visible in the container.
	const localVolumeType = "downloads"
	if err := sharePath(ctx, fconn, localVolumeType, "/Downloads/shared"); err != nil {
		s.Fatal("Failed to share: ", err)
	}
	if err := cont.CheckFileContent(ctx, sharedContFileName, sharedFileContent); err != nil {
		s.Fatalf("Wrong file content for %v: %v", sharedContFileName, err)
	}
	if err := crostini.VerifyFileNotInContainer(ctx, cont, myFilesContFileName); err != nil {
		s.Errorf("File %v unexpectedly exists", myFilesContFileName)
	}

	// Share root path, verify all files are now visible in the container.
	if err := sharePath(ctx, fconn, localVolumeType, "/Downloads"); err != nil {
		s.Fatal("Failed to share: ", err)
	}
	if err := cont.CheckFileContent(ctx, sharedContFileName, sharedFileContent); err != nil {
		s.Fatalf("Wrong file content for %v: %v", sharedContFileName, err)
	}
	if err := cont.CheckFileContent(ctx, myFilesContFileName, myfilesFileContent); err != nil {
		s.Fatalf("Wrong file content for %v: %v", myFilesContFileName, err)
	}

	// Create dir and write file from container and verify it exists in the host.
	contWriteCrosDir := filepath.Join(myFilesCros, "contwrite")
	contWriteCrosFileName := filepath.Join(contWriteCrosDir, testFileName)
	contWriteContDir := filepath.Join(myFilesCont, "contwrite")
	contWriteContFileName := filepath.Join(contWriteContDir, testFileName)
	contWriteFileContent := testFileContent + ":" + contWriteCrosFileName
	if err := cont.Command(ctx, "mkdir", "-p", contWriteContDir).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to create dir %v in container: %v", contWriteContDir, err)
	}
	defer os.RemoveAll(contWriteCrosDir)
	if err := cont.WriteFile(ctx, contWriteContFileName, contWriteFileContent); err != nil {
		s.Fatalf("Failed to write file %v in container: %v", contWriteContFileName, err)
	}
	if out, err := ioutil.ReadFile(contWriteCrosFileName); err != nil {
		s.Fatalf("Failed to read %v: %v", contWriteCrosFileName, err)
	} else if string(out) != contWriteFileContent {
		s.Errorf("%v contains %q; want %q", contWriteCrosFileName, out, contWriteFileContent)
	}

	// Unshare and verify files are no longer visible in container.
	if err := unsharePath(ctx, fconn, localVolumeType, "/Downloads"); err != nil {
		s.Fatal("Failed to unshare: ", err)
	}
	if err := crostini.VerifyFileNotInContainer(ctx, cont, sharedContFileName); err != nil {
		s.Errorf("File %v unexpectedly exists", sharedContFileName)
	}
	if err := crostini.VerifyFileNotInContainer(ctx, cont, myFilesContFileName); err != nil {
		s.Errorf("File %v unexpectedly exists", myFilesContFileName)
	}
	if err := crostini.VerifyFileNotInContainer(ctx, cont, contWriteContFileName); err != nil {
		s.Errorf("File %v unexpectedly exists", contWriteContFileName)
	}
}

// sharePath calls FilesApp chrome.fileManagerPrivate API to share the specified path within the given volume with the container.
// Param volume must be a valid FilesApp VolumeManagerCommon.VolumeType.
func sharePath(ctx context.Context, fconn *chrome.Conn, volume, path string) error {
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
		return errors.Wrapf(err, "fileManagerPrivate.sharePathsWithCrostini %v:%v failed", volume, path)
	}
	return nil
}

// unsharePath calls FilesApp chrome.fileManagerPrivate API to unshare the specified path within the given volume with the container.
// Param volume must be a valid FilesApp VolumeManagerCommon.VolumeType.
func unsharePath(ctx context.Context, fconn *chrome.Conn, volume, path string) error {
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
		return errors.Wrapf(err, "fileManagerPrivate.unsharePathWithCrostini %v:%v failed", volume, path)
	}
	return nil
}
