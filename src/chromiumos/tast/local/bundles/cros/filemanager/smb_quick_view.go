// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/smb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SMBQuickView,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify quick view gets image metadata from SMB share",
		Contacts: []string{
			"benreich@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"files_app_test.png"},
		Fixture:      "smbStarted",
	})
}

func SMBQuickView(ctx context.Context, s *testing.State) {
	fixt := s.FixtValue().(smb.FixtureData)

	// Setup the test image.
	const (
		previewImageFile       = "files_app_test.png"
		previewImageDimensions = "100 x 100"
	)
	imageFileLocation := filepath.Join(fixt.GuestSharePath, previewImageFile)
	if err := fsutil.CopyFile(s.DataPath(previewImageFile), imageFileLocation); err != nil {
		s.Fatalf("Failed to copy the test image to %s: %s", imageFileLocation, err)
	}
	defer os.Remove(imageFileLocation)

	// Open the test API.
	tconn, err := fixt.Chrome.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Get a handle to the input keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard handle: ", err)
	}
	defer kb.Close()

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}

	ui := uiauto.New(tconn)
	openButton := nodewith.Name("Open").Role(role.Button)
	dimensionText := nodewith.Name(previewImageDimensions).Role(role.StaticText)
	uiTimeout := 10 * time.Second
	// View image preview information of test image.
	if err := uiauto.Combine("ensure image dimensions show in quick view",
		files.ClickMoreMenuItem("Services", "SMB file share"),
		smb.AddFileShareAction(ui, kb, true /*=rememberPassword*/, smb.GuestShareName, "" /*=username*/, "" /*=password*/),
		files.OpenPath(filesapp.FilesTitlePrefix+smb.GuestShareName, smb.GuestShareName),
		files.WithTimeout(uiTimeout).WaitForFile(previewImageFile),
		files.SelectFile(previewImageFile),
		files.WithTimeout(uiTimeout).WaitUntilExists(openButton),
		files.OpenQuickView(previewImageFile),
		files.WithTimeout(uiTimeout).WaitUntilExists(dimensionText),
		files.Close,
	)(ctx); err != nil {
		s.Fatal("Failed to view image preview information: ", err)
	}
}
