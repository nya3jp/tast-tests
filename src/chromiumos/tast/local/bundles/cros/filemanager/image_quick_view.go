// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ImageQuickView,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests image QuickView within the Files app",
		Contacts: []string{
			"chromeos-files-syd@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"files_app_test.png"},
		Pre:          chrome.LoggedIn(),
	})
}

func ImageQuickView(ctx context.Context, s *testing.State) {
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
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}

	openButton := nodewith.Name("Open").Role(role.Button)
	dimensionText := nodewith.Name(previewImageDimensions).Role(role.StaticText)
	// View image preview information of test image.
	if err := uiauto.Combine("View image preview information",
		files.OpenDownloads(),
		files.WithTimeout(10*time.Second).WaitForFile(previewImageFile),
		files.SelectFile(previewImageFile),
		files.WithTimeout(10*time.Second).WaitUntilExists(openButton),
		files.OpenQuickView(previewImageFile),
		files.WithTimeout(10*time.Second).WaitUntilExists(dimensionText))(ctx); err != nil {
		s.Fatal("Failed to view image preview information: ", err)
	}
}
