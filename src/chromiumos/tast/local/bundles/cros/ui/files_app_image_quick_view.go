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
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/filesapp"
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
		Attr:         []string{"informational"},
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
	image, err := ioutil.ReadFile(s.DataPath(previewImageFile))
	if err != nil {
		s.Fatalf("Failed to load image %s: %s", s.DataPath(previewImageFile), err)
	}
	imageFileLocation := filepath.Join(filesapp.DownloadPath, previewImageFile)
	if err := ioutil.WriteFile(imageFileLocation, image, 0644); err != nil {
		s.Fatalf("Failed to create file %s: %s", imageFileLocation, err)
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

	// Open the Downloads folder.
	if err := files.OpenDownloads(ctx); err != nil {
		s.Fatal("Opening Downloads folder failed: ", err)
	}

	// Click the test image and wait for Open button in top bar.
	if err := files.WaitForElement(ctx, filesapp.RoleStaticText, previewImageFile, 10*time.Second); err != nil {
		s.Fatal("Waiting for test image failed: ", err)
	}
	if err := files.ClickElement(ctx, filesapp.RoleStaticText, previewImageFile); err != nil {
		s.Fatal("Clicking test image failed: ", err)
	}
	if err := files.WaitForElement(ctx, filesapp.RoleButton, "Open", 10*time.Second); err != nil {
		s.Fatal("Waiting for Open button failed: ", err)
	}

	// Setup keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	// Open QuickView for the test image and check dimensions.
	if err := kb.Accel(ctx, "Space"); err != nil {
		s.Fatal("Failed to press space key: ", err)
	}
	if err := files.WaitForElement(ctx, filesapp.RoleStaticText, previewImageDimensions, 10*time.Second); err != nil {
		s.Fatal("Waiting for image dimensions failed: ", err)
	}
}
