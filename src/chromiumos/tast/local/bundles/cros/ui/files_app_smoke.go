// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/ui/filesapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FilesAppSmoke,
		Desc: "Basic smoke test for Files app",
		Contacts: []string{
			"bhansknecht@chromium.org",
			"dhaddock@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func FilesAppSmoke(ctx context.Context, s *testing.State) {
	// TODO(crbug.com/987755): Port image preview part of the test from desktopui_FilesApp.

	cr := s.PreValue().(*chrome.Chrome)

	// Setup test file.
	const textFile = "test.txt"
	testFileLocation := filepath.Join(filesapp.DownloadPath, textFile)
	if err := ioutil.WriteFile(testFileLocation, []byte("blahblah"), 0644); err != nil {
		s.Fatalf("Creating file %s failed: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	// Get test api.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Get Files App.
	files, err := filesapp.NewFilesApp(ctx, tconn)
	if err != nil {
		s.Fatal("Creating Files App failed: ", err)
	}

	// Open Downloads folder and check for test file.
	if err := files.OpenDownloadsFolder(); err != nil {
		s.Fatal("Opening Downloads folder failed: ", err)
	}
	if err = files.WaitForElement(filesapp.RoleStaticText, textFile, 10*time.Second); err != nil {
		s.Fatal("Waiting for test file failed: ", err)
	}

	// Open More menu.
	if err = files.WaitForElement(filesapp.RoleButton, "More…", 10*time.Second); err != nil {
		s.Fatal("Waiting for More menu failed: ", err)
	}
	if err = files.ClickElement(filesapp.RoleButton, "More…"); err != nil {
		s.Fatal("Clicking More menu failed: ", err)
	}
	if err = files.WaitForElement(filesapp.RoleStaticText, "New folder", 10*time.Second); err != nil {
		s.Fatal("Waiting for More menu to open failed: ", err)
	}
}
