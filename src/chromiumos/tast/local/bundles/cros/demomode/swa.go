// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package demomode

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/demomode/fixture"
	"chromiumos/tast/local/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SWA,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that the Demo Mode System Web App launches in fullscreen and goes to windowed mode after user interaction",
		Contacts:     []string{"jacksontadie@google.com", "cros-demo-mode-eng@google.com"},
		Fixture:      fixture.PostDemoModeOOBE,
		Attr:         []string{"group:mainline", "informational"},
		// Demo Mode uses Zero Touch Enrollment for enterprise enrollment, which
		// requires a real TPM.
		// We require "arc" and "chrome_internal" because the ARC TOS screen
		// is only shown for chrome-branded builds when the device is ARC-capable.
		SoftwareDeps: []string{"chrome", "chrome_internal", "arc", "tpm"},
		Params: []testing.Param{{
			Name: "online",
			Val:  true, // shouldRunOnline
		}, {
			Name: "offline",
			Val:  false, // shouldRunOnline
		}},
	})
}

func SWA(ctx context.Context, s *testing.State) {
	// Behavior that will be tested both online and offline.
	restartChromeAndVerifySWA := func(ctx context.Context) error {
		cr, err := chrome.New(ctx,
			chrome.NoLogin(),
			chrome.ARCSupported(),
			chrome.KeepEnrollment(),
			chrome.EnableFeatures("DemoModeSWA"),
			// --force-devtools-available forces devtools on regardless of policy (devtools is
			// disabled in Demo Mode policy) to support connecting to the test API extension.
			//
			// --component-updater=test-request adds a "test-request" parameter to Omaha
			// update requests, causing the fetched Demo Mode App component to come from a
			// test cohort.
			chrome.ExtraArgs("--force-devtools-available", "--component-updater=test-request"))
		if err != nil {
			return errors.Wrap(err, "failed to restart Chrome")
		}

		clearUpCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
		defer cancel()
		defer cr.Close(clearUpCtx)

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to create the test API connection")
		}
		defer faillog.DumpUITreeOnError(clearUpCtx, s.OutDir(), s.HasError, tconn)
		ui := uiauto.New(tconn).WithTimeout(100 * time.Second)

		// Verify that splash screen has disappeared before moving mouse.
		splashScreen := nodewith.ClassName("WallpaperView").Ancestor(nodewith.ClassName("AlwaysOnTopWallpaperContainer"))
		if err := ui.WaitUntilGone(splashScreen)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait until splash screen is gone")
		}

		s.Log("Waiting for Demo Mode App to launch")
		demoApp := nodewith.Name("Demo Mode App").First()
		if err := ui.WaitUntilExists(demoApp)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait until Demo App exists")
		}

		// Verify app is fullscreen (Attract Loop) by checking that the toolbar
		// is not present
		s.Log("Confirming that app is in fullscreen mode")
		toolbar := nodewith.Role(role.Toolbar).Ancestor(demoApp)
		if err := ui.WaitUntilGone(toolbar)(ctx); err != nil {
			return errors.Wrap(err, "failed to confirm that the toolbar is not present")
		}

		pc := pointer.NewMouse(tconn)
		defer pc.Close()

		demoAppLocation, _ := ui.Location(ctx, demoApp)

		// Move mouse (arbitrarily) from center of demo app to screen corner to
		// trigger interaction, breaking fullscreen Attract Loop.
		if err := pc.Drag(
			demoAppLocation.CenterPoint(),
			pc.DragTo(coords.NewPoint(0, 0), 1*time.Second))(ctx); err != nil {
			return errors.Wrap(err, "failed to drag mouse across screen")
		}

		// Assert that app is now windowed by presence of toolbar.
		s.Log("Waiting for windowed Highlights mode")
		if err := ui.WaitUntilExists(toolbar)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for the toolbar to be present")
		}
		return nil
	}

	shouldRunOnline := s.Param().(bool)
	if shouldRunOnline {
		if err := restartChromeAndVerifySWA(ctx); err != nil {
			s.Fatal("Failed to verify SWA functionality online: ", err)
		}
	} else {
		if err := network.ExecFuncOnChromeOffline(ctx, restartChromeAndVerifySWA); err != nil {
			s.Fatal("Failed to verify SWA functionality offline: ", err)
		}
	}
}
