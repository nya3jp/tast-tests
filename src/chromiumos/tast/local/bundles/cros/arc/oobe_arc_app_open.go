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
		Func:         OobeArcAppOpen,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Launch ARC App post the OOBE Flow Setup Complete",
		Contacts:     []string{"rnanjappan@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 20*time.Minute,
		VarDeps: []string{"arc.parentUser", "arc.parentPassword"},
	})
}

func OobeArcAppOpen(ctx context.Context, s *testing.State) {

	const (
		appPkgName  = "com.google.android.apps.books"
		appActivity = ".app.BooksActivity"
	)

	username := s.RequiredVar("arc.parentUser")
	password := s.RequiredVar("arc.parentPassword")

	cr, err := chrome.New(ctx,
		chrome.DontSkipOOBEAfterLogin(),
		chrome.ARCSupported(),
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}))
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

	skip := nodewith.Name("Skip").Role(role.StaticText)
	noThanks := nodewith.Name("No thanks").Role(role.Button)
	getStarted := nodewith.Name("Get started").Role(role.Button)

	if err := uiauto.Combine("go through the oobe flow",
		ui.LeftClick(nodewith.NameRegex(regexp.MustCompile(
			"Accept and continue|Got it")).Role(role.Button)),
		ui.IfSuccessThen(ui.WithTimeout(10*time.Second).WaitUntilExists(skip), ui.LeftClick(skip)),
		ui.LeftClick(nodewith.Name("More").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Accept").Role(role.Button)),
		ui.IfSuccessThen(ui.WithTimeout(60*time.Second).WaitUntilExists(noThanks), ui.LeftClick(noThanks)),
		ui.IfSuccessThen(ui.WithTimeout(20*time.Second).WaitUntilExists(noThanks), ui.LeftClick(noThanks)),
		ui.LeftClick(getStarted),
	)(ctx); err != nil {
		s.Fatal("Failed to go through the oobe flow: ", err)
	}

	next := nodewith.Name("Next").Role(role.Button)
	if err := uiauto.Combine("go through the tablet specific flow",
		ui.IfSuccessThen(ui.WithTimeout(30*time.Second).WaitUntilExists(next), ui.LeftClick(next)),
		ui.IfSuccessThen(ui.WithTimeout(30*time.Second).WaitUntilExists(next), ui.LeftClick(next)),
		ui.IfSuccessThen(ui.WithTimeout(30*time.Second).WaitUntilExists(next), ui.LeftClick(next)),
		ui.IfSuccessThen(ui.WithTimeout(30*time.Second).WaitUntilExists(getStarted), ui.LeftClick(getStarted)),
	)(ctx); err != nil {
		s.Fatal("Failed to test oobe Arc tablet flow: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	statusArea := nodewith.HasClass("ash/StatusAreaWidgetDelegate")
	s.Log("Waiting for notification")
	_, err = ash.WaitForNotification(ctx, tconn, 20*time.Minute, ash.WaitTitle("Setup complete"))
	if err != nil {
		if err := ui.LeftClick(statusArea)(ctx); err != nil {
			s.Log("Failed to click status area : ", err)
		}
		s.Fatal("Failed waiting for Setup complete notification: ", err)
	}

	s.Log("Waiting to check if app is installed before launching the app")
	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.PlayBooks.ID, 2*time.Minute); err != nil {
		s.Fatal("Failed to wait for app to install: ", err)
	}

	s.Log("Launch the App")
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create a new activity: ", err)
	}
	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		act.Close()
		s.Fatal("Failed to start the activity: ", err)
	}

}
