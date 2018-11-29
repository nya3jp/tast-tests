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
	sshfsMountDir := "/media/fuse/crostini_" + ownerID + "_termina_penguin"

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
	testSshfsMount(ctx, s, ownerID, sshfsMountDir)
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
		s.Error("Running autotestPrivate.runCrostiniUninstaller failed: ", err)
	}

	// Verify the sshfs mount is no longer active.
	if _, err := os.Stat(sshfsMountDir); err == nil {
		s.Errorf("SSHFS mount %v still existed after crostini uninstall", sshfsMountDir)
	}
}

func testSshfsMount(ctx context.Context, s *testing.State, ownerID string, sshfsMountDir string) {
	if stat, err := os.Stat(sshfsMountDir); err != nil {
		s.Fatal("Didn't find sshfs mount: ", err)
	} else if !stat.IsDir() {
		s.Fatal("Didn't get directory for sshfs mount: ", sshfsMountDir)
	}

	// Verify mount works for writing a file.
	fileName := "hello.txt"
	fileContent := "hello"
	createFile(s, filepath.Join(sshfsMountDir, fileName), fileContent)

	// Verify hello.txt in the container.
	cmd := vm.ContainerCommand(ctx, "termina", "penguin", ownerID, "cat", fileName)
	if out, err := cmd.Output(); err != nil {
		cmd.DumpLog(ctx)
		s.Error("Failed to run cat hello.txt: ", err)
	} else if string(out) != fileContent {
		s.Errorf("Invalid output from cat hello.txt got %q", string(out))
	}
}

func createFile(s *testing.State, path string, content string) {
	err := ioutil.WriteFile(path, []byte(content), 0644)
	if err != nil {
		s.Errorf("Failed writing file %s: %v", path, err)
	}
}

func testShareFiles(ctx context.Context, s *testing.State, ownerID string, cr *chrome.Chrome) {
	// Create Downloads/hello.txt and Downloads/shared/hello.txt.
	downloadsCros := filepath.Join("/home/user", ownerID, "Downloads")
	downloadsCont := "/mnt/chromeos/MyFiles/Downloads"
	sharedCros := filepath.Join(downloadsCros, "shared")
	sharedCont := filepath.Join(downloadsCont, "shared")
	if err := os.MkdirAll(sharedCros, 0755); err != nil {
		s.Errorf("Failed to create dir %s: %v", sharedCros, err)
	}
	downloadsCrosFileName := filepath.Join(downloadsCros, "hello.txt")
	downloadsContFileName := filepath.Join(downloadsCont, "hello.txt")
	downloadsFileContent := "hello:" + downloadsCrosFileName
	createFile(s, downloadsCrosFileName, downloadsFileContent)
	sharedCrosFileName := filepath.Join(sharedCros, "hello.txt")
	sharedContFileName := filepath.Join(sharedCont, "hello.txt")
	sharedFileContent := "hello:" + sharedCrosFileName
	createFile(s, sharedCrosFileName, sharedFileContent)

	// Share paths from Downloads.
	extID := "hhaomjibdihmijegdhdafkllkbggdgoj"
	testForReady := "'fileManagerPrivate' in chrome && 'background' in window"
	fconn, err := cr.ExtConn(ctx, extID, testForReady)
	if err != nil {
		s.Fatal("Creating FilesApp connection failed: ", err)
	}

	sharePath(ctx, s, fconn, "downloads", "/shared")
	verifyFileInContainer(ctx, s, ownerID, sharedContFileName, sharedFileContent)
	verifyNoFileInContainer(ctx, s, ownerID, downloadsContFileName)

	sharePath(ctx, s, fconn, "downloads", "/")
	verifyFileInContainer(ctx, s, ownerID, sharedContFileName, sharedFileContent)
	verifyFileInContainer(ctx, s, ownerID, downloadsContFileName, downloadsFileContent)

	unsharePath(ctx, s, fconn, "downloads", "/")
	verifyNoFileInContainer(ctx, s, ownerID, sharedContFileName)
	verifyNoFileInContainer(ctx, s, ownerID, downloadsContFileName)
}

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
		s.Error("Running fileManagerPrivate.sharePathsWithCrostini failed: ", err)
	}
}

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
		s.Error("Running fileManagerPrivate.unsharePathWithCrostini failed: ", err)
	}
}

func verifyFileInContainer(ctx context.Context, s *testing.State, ownerID string, path string, content string) {
	cmd := vm.ContainerCommand(ctx, "termina", "penguin", ownerID, "cat", path)
	if out, err := cmd.Output(); err != nil {
		cmd.DumpLog(ctx)
		s.Errorf("Failed to run cat %s: %v", path, err)
	} else if string(out) != content {
		s.Errorf("Invalid output from cat %s got %q", path, string(out))
	}
}

func verifyNoFileInContainer(ctx context.Context, s *testing.State, ownerID string, path string) {
	cmd := vm.ContainerCommand(ctx, "termina", "penguin", ownerID, "cat", path)
	if out, err := cmd.Output(); err == nil {
		cmd.DumpLog(ctx)
		s.Errorf("File exists when not expected %s: %v", path, string(out))
	}
}
