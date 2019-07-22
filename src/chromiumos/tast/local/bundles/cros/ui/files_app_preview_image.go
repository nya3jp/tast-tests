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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FilesAppPreviewImage,
		Desc:         "Tests image preview within the Files app",
		Contacts:     []string{"bhansknecht@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"files_app_test.png"},
		Pre:          chrome.LoggedIn(),
	})
}

func FilesAppPreviewImage(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	// Setup test image
	const previewImageFile = "files_app_test.png"
	const previewImageDimensions = "100 x 100"
	image, err := ioutil.ReadFile(s.DataPath(previewImageFile))
	if err != nil {
		s.Fatalf("Failed to load image %s: %s", s.DataPath(previewImageFile), err)
	}
	imageFileLocation := filepath.Join(filesapp.DownloadPath, previewImageFile)
	if err = ioutil.WriteFile(imageFileLocation, image, 0644); err != nil {
		s.Fatalf("Failed to create file %s: %s", imageFileLocation, err)
	}
	defer os.Remove(imageFileLocation)

	// Get test api
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Launch files app
	if err = filesapp.LaunchFilesApp(ctx, tconn); err != nil {
		s.Fatal("Launching Files App failed: ", err)
	}

	// Open Downloads folder
	const downloads = "Downloads"
	if err = filesapp.WaitForElement(ctx, tconn, filesapp.RoleStaticText, downloads, 10*time.Second); err != nil {
		filesapp.LogDebugInfo(ctx, tconn, s)
		s.Fatal("Waiting for downloads failed: ", err)
	}
	if err = filesapp.ClickElement(ctx, tconn, filesapp.RoleStaticText, downloads); err != nil {
		filesapp.LogDebugInfo(ctx, tconn, s)
		s.Fatal("Clicking downloads failed: ", err)
	}

	// Click image and wait for menu
	if err = filesapp.WaitForElement(ctx, tconn, filesapp.RoleStaticText, previewImageFile, 10*time.Second); err != nil {
		filesapp.LogDebugInfo(ctx, tconn, s)
		s.Fatal("Waiting for test image failed: ", err)
	}
	if err = filesapp.ClickElement(ctx, tconn, filesapp.RoleStaticText, previewImageFile); err != nil {
		filesapp.LogDebugInfo(ctx, tconn, s)
		s.Fatal("Clicking test image failed: ", err)
	}
	if err = filesapp.WaitForElement(ctx, tconn, filesapp.RoleButton, "Open", 10*time.Second); err != nil {
		filesapp.LogDebugInfo(ctx, tconn, s)
		s.Fatal("Waiting for test image failed: ", err)
	}

	// Setup keyboard
	kb, err := input.Keyboard(ctx)
	if err != nil {
		filesapp.LogDebugInfo(ctx, tconn, s)
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	// Activate image preview
	if err = filesapp.SetNavigationOnElement(ctx, tconn, filesapp.RoleStaticText, previewImageFile); err != nil {
		filesapp.LogDebugInfo(ctx, tconn, s)
		s.Fatal("Setting Navigation on image failed: ", err)
	}
	testing.Sleep(ctx, time.Second) // Fix flackiness on slow devices
	if err = kb.Accel(ctx, "Tab"); err != nil {
		filesapp.LogDebugInfo(ctx, tconn, s)
		s.Fatal("Failed to press tab key: ", err)
	}
	testing.Sleep(ctx, time.Second) // Fix flackiness on slow devices
	if err = kb.Accel(ctx, "Space"); err != nil {
		filesapp.LogDebugInfo(ctx, tconn, s)
		s.Fatal("Failed to press space key: ", err)
	}

	// Ensure preview opened
	if err = filesapp.WaitForElement(ctx, tconn, filesapp.RoleStaticText, previewImageDimensions, 10*time.Second); err != nil {
		filesapp.LogDebugInfo(ctx, tconn, s)
		s.Fatal("Waiting for image dimensions failed: ", err)
	}

	filesapp.CloseFilesApp(ctx, tconn)

}
