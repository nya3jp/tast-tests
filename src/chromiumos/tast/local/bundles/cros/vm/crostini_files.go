// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const (
	filesAppExtID   = "hhaomjibdihmijegdhdafkllkbggdgoj" // FilesApp extension ID
	testFileName    = "hello.txt"                        // test file name
	testFileContent = "hello"                            // test file content
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrostiniFiles,
		Desc:         "Checks that crostini sshfs mount works",
		Attr:         []string{"informational"},
		Timeout:      10 * time.Minute,
		SoftwareDeps: []string{"chrome_login", "vm_host"},
	})
}

func CrostiniFiles(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to log in: ", err)
	}
	defer cr.Close(ctx)

	ownerID, err := cryptohome.UserHash(cr.User())
	if err != nil {
		s.Fatal("Failed to get user hash: ", err)
	}
	sshfsMountDir := fmt.Sprintf("/media/fuse/crostini_%s_%s_%s", ownerID, vm.DefaultVMName, vm.DefaultContainerName)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	s.Log("Waiting for crostini to install (typically ~ 3 mins) and mount sshfs dir ", sshfsMountDir)
	if err = tconn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
		   chrome.autotestPrivate.runCrostiniInstaller(() => {
		     if (chrome.runtime.lastError === undefined) {
		       resolve();
		     } else {
		       reject(new Error(chrome.runtime.lastError.message));
		     }
		   });
		 })`, nil); err != nil {
		s.Fatal("Running autotestPrivate.runCrostiniInstaller failed: ", err)
	}

	s.Log("Testing SSHFS Mount")
	testSSHFSMount(ctx, s, ownerID, sshfsMountDir)
	s.Log("Testing sharing files")
	testShareFiles(ctx, s, ownerID, cr)

	s.Log("Uninstalling crostini")
	if err = tconn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
			   chrome.autotestPrivate.runCrostiniUninstaller(() => {
			     if (chrome.runtime.lastError === undefined) {
			       resolve();
			     } else {
			       reject(new Error(chrome.runtime.lastError.message));
			     }
			   });
			 })`, nil); err != nil {
		s.Fatal("Running autotestPrivate.runCrostiniUninstaller failed: ", err)
	}

	// Verify the sshfs mount is no longer active.
	if _, err := os.Stat(sshfsMountDir); err == nil {
		s.Errorf("SSHFS mount %v still existed after crostini uninstall", sshfsMountDir)
	}
}

func testSSHFSMount(ctx context.Context, s *testing.State, ownerID, sshfsMountDir string) {
	if stat, err := os.Stat(sshfsMountDir); err != nil {
		s.Fatalf("Didn't find sshfs mount %v: %v", sshfsMountDir, err)
	} else if !stat.IsDir() {
		s.Fatal("Didn't get directory for sshfs mount ", sshfsMountDir)
	}

	// Verify mount works for writing a file.
	crosFileName := filepath.Join(sshfsMountDir, testFileName)
	if err := ioutil.WriteFile(crosFileName, []byte(testFileContent), 0644); err != nil {
		s.Fatalf("Failed writing file %v: %v", crosFileName, err)
	}

	// Verify hello.txt in the container.
	cmd := vm.DefaultContainerCommand(ctx, ownerID, "cat", testFileName)
	if out, err := cmd.Output(); err != nil {
		cmd.DumpLog(ctx)
		s.Errorf("Failed to cat %v: %v", testFileName, err)
	} else if string(out) != testFileContent {
		s.Errorf("%v contains %q; want %q", testFileName, out, testFileContent)
	}
}

func testShareFiles(ctx context.Context, s *testing.State, ownerID string, cr *chrome.Chrome) {
	// Create Downloads/hello.txt and Downloads/shared/hello.txt.
	downloadsCros := filepath.Join("/home/user", ownerID, "Downloads")
	const downloadsCont = "/mnt/chromeos/MyFiles/Downloads"
	sharedCros := filepath.Join(downloadsCros, "shared")
	sharedCont := filepath.Join(downloadsCont, "shared")
	if err := os.MkdirAll(sharedCros, 0755); err != nil {
		s.Fatalf("Failed to create dir %v: %v", sharedCros, err)
	}
	downloadsCrosFileName := filepath.Join(downloadsCros, testFileName)
	downloadsContFileName := filepath.Join(downloadsCont, testFileName)
	downloadsFileContent := testFileContent + ":" + downloadsCrosFileName
	if err := ioutil.WriteFile(downloadsCrosFileName, []byte(downloadsFileContent), 0644); err != nil {
		s.Fatalf("Failed writing file %v: %v", downloadsCrosFileName, err)
	}
	sharedCrosFileName := filepath.Join(sharedCros, testFileName)
	sharedContFileName := filepath.Join(sharedCont, testFileName)
	sharedFileContent := testFileContent + ":" + sharedCrosFileName
	if err := ioutil.WriteFile(sharedCrosFileName, []byte(sharedFileContent), 0644); err != nil {
		s.Fatalf("Failed writing file %v: %v", sharedCrosFileName, err)
	}

	// Share paths from Downloads.
	bgURL := chrome.ExtensionBackgroundPageURL(filesAppExtID)
	f := func(t *chrome.Target) bool { return t.URL == bgURL }
	fconn, err := cr.NewConnForTarget(ctx, f)
	if err != nil {
		s.Fatalf("Failed to find %v: %v", bgURL, err)
	}
	const readyExpr = "'fileManagerPrivate' in chrome && 'background' in window"
	if err := fconn.WaitForExpr(ctx, readyExpr); err != nil {
		s.Fatalf("Failed waiting for %q: %v", readyExpr, err)
	}
	defer fconn.Close()

	sharePath(ctx, s, fconn, "downloads", "/shared")
	verifyFileInContainer(ctx, s, ownerID, sharedContFileName, sharedFileContent)
	verifyFileNotInContainer(ctx, s, ownerID, downloadsContFileName)

	sharePath(ctx, s, fconn, "downloads", "/")
	verifyFileInContainer(ctx, s, ownerID, sharedContFileName, sharedFileContent)
	verifyFileInContainer(ctx, s, ownerID, downloadsContFileName, downloadsFileContent)

	unsharePath(ctx, s, fconn, "downloads", "/")
	verifyFileNotInContainer(ctx, s, ownerID, sharedContFileName)
	verifyFileNotInContainer(ctx, s, ownerID, downloadsContFileName)
}

// Calls FilesApp chrome.fileManagerPrivate API to share the specified path within the given volume with the container.  Param volume must be a valid FilesApp VolumeManagerCommon.VolumeType.
func sharePath(ctx context.Context, s *testing.State, fconn *chrome.Conn, volume string, path string) {
	js := fmt.Sprintf(
		`volumeManagerFactory.getInstance().then(vmgr => {
		    return util.getEntries(vmgr.getCurrentProfileVolumeInfo('%s'));
		 }).then(entries => {
		   const path = entries['%s'];
		   chrome.fileManagerPrivate.sharePathsWithCrostini([path], false, () => {
		     if (chrome.runtime.lastError !== undefined) {
		       throw new Error(chrome.runtime.lastError.message);
		     }
		   });
		 })`, volume, path)
	if err := fconn.EvalPromise(ctx, js, nil); err != nil {
		s.Fatalf("Running fileManagerPrivate.sharePathsWithCrostini failed: ", err)
	}
}

// Calls FilesApp chrome.fileManagerPrivate API to unshare the specified path within the given volume with the container.  Param volume must be a valid FilesApp VolumeManagerCommon.VolumeType.
func unsharePath(ctx context.Context, s *testing.State, fconn *chrome.Conn, volume string, path string) {
	js := fmt.Sprintf(
		`volumeManagerFactory.getInstance().then(vmgr => {
		    return util.getEntries(vmgr.getCurrentProfileVolumeInfo('%s'));
		 }).then(entries => {
		   const path = entries['%s'];
		   chrome.fileManagerPrivate.unsharePathWithCrostini(path, () => {
		     if (chrome.runtime.lastError !== undefined) {
		       throw new Error(chrome.runtime.lastError.message);
		     }
		   });
		 })`, volume, path)
	if err := fconn.EvalPromise(ctx, js, nil); err != nil {
		s.Fatal("Running fileManagerPrivate.unsharePathWithCrostini failed: ", err)
	}
}

func verifyFileInContainer(ctx context.Context, s *testing.State, ownerID, path, content string) {
	cmd := vm.DefaultContainerCommand(ctx, ownerID, "cat", path)
	if out, err := cmd.Output(); err != nil {
		cmd.DumpLog(ctx)
		s.Errorf("Failed to run cat %v: %v", path, err)
	} else if string(out) != content {
		s.Errorf("Invalid output from cat %v got %q", path, string(out))
	}
}

func verifyFileNotInContainer(ctx context.Context, s *testing.State, ownerID string, path string) {
	cmd := vm.DefaultContainerCommand(ctx, ownerID, "sh", "-c", "[ -f "+path+" ]")
	if err := cmd.Run(); err == nil {
		s.Errorf("File unexpectedly exists %v", path)
	}
}
