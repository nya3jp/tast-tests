// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/local/chrome/ash"
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
		Func:         SMB,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify Files app can mount an SMB share and verify the contents",
		Contacts: []string{
			"benreich@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "smbStarted",
	})
}

func SMB(ctx context.Context, s *testing.State) {
	fixt := s.FixtValue().(smb.FixtureData)

	// Write a file to the folder that is being shared via samba.
	const textFile = "test.txt"
	testFileLocation := filepath.Join(fixt.GuestSharePath, textFile)
	if err := ioutil.WriteFile(testFileLocation, []byte("blahblah"), 0644); err != nil {
		s.Fatalf("Failed to create file %q: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	// Open the test API.
	tconn, err := fixt.Chrome.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Launch the files application.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}

	// Files app will start with the window positioning of the previous session, this can cause the 3-dot menu position to end up out of view. Maximize the window to ensure the window bounds are all visible.
	window, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
		return strings.HasPrefix(w.Title, filesapp.FilesTitlePrefix)
	})
	if err != nil {
		s.Fatal("Failed to find the Files app window: ", err)
	}
	if err := ash.SetWindowStateAndWait(ctx, tconn, window.ID, ash.WindowStateMaximized); err != nil {
		s.Fatal("Failed to maximize the Files app window: ", err)
	}

	ui := uiauto.New(tconn)
	fileShareURLTextBox := nodewith.Name("File share URL").Role(role.TextField)
	if err := uiauto.Combine("Click add SMB file share",
		files.ClickMoreMenuItem("Services", "SMB file share"),
		ui.WaitForLocation(fileShareURLTextBox),
		ui.LeftClick(fileShareURLTextBox))(ctx); err != nil {
		s.Fatal("Failed to click add SMB share: ", err)
	}

	// Get a handle to the input keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard handle: ", err)
	}
	defer kb.Close()

	if err := kb.Type(ctx, `\\localhost\guestshare`); err != nil {
		s.Fatal("Failed entering the new SMB file share path: ", err)
	}

	if err := kb.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Failed pressing enter: ", err)
	}

	if err := uiauto.Combine("Wait for SMB to mount",
		files.OpenPath(filesapp.FilesTitlePrefix+smb.GuestShareName, smb.GuestShareName),
		files.WaitForFile(textFile))(ctx); err != nil {
		s.Fatal("Failed to wait for SMB to mount: ", err)
	}
}
