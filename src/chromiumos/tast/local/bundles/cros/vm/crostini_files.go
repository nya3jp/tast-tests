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
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrostiniFiles,
		Desc:         "Checks that crostini files integration works including sshfs, shared folders, backup",
		Contacts:     []string{"joelhockey@chromium.org", "jkardatzke@chromium.org", "cros-containers-dev@google.com"},
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
	s.Log("Testing backup and restore")
	testBackupRestore(ctx, s, tconn, ownerID)

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
	const (
		testFileName    = "hello.txt"
		testFileContent = "hello"
	)
	crosFileName := filepath.Join(sshfsMountDir, testFileName)
	if err := ioutil.WriteFile(crosFileName, []byte(testFileContent), 0644); err != nil {
		s.Fatalf("Failed writing file %v: %v", crosFileName, err)
	}

	// Verify hello.txt in the container.
	verifyFileInContainer(ctx, s, ownerID, testFileName, testFileContent)
}

func testShareFiles(ctx context.Context, s *testing.State, ownerID string, cr *chrome.Chrome) {
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

	// Share '/shared' dir, verify appropriate files are visible in the container.
	const localVolumeType = "downloads"
	sharePath(ctx, s, fconn, localVolumeType, "/Downloads/shared")
	verifyFileInContainer(ctx, s, ownerID, sharedContFileName, sharedFileContent)
	verifyFileNotInContainer(ctx, s, ownerID, myfilesContFileName)

	// Share root path, verify all files are now visible in the container.
	sharePath(ctx, s, fconn, localVolumeType, "/Downloads")
	verifyFileInContainer(ctx, s, ownerID, sharedContFileName, sharedFileContent)
	verifyFileInContainer(ctx, s, ownerID, myfilesContFileName, myfilesFileContent)

	// Create dir and write file from container and verify it exists in the host.
	contWriteCrosFileName := filepath.Join(myfilesCros, "contwrite/"+testFileName)
	contWriteContDir := filepath.Join(myfilesCont, "contwrite")
	contWriteContFileName := filepath.Join(contWriteContDir, testFileName)
	contWriteFileContent := testFileContent + ":" + contWriteCrosFileName
	cmd := vm.DefaultContainerCommand(ctx, ownerID, "mkdir", "-p", contWriteContDir)
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatalf("Failed to create dir %v in container: %v", contWriteContDir, err)
	}
	createFileInContainer(ctx, s, ownerID, contWriteContFileName, contWriteFileContent)
	if out, err := ioutil.ReadFile(contWriteCrosFileName); err != nil {
		cmd.DumpLog(ctx)
		s.Fatalf("Failed to read %v: %v", contWriteCrosFileName, err)
	} else if string(out) != contWriteFileContent {
		s.Errorf("%v contains %q; want %q", contWriteCrosFileName, out, contWriteFileContent)
	}

	// Unshare and verify files are no longer visible in container.
	unsharePath(ctx, s, fconn, localVolumeType, "/Downloads")
	verifyFileNotInContainer(ctx, s, ownerID, sharedContFileName)
	verifyFileNotInContainer(ctx, s, ownerID, myfilesContFileName)
	verifyFileNotInContainer(ctx, s, ownerID, contWriteContFileName)
}

// Calls FilesApp chrome.fileManagerPrivate API to share the specified path within the given volume with the container.  Param volume must be a valid FilesApp VolumeManagerCommon.VolumeType.
func sharePath(ctx context.Context, s *testing.State, fconn *chrome.Conn, volume, path string) {
	js := fmt.Sprintf(
		`volumeManagerFactory.getInstance().then(vmgr => {
		    return util.getEntries(vmgr.getCurrentProfileVolumeInfo('%s'));
		 }).then(entries => {
		   const path = entries['%s'];
		   return new Promise((resolve, reject) => {
		     chrome.fileManagerPrivate.sharePathsWithCrostini([path], false, () => {
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

// Calls FilesApp chrome.fileManagerPrivate API to unshare the specified path within the given volume with the container.  Param volume must be a valid FilesApp VolumeManagerCommon.VolumeType.
func unsharePath(ctx context.Context, s *testing.State, fconn *chrome.Conn, volume, path string) {
	js := fmt.Sprintf(
		`volumeManagerFactory.getInstance().then(vmgr => {
		   return util.getEntries(vmgr.getCurrentProfileVolumeInfo('%s'));
		 }).then(entries => {
		   const path = entries['%s'];
		   return new Promise((resolve, reject) => {
		     chrome.fileManagerPrivate.unsharePathWithCrostini(path, () => {
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

func createFileInContainer(ctx context.Context, s *testing.State, ownerID, fileName, fileContent string) {
	cmd := vm.DefaultContainerCommand(ctx, ownerID, "sh", "-c", fmt.Sprintf("echo -n %s > %s", shutil.Escape(fileContent), fileName))
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatalf("Failed to write file %v in container: %v", fileName, err)
	}
}

func verifyFileInContainer(ctx context.Context, s *testing.State, ownerID, path, content string) {
	cmd := vm.DefaultContainerCommand(ctx, ownerID, "cat", path)
	if out, err := cmd.Output(); err != nil {
		cmd.DumpLog(ctx)
		s.Errorf("Failed to run cat %v: %v", path, err)
	} else if string(out) != content {
		s.Errorf("%v contains %q; want %q", path, out, content)
	}
}

func verifyFileNotInContainer(ctx context.Context, s *testing.State, ownerID string, path string) {
	cmd := vm.DefaultContainerCommand(ctx, ownerID, "sh", "-c", "[ -f "+path+" ]")
	if err := cmd.Run(); err == nil {
		s.Errorf("File %v unexpectedly exists", path)
	}
}

func testBackupRestore(ctx context.Context, s *testing.State, tconn *chrome.Conn, ownerID string) {
	const (
		testFileName    = "backup.txt"
		testFileContent = "backup"
	)

	createFileInContainer(ctx, s, ownerID, testFileName, testFileContent)
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

	// Delete the file.
	cmd := vm.DefaultContainerCommand(ctx, ownerID, "rm", testFileName)
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatalf("Failed to delete file %v in container: %v", testFileName, err)
	}
	verifyFileNotInContainer(ctx, s, ownerID, testFileName)

	// Restore container and verify file is back.
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

	verifyFileInContainer(ctx, s, ownerID, testFileName, testFileContent)
}
