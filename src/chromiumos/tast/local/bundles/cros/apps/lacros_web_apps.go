// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LacrosWebApps,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests installing and launching web apps with lacros",
		Contacts: []string{
			"mxcai@google.com",
			"lacros-team@google.com",
			"chromeos-apps-foundation-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Params: []testing.Param{
			{
				Fixture:           "lacrosKeepAlive",
				ExtraSoftwareDeps: []string{"lacros_stable"},
			},
			{
				Name:              "unstable",
				Fixture:           "lacrosKeepAlive",
				ExtraSoftwareDeps: []string{"lacros_unstable"},
			},
		},
	})
}

func LacrosWebApps(ctx context.Context, s *testing.State) {
	const (
		installTimeout = 30 * time.Second
		// Unique app id for Santa Tracker.
		appID   = "cbmkndbkpggpgbhflhebahghfebdomka"
		appName = "Santa Tracker"
	)

	tconn, err := s.FixtValue().(chrome.HasChrome).Chrome().TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Sometimes the lacros icon is not ready to be clicked during testing,
	// especially for new image. Wait a little here to ensure lacros is ready.
	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	// Launch lacros.
	l, err := lacros.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch lacros: ", err)
	}

	lacrosClosed := false
	defer func() {
		if !lacrosClosed {
			if err := l.Close(ctx); err != nil {
				s.Fatal("Failed to close lacros: ", err)
			}
		}
	}()

	const pwaURL = "https://santatracker.google.com/"
	conn, err := l.NewConn(ctx, pwaURL)
	if err != nil {
		s.Fatalf("Failed to open URL %q", pwaURL)
	}
	defer conn.Close()

	ui := uiauto.New(tconn).WithInterval(2 * time.Second)
	installIcon := nodewith.ClassName("PwaInstallView").Role(role.Button)
	if err := ui.WithTimeout(installTimeout).WaitUntilExists(installIcon)(ctx); err != nil {
		s.Fatal("Failed to wait for the install button in the omnibox")
	}
	installButton := nodewith.Name("Install").Role(role.Button)

	if err := uiauto.Combine("",
		ui.LeftClick(installIcon),
		ui.LeftClick(installButton))(ctx); err != nil {
		s.Fatal("Failed to click install button: ", err)
	}

	if err := ash.WaitForChromeAppInstalled(ctx, tconn, appID, installTimeout); err != nil {
		s.Fatal("Failed to wait for PWA to be installed: ", err)
	}

	if err := l.Close(ctx); err != nil {
		s.Fatal("Failed to close lacros: ", err)
	}
	lacrosClosed = true

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	if err := launcher.SearchAndLaunch(tconn, kb, appName)(ctx); err != nil {
		s.Fatalf("Failed to launch %s", appName)
	}

	if err := ash.WaitForApp(ctx, tconn, appID, time.Minute); err != nil {
		s.Fatalf("%s did not appear in shelf after launch", appName)
	}
}
