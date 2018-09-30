// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
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

	hash, err := cryptohome.UserHash(cr.User())
	if err != nil {
		s.Fatal("Failed to get user hash: ", err)
	}
	dir := "/media/fuse/crostini_" + hash + "_termina_penguin"

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	s.Log("Waiting for crostini to install (typically ~ 3 mins) and mount sshfs dir ", dir)
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

	if stat, err := os.Stat(dir); err != nil {
		s.Fatal("Didn't find sshfs mount: ", err)
	} else if !stat.IsDir() {
		s.Fatal("Didn't get directory for sshfs mount: ", dir)
	}

	// Verify mount works for writing a file.
	err = ioutil.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello"), 0644)
	if err != nil {
		s.Fatal("Failed writing file into sshfs mount directory: ", err)
	}

	// TODO(joehockey): Use terminal app to verify hello.txt.

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
	if _, err := os.Stat(dir); err == nil {
		s.Fatalf("SSHFS mount %v still existed after crostini uninstall", dir)
	}
}
