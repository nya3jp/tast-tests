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

	"chromiumos/tast/local/bundles/cros/ui/filesapp"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FilesAppSmoke,
		Desc:         "Basic smoke test for Files app",
		Contacts:     []string{"bhansknecht@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func FilesAppSmoke(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	// Setup test file
	const textFile = "test.txt"
	testFileLocation := filepath.Join(filesapp.DownloadPath, textFile)
	if err := ioutil.WriteFile(testFileLocation, []byte("blahblah"), 0644); err != nil {
		s.Fatalf("Failed to create file %s: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	// Get test api
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Launch files app
	if err = filesapp.LaunchFilesApp(ctx, tconn); err != nil {
		s.Fatal("Launching Files App failed: ", err)
	}

	// Open Downloads folder and check for test file
	const downloads = "Downloads"
	if err = filesapp.WaitForElement(ctx, tconn, filesapp.RoleStaticText, downloads, 10*time.Second); err != nil {
		filesapp.LogDebugInfo(ctx, tconn, s)
		s.Fatal("Waiting for downloads failed: ", err)
	}
	if err = filesapp.ClickElement(ctx, tconn, filesapp.RoleStaticText, downloads); err != nil {
		filesapp.LogDebugInfo(ctx, tconn, s)
		s.Fatal("Clicking downloads failed: ", err)
	}
	if err = filesapp.WaitForElement(ctx, tconn, filesapp.RoleStaticText, textFile, 10*time.Second); err != nil {
		filesapp.LogDebugInfo(ctx, tconn, s)
		s.Fatal("Waiting for test file failed: ", err)
	}

	// Open More menu
	const more = "More…"
	if err = filesapp.WaitForElement(ctx, tconn, filesapp.RoleButton, more, 10*time.Second); err != nil {
		filesapp.LogDebugInfo(ctx, tconn, s)
		s.Fatal("Waiting for More menu failed: ", err)
	}
	if err = filesapp.ClickElement(ctx, tconn, filesapp.RoleButton, more); err != nil {
		filesapp.LogDebugInfo(ctx, tconn, s)
		s.Fatal("Clicking More menu failed: ", err)
	}
	if err = filesapp.WaitForElement(ctx, tconn, filesapp.RoleStaticText, "New folder", 10*time.Second); err != nil {
		filesapp.LogDebugInfo(ctx, tconn, s)
		s.Fatal("Waiting for More menu to open failed: ", err)
	}

	filesapp.CloseFilesApp(ctx, tconn)
}
