// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
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
		Func:         SMBPassword,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify an SMB share can be mounted and password remembered after Chrome restart",
		Contacts: []string{
			"benreich@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "smbStartedWithoutChrome",
		// TODO(crbug/1295640): Add test when the "Remember my password" button
		// is not checked.
		Params: []testing.Param{{
			Name: "remember_password",
			Val:  true,
		}},
	})
}

func SMBPassword(ctx context.Context, s *testing.State) {
	const (
		smbUsername = "chronos"
		smbPassword = "test0000"
		shareName   = "secureshare"
		textFile    = "test.txt"
	)

	fixt := s.FixtValue().(smb.FixtureData)
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to login to Chrome: ", err)
	}

	// Give 10 seconds to perform cleanup as the fixture does not manage
	// cleanup due to restarting the Chrome instance.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Unmount the SMB mount and close Chrome.
	defer func() {
		if cr != nil {
			if err := smb.UnmountAllSmbMounts(cleanupCtx, cr); err != nil {
				s.Fatal("Failed to unmount all SMB mounts: ", err)
			}
			if err := cr.Close(cleanupCtx); err != nil {
				s.Log("Failed to close Chrome: ", err)
			}
		}
	}()

	// Write a file to the folder that is being shared via samba.
	testFileLocation := filepath.Join(fixt.GuestSharePath, textFile)
	if err := ioutil.WriteFile(testFileLocation, []byte("blahblah"), 0644); err != nil {
		s.Fatalf("Failed to create file %q: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Launch the files application.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}

	// Get a handle to the input keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard handle: ", err)
	}
	defer kb.Close()

	ui := uiauto.New(tconn)
	fileShareURLTextBox := nodewith.Name("File share URL").Role(role.TextField)
	if err := uiauto.Combine("add secureshare via Files context menu",
		files.ClickMoreMenuItem("Services", "SMB file share"),
		ui.WaitForLocation(fileShareURLTextBox),
		ui.LeftClick(fileShareURLTextBox),
		kb.TypeAction(`\\localhost\`+shareName),
		kb.AccelAction("Tab"), // Tab past share name to Username box.
		kb.AccelAction("Tab"),
		kb.TypeAction(smbUsername),
		kb.AccelAction("Tab"), // Tab to the password box.
		kb.TypeAction(smbPassword),
		kb.AccelAction("Enter"), // Add the Samba share.
		files.OpenPath(filesapp.FilesTitlePrefix+shareName, shareName),
		files.WaitForFile(textFile),
	)(ctx); err != nil {
		s.Fatal("Failed to click add SMB share: ", err)
	}

	// Close the current Chrome instance
	if err := cr.Close(ctx); err != nil {
		s.Fatal("Failed to close Chrome instance: ", err)
	}
	// Reset the tconn as the Chrome instance has closed.
	tconn = nil

	// Restart Chrome but ensure the local state is maintained, this emulates
	// a Chrome reboot to ensure the password is remembered.
	cr, err = chrome.New(ctx, chrome.KeepState())
	if err != nil {
		s.Fatal("Failed to login to Chrome: ", err)
	}

	// Open the test API.
	tconn, err = cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Launch the files application.
	files, err = filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}

	// Verify the Samba share does not prompt for a password.
	if err := uiauto.Combine("ensure secureshare is still available",
		files.OpenPath(filesapp.FilesTitlePrefix+shareName, shareName),
		files.WaitForFile(textFile),
	)(ctx); err != nil {
		s.Fatal("Failed to ensure secureshare is still available: ", err)
	}
}
