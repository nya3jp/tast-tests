// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OobeArcAppOpen,
		Desc:         "Launch ARC App post the OOBE Flow Setup Complete",
		Contacts:     []string{"rnanjappan@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: chrome.LoginTimeout + arc.BootTimeout + 600*time.Second,
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

	// To tap on Accept and continue/Got it whichever the OOBE flow displays.
	if err := ui.LeftClick(nodewith.Name("Accept and continue").Role(role.Button))(ctx); err != nil {
		s.Log("Failed to click Accept and continue : ", err)
		if err := ui.LeftClick(nodewith.Name("Got it").Role(role.Button))(ctx); err != nil {
			s.Fatal("Failed to click Got it : ", err)
		}
	}

	if err := uiauto.Run(ctx,
		ui.LeftClick(nodewith.Name("More").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Accept").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Continue").Role(role.Button)),
		ui.LeftClick(nodewith.Name("No thanks").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Done").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Get started").Role(role.Button)),
	); err != nil {
		s.Fatal("Failed to go through the oobe flow: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	s.Log("Waiting for notification")
	_, err = ash.WaitForNotification(ctx, tconn, 600*time.Second, ash.WaitTitle("Setup complete"))
	if err != nil {
		s.Fatal("Failed waiting for Setup complete notification: ", err)
	}

	s.Log("Waiting few seconds before launching the installed app")
	if err := testing.Sleep(ctx, 15*time.Second); err != nil {
		s.Fatal("Failed to wait: ", err)
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
