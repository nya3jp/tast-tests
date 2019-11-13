// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FilesAppImageQuickView,
		Desc: "Tests image QuickView within the Files app",
		Contacts: []string{
			"bhansknecht@chromium.org",
			"dhaddock@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"files_app_test.png"},
		Pre:          chrome.LoggedIn(),
	})
}

func FilesAppImageQuickView(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	// Setup the test image.
	const (
		previewImageFile       = "files_app_test.png"
		previewImageDimensions = "100 x 100"
	)
	imageFileLocation := filepath.Join(filesapp.DownloadPath, previewImageFile)
	if err := fsutil.CopyFile(s.DataPath(previewImageFile), imageFileLocation); err != nil {
		s.Fatalf("Failed to copy the test image to %s: %s", imageFileLocation, err)
	}
	defer os.Remove(imageFileLocation)

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}
	defer files.Close(ctx)

	// Open the Downloads folder.
	if err := files.OpenDownloads(ctx); err != nil {
		s.Fatal("Opening Downloads folder failed: ", err)
	}

	// Click the test image and wait for Open button in top bar.
	if err := files.WaitForFile(ctx, previewImageFile, 10*time.Second); err != nil {
		s.Fatal("Waiting for test file failed: ", err)
	}
	if err := files.SelectFile(ctx, previewImageFile); err != nil {
		s.Fatal("Waiting for test file failed: ", err)
	}
	params := ui.FindParams{
		Name: "Open",
		Role: "button",
	}
	if err := files.Root.WaitForDescendantAdded(ctx, params, 10*time.Second); err != nil {
		s.Fatal("Waiting for Open button failed: ", err)
	}

	// Open QuickView for the test image and check dimensions.
	if err := files.OpenQuickView(ctx, previewImageFile); err != nil {
		s.Fatal("Openning QuickView failed: ", err)
	}
	params = ui.FindParams{
		Name: previewImageDimensions,
		Role: "staticText",
	}
	if err := files.Root.WaitForDescendantAdded(ctx, params, 10*time.Second); err != nil {
		s.Fatal("Waiting for Open button failed: ", err)
	}
}
