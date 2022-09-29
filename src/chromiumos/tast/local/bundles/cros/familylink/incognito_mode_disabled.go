// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink is used for writing Family Link tests.
package familylink

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IncognitoModeDisabled,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests that incognito mode is disabled for Unicorn users",
		Contacts: []string{
			"tobyhuang@chromium.org", "cros-families-eng+test@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      time.Minute,
		Params: []testing.Param{{
			Fixture: "familyLinkUnicornLogin",
		}, {
			Name:    "lacros",
			Fixture: "familyLinkUnicornLoginWithLacros",
		}},
	})
}

func IncognitoModeDisabled(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(familylink.HasTestConn).TestConn()

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	// Get the primary browser.
	chromeApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find the Chrome app: ", err)
	}

	// Chrome app name doesn't exactly match the chrome shelf name so modify it here for simpler code later.
	if chromeApp.Name == apps.Chrome.Name && chromeApp.ID != apps.LacrosID {
		chromeApp.Name = "Google Chrome"
	}

	s.Logf("Right clicking the %s shelf app button", chromeApp.Name)
	if err := uiauto.Combine(fmt.Sprintf("Right clicking the %s shelf app button", chromeApp.Name),
		ui.RightClick(nodewith.Name(chromeApp.Name).Role(role.Button)),
		ui.WaitUntilExists(nodewith.Role(role.MenuItem).First()))(ctx); err != nil {
		s.Fatal(fmt.Sprintf("Failed to right click the %s shelf app button: ", chromeApp.Name), err)
	}

	s.Log("Verifying the New Incognito window menu item does not exist")
	if err := ui.Exists(nodewith.Name("New Incognito window").Role(role.MenuItem))(ctx); err == nil {
		s.Fatal("Incognito mode detected and enabled: ", err)
	}
}
