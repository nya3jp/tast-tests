// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package a11y

import (
	"context"
	"fmt"
	"os"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/launcher"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Smoke,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests that a11y nodes on various browsers are accessible in Tast using the test extension from Ash",
		Contacts: []string{
			"hyungtaekim@chromium.org",
			"chromeos-sw-engprod@google.com",
			"chromeos-a11y-eng@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     lacros.ChromeTypeChromeOS,
		}, {
			Name:              "lacros",
			Fixture:           "lacrosUI",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraAttr:         []string{"informational"},
			Val:               lacros.ChromeTypeLacros,
		}, {
			Name:              "lacros_primary",
			Fixture:           "lacrosPrimary",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraAttr:         []string{"informational"},
			Val:               lacros.ChromeTypeLacros,
		}},
	})
}

func Smoke(ctx context.Context, s *testing.State) {
	ct := s.Param().(lacros.ChromeType)
	s.Log("Initializing ash-chrome and/or lacros-chrome based on the target browser: ", ct)
	if ct == lacros.ChromeTypeLacros {
		// Clean up user data dir to ensure a clean start.
		os.RemoveAll(launcher.LacrosUserDataDir)
	}
	cr, l, cs, err := lacros.Setup(ctx, s.FixtValue(), ct)
	if err != nil {
		s.Fatal("Failed to initialize setup: ", err)
	}
	if l != nil {
		defer l.Close(ctx)
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// TODO(crbug.com/1240344): Ensure the tablet mode is turned off until it is supported on Lacros.
	const tabletMode = false
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatalf("Failed to ensure the tablet mode is set to %v: %v", tabletMode, err)
	}
	defer cleanup(ctx)

	var app apps.App
	var topWindowName string
	switch ct {
	case lacros.ChromeTypeChromeOS:
		app, err = apps.ChromeOrChromium(ctx, tconn)
		if err != nil {
			s.Fatal("Could not determine the correct Chrome app to use: ", err)
		}
		topWindowName = "BrowserFrame"
	case lacros.ChromeTypeLacros:
		app = apps.Lacros
		topWindowName = "ExoShellSurface"
	default:
		s.Fatal("Unrecognized Chrome type: ", ct)
	}
	topLevelWindow := nodewith.Role(role.Window).HasClass(topWindowName)

	s.Logf("Opening a new tab in %v browser", ct)
	conn, err := cs.NewConn(ctx, "chrome://newtab")
	if err != nil {
		s.Fatalf("Failed to open a new tab in %v browser: %v", ct, err)
	}
	defer conn.Close()

	ui := uiauto.New(tconn)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Logf("Asserting that UI elements on browser window frame are accessible in %v browser", ct)
	for _, e := range []struct {
		name   string
		finder *nodewith.Finder
	}{
		{"Browser: New Tab", nodewith.HasClass("NewTabButton").Role(role.Button).Ancestor(topLevelWindow).First()},
		{"Browser: Tab Close", nodewith.HasClass("TabCloseButton").Role(role.Button).Ancestor(topLevelWindow).First()},
		{"Browser: Minimize", nodewith.HasClass("FrameCaptionButton").Name("Minimize").Role(role.Button).Ancestor(topLevelWindow)},
		{"Browser: Close", nodewith.HasClass("FrameCaptionButton").Name("Close").Role(role.Button).Ancestor(topLevelWindow)},
	} {
		if err = ui.WaitUntilExists(e.finder)(ctx); err != nil {
			s.Fatalf("Failed to find the UI element (%v) in %v: %v", e.name, ct, err)
		}
	}

	s.Logf("Asserting that the a11y node (rootWebArea) on the webview are accessible inside %v browser", ct)
	rootWebArea := nodewith.Role("rootWebArea").Ancestor(topLevelWindow).First()
	if err := ui.WaitUntilExists(rootWebArea)(ctx); err != nil {
		s.Fatalf("Failed to find the rootWebArea inside %v browser: %v", ct, err)
	}

	s.Logf("Asserting that mouse click works on the close button in %v browser", ct)
	closeButton := nodewith.HasClass("FrameCaptionButton").Name("Close").Role(role.Button).Ancestor(topLevelWindow)
	if err := uiauto.Combine(
		fmt.Sprintf("Click the close button in %v browser", ct),
		ui.WaitUntilExists(closeButton),
		ui.LeftClick(closeButton),
	)(ctx); err != nil {
		s.Fatalf("Failed to find and click the close button in %v: %v", ct, err)
	}

	if err = ash.WaitForAppClosed(ctx, tconn, app.ID); err != nil {
		s.Fatalf("%s did not close after clicking the close button: %s", app.Name, err)
	}
}
