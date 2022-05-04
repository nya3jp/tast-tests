// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mgs

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/printpreview"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mgs"
	"chromiumos/tast/testing"
)

const fileName = "chrome___dino_.pdf"

func init() {
	testing.AddTest(&testing.Test{
		Func:         FilesApp,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that files app is working with managed guest sessions by saving and opening a pdf",
		Contacts: []string{
			"mpolzer@google.com", // Test author
			"chromeos-kiosk-eng+TAST@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.FakeDMSEnrolled,
	})
}

// FilesApp Tests the Files App by:
// 1. Starting a managed guest session (MGS).
// 2. Opening chrome://dino and printing the page with Print to PDF
// 3. Using the Files App to open the saved PDF file
// 4. Verifying the file is opened by checking the Gallery app is opened
func FilesApp(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	mgs, cr, err := mgs.New(
		ctx,
		fdms,
		mgs.DefaultAccount(),
		mgs.AutoLaunch(mgs.MgsAccountID),
	)
	if err != nil {
		s.Fatal("Failed to start MGS: ", err)
	}
	defer mgs.Close(ctx)

	if _, err := cr.NewConn(ctx, "chrome://dino"); err != nil {
		s.Fatal("Failed to navigate: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}
	defer kb.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	ui := uiauto.New(tconn)
	if err := uiauto.Combine("open Print Preview with shortcut Ctrl+P",
		kb.AccelAction("Ctrl+P"),
		printpreview.WaitForPrintPreview(tconn),
		kb.AccelAction("enter"),
		ui.WaitUntilExists(nodewith.NameStartingWith(fileName).First()),
		kb.AccelAction("enter"),
	)(ctx); err != nil {
		s.Fatal("Failed to save to pdf: ", err)
	}

	fa, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch files app: ", err)
	}

	if err := uiauto.Combine("open filesapp and look at file",
		fa.OpenDownloads(),
		fa.WaitForFile(fileName),
		fa.OpenFile(fileName),
	)(ctx); err != nil {
		s.Fatal("Failed to verify file exists: ", err)
	}

	if err := ash.WaitForApp(ctx, tconn, apps.Gallery.ID, time.Second*30); err != nil {
		s.Fatal("Failed to check Gallery in shelf: ", err)
	}
}
