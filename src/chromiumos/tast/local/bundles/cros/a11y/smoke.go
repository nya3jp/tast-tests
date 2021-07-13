// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package a11y

import (
	"context"
	"fmt"
	"os"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/lacros"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Smoke,
		Desc: "Tests that a11y nodes on various browsers are accessible in Tast using the test extension from Ash",
		Contacts: []string{
			"hyungtaekim@chromuim.org", "chromeos-sw-engprod@google.com", // Test author
			"chromeos-a11y-eng@google.com", // Backup mailing list
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     lacros.ChromeTypeChromeOS,
		}, {
			Name:              "lacros",
			Fixture:           "lacrosStartedByDataUI",
			ExtraData:         []string{launcher.DataArtifact},
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               lacros.ChromeTypeLacros,
		}, {
			Name:              "lacros_rootfs",
			Fixture:           "lacrosStartedFromRootfs",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               lacros.ChromeTypeLacros,
		}},
	})
}

func Smoke(ctx context.Context, s *testing.State) {

	// From chromeos-chrome in ash
	var cr *chrome.Chrome
	var tconn *chrome.TestConn

	// From lacros-chrome (only enabled on M93 or higher)
	var l *launcher.LacrosChrome
	var lconn *chrome.Conn

	// Abstract interface between chromeos-chrome and lacros-chrome
	var cs ash.ConnSource
	var ct lacros.ChromeType

	var app apps.App
	var err error

	ct = s.Param().(lacros.ChromeType)
	s.Log("Initializing chromeos-chrome and/or lacros-chrome based on the target browser: ", ct)
	if ct == lacros.ChromeTypeChromeOS {
		cr = s.FixtValue().(*chrome.Chrome)
		cs = cr
		tconn, err = cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to create Test API connection: ", err)
		}
		app = apps.App{ID: apps.Chrome.ID, Name: apps.Chrome.Name}
	}
	if ct == lacros.ChromeTypeLacros {
		f := s.FixtValue().(launcher.FixtData)
		var artifactPath string
		// TODO(crbug.com/1127165): Remove the artifactPath argument when we can use Data in fixtures
		// or by moving it into the launcher.EnsureLacrosChrome that is the only method using this argument.
		if f.Mode == launcher.PreExist {
			artifactPath = s.DataPath(launcher.DataArtifact)
		}
		cr, l, cs, err = lacros.Setup(ctx, s.FixtValue(), artifactPath, ct)
		if err != nil {
			s.Fatal("Failed to initialize setup: ", err)
		}
		tconn, err = cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to create Test API connection: ", err)
		}
		app = apps.App{ID: apps.Lacros.ID, Name: apps.Lacros.Name}
		defer lacros.CloseLacrosChrome(ctx, l)

		// Clean up user data dir to ensure a clean start.
		os.RemoveAll(launcher.LacrosUserDataDir)
	}

	s.Log("Starting A11y smoke test")
	ui := uiauto.New(tconn)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Logf("Open a new tab in %v browser", ct)
	lconn, err = cs.NewConn(ctx, "chrome://newtab")
	if err != nil {
		s.Fatalf("Failed to open a new tab in %v browser: %v", ct, err)
	}
	defer lconn.Close()

	// Note that chromeos-chrome and lacros-chrome have different layer hierarchy.
	var topLevelWindow *nodewith.Finder
	if ct == lacros.ChromeTypeChromeOS {
		topLevelWindow = nodewith.Role(role.Window).HasClass("BrowserFrame")
	} else {
		topLevelWindow = nodewith.Role(role.Window).HasClass("ExoShellSurface")
	}

	s.Logf("Assert that UI elements on browser window frame are accessible in %v browser", ct)
	for _, e := range []struct {
		name   string
		finder *nodewith.Finder
	}{
		{"Browser: New Tab", nodewith.HasClass("NewTabButton").Role(role.Button).Ancestor(topLevelWindow).First()},
		{"Browser: Tab Close", nodewith.HasClass("TabCloseButton").Role(role.Button).Ancestor(topLevelWindow).First()},
		{"Browser: Minimize", nodewith.HasClass("FrameCaptionButton").Name("Minimize").Role(role.Button).Ancestor(topLevelWindow)},
		{"Browser: Close", nodewith.HasClass("FrameCaptionButton").Name("Close").Role(role.Button).Ancestor(topLevelWindow)},
	} {
		if err = ui.WithTimeout(10 * time.Second).WaitUntilExists(e.finder)(ctx); err != nil {
			s.Fatalf("Failed to find the UI element in %v: %v", ct, err)
		}
		s.Logf("> %v is present", e.name)
	}

	s.Log("Assert that UI elements on system UI are accessible")
	for _, e := range []struct {
		name   string
		finder *nodewith.Finder
	}{
		{"System UI: Tray", nodewith.HasClass("UnifiedSystemTray").Role(role.Button)},
		{"System UI: Launcher", nodewith.HasClass("ash/HomeButton").Role(role.Button)},
	} {
		if err = ui.WithTimeout(10 * time.Second).WaitUntilExists(e.finder)(ctx); err != nil {
			s.Fatalf("Failed to find the UI element in %v: %v", ct, err)
		}
		s.Logf("> %v is present", e.name)
	}

	s.Logf("Assert that the a11y node on the webview is accessible inside %v browser", ct)
	rootWebArea := nodewith.Role("rootWebArea").Ancestor(topLevelWindow).First()
	if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(rootWebArea)(ctx); err != nil {
		s.Fatalf("Failed to find the rootWebArea inside %v browser: %v", ct, err)
	}
	s.Log("> Browser: rootWebArea is present")

	s.Logf("Assert that performing click on the close button works in %v browser", ct)
	closeButton := nodewith.HasClass("FrameCaptionButton").Name("Close").Role(role.Button).Ancestor(topLevelWindow)
	if err := uiauto.Combine(
		fmt.Sprintf("Click the close button in %v browser", ct),
		ui.WithTimeout(10*time.Second).WaitUntilExists(closeButton),
		ui.LeftClick(closeButton),
	)(ctx); err != nil {
		s.Fatalf("Failed to find and click the button in %v: %v", ct, err)
	}

	if err = ash.WaitForAppClosed(ctx, tconn, app.ID); err != nil {
		s.Fatalf("%s did not close successfully: %s", app.Name, err)
	}
}
