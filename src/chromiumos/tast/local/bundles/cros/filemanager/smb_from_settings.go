// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/smb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SMBFromSettings,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify SMB mount can be added from OS Settings",
		Contacts: []string{
			"benreich@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "smbStarted",
	})
}

func SMBFromSettings(ctx context.Context, s *testing.State) {
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

	// Get a handle to the input keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard handle: ", err)
	}
	defer kb.Close()

	// Launch the files application.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}

	ui := uiauto.New(tconn)
	addFileShareButton := nodewith.Name("Add file share").Role(role.Button)
	_, err = ossettings.LaunchAtPageURL(ctx, tconn, fixt.Chrome, "smbShares", ui.Exists(addFileShareButton))
	if err != nil {
		s.Fatal("Failed to launch Settings: ", err)
	}

	if err := uiauto.Combine("add guestshare via OS Settings",
		ui.LeftClick(addFileShareButton),
		smb.AddFileShareAction(ui, kb, true /*=rememberPassword*/, smb.GuestShareName, "" /*=username*/, "" /*=password*/),
		files.OpenPath(filesapp.FilesTitlePrefix+smb.GuestShareName, smb.GuestShareName),
		files.WaitForFile(textFile),
	)(ctx); err != nil {
		s.Fatal("Failed to add SMB share via OS Settings: ", err)
	}
}
