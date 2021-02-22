// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     OobeArcAppOpen,
		Desc:     "Launch ARC App post the OOBE Flow Setup Complete",
		Contacts: []string{"rnanjappan@google.com", "cros-arc-te@google.com"},
		//TODO(b/179637267): Enable once the bug is fixed.
		//Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 300*time.Second,
		Vars:    []string{"arc.parentUser", "arc.parentPassword"},
	})
}

func OobeArcAppOpen(ctx context.Context, s *testing.State) {

	const (
		appPkgName  = "com.google.android.apps.kids.familylinkhelper"
		appActivity = ".home.HomeActivity"
	)

	username := s.RequiredVar("arc.parentUser")
	password := s.RequiredVar("arc.parentPassword")

	cr, err := chrome.New(ctx, chrome.GAIALogin(), chrome.DontSkipOOBEAfterLogin(),
		chrome.ARCSupported(),
		chrome.Auth(username, password, "gaia-id"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	ui := uiauto.New(tconn)

	if err := uiauto.Combine("go through the oobe flow",
		ui.LeftClick(nodewith.NameRegex(regexp.MustCompile(
			"Accept and continue|Got it")).Role(role.Button)),
		ui.LeftClick(nodewith.Name("More").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Accept").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Continue").Role(role.Button)),
		ui.LeftClick(nodewith.Name("No thanks").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Done").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Get started").Role(role.Button)),
	)(ctx); err != nil {
		s.Fatal("Failed to go through the oobe flow: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	s.Log("Waiting for notification")
	_, err = ash.WaitForNotification(ctx, tconn, 5*time.Minute, ash.WaitTitle("Setup complete"))
	if err != nil {
		s.Fatal("Failed waiting for Setup complete notification: ", err)
	}

	s.Log("Waiting to check if app is installed before launching the app")
	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.FamilyLink.ID, 2*time.Minute); err != nil {
		s.Fatal("Failed to wait for Family Link App to install: ", err)
	}

	s.Log("Launch the Family Link App")
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create a new activity: ", err)
	}
	if err := act.Start(ctx, tconn); err != nil {
		act.Close()
		s.Fatal("Failed to start the activity: ", err)
	}

}
