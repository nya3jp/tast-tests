// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink is used for writing Family Link tests.
package familylink

import (
	"context"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: IncognitoModeDisabled,
		Desc: "Tests that incognito mode is disabled for Unicorn users",
		Contacts: []string{
			"tobyhuang@chromium.org", "cros-families-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.GAIALoginTimeout,
		Fixture:      "familyLinkUnicornLogin",
	})
}

func IncognitoModeDisabled(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*familylink.FixtData).TestConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	// Get the expected browser.
	chromeApp, err := apps.ChromeOrChromium(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find the Chrome app: ", err)
	}

	s.Log("Right clicking the " + chromeApp.Name + " shelf app button")
	if err := uiauto.Combine("Right clicking the "+chromeApp.Name+" shelf app button",
		ui.RightClick(nodewith.Name(chromeApp.Name).Role(role.Button)),
		ui.WaitUntilExists(nodewith.Name("New window").Role(role.MenuItem)))(ctx); err != nil {
		s.Fatal("Failed to right click the "+chromeApp.Name+" shelf app button: ", err)
	}

	s.Log("Verifying the New Incognito window menu item does not exist")
	if err := ui.Exists(nodewith.Name("New Incognito window").Role(role.MenuItem))(ctx); err == nil {
		s.Fatal("Incognito mode detected and enabled")
	}
}
