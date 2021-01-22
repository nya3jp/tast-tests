// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UnicornParentPermission,
		Desc:         "Checks if App Install Triggers Parent Permission For Unicorn Account",
		Contacts:     []string{"rnanjappan@chromium.org", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Vars:         []string{"arc.parentUser", "arc.parentPassword", "arc.childUser", "arc.childPassword"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func UnicornParentPermission(ctx context.Context, s *testing.State) {
	const (
		askinMessageButtonText = "Ask in a message"
		askinPersonButtonText  = "Ask in person"
		installButtonText      = "install"
		playStoreSearchText    = "Search for apps & games"
		gamesAppName           = "roblox"
	)
	parentUser := s.RequiredVar("arc.parentUser")
	parentPass := s.RequiredVar("arc.parentPassword")
	childUser := s.RequiredVar("arc.childUser")
	childPass := s.RequiredVar("arc.childPassword")

	cr, err := chrome.New(ctx, chrome.GAIALogin(),
		chrome.Auth(childUser, childPass, "gaia-id"),
		chrome.ParentAuth(parentUser, parentPass), chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// Optin to Play Store.
	s.Log("Opting into Play Store")
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}
	// Verify PlayStore is Open.
	if err := optin.WaitForPlayStoreShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	// Try on Install Some Games App.
	searchText := d.Object(ui.ClassName("android.widget.TextView"), ui.Text(playStoreSearchText))
	if err := searchText.WaitForExists(ctx, 90*time.Second); err != nil {
		s.Fatal("searchText doesn't exist: ", err)
	}
	if err := searchText.Click(ctx); err != nil {
		s.Fatal("Failed to click on searchText: ", err)
	}

	searchTextEdit := d.Object(ui.ClassName("android.widget.EditText"), ui.Text(playStoreSearchText))
	if err := searchTextEdit.SetText(ctx, gamesAppName); err != nil {
		s.Fatal("Failed to set text to search: ", err)
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Fatal("Failed to click on KEYCODE_ENTER button: ", err)
	}

	installButton := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)"+installButtonText), ui.Enabled(true))
	if err := installButton.Exists(ctx); err != nil {
		s.Fatal("Install Button Exisits: ", err)
	}
	if err := installButton.Click(ctx); err != nil {
		s.Fatal("Failed to click  installButton: ", err)
	}

	// Verify Parent Permission Dialog is displayed.
	askinPersonButton := d.Object(ui.ClassName("android.widget.Button"), ui.Text(askinPersonButtonText), ui.Enabled(true))
	if err := askinPersonButton.WaitForExists(ctx, 90*time.Second); err != nil {
		s.Fatal("Ask in person button doesn't Exists: ", err)
	}

	if err := d.Object(ui.TextMatches(askinMessageButtonText)).Exists(ctx); err != nil {
		s.Fatal("Ask in a message button doesn't exist: ", err)
	}

	if err = askinPersonButton.Click(ctx); err != nil {
		s.Fatal("Failed to click  Ask in person: ", err)
	}

	parentPwd := d.Object(ui.ClassName("android.widget.EditText"), ui.Text(parentUser))
	if err := parentPwd.WaitForExists(ctx, 90*time.Second); err != nil {
		s.Fatal("parentPwd doesn't Exists: ", err)
	}

}
