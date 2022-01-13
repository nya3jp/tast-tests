// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package familylink

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/apputil"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ParentalControlsApp,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that the 'Family Link' app is launched after clicking the 'Parental controls' in the Accounts page in the Settings",
		Contacts: []string{
			"sun.tsai@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"cros-families-eng+test@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "arc"},
		Params: []testing.Param{
			{
				Name:    "unicorn",
				Fixture: "familyLinkUnicornArcLogin",
			}, {
				Name:    "geller",
				Fixture: "familyLinkGellerArcLogin",
			},
		},
		Timeout: 2*time.Minute + optin.OptinTimeout + apputil.InstallationTimeout,
	})
}

// ParentalControlsApp verifies that the 'Family Link' app is launched after clicking the 'Parental controls' in the Accounts page in the Settings.
func ParentalControlsApp(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn := s.FixtValue().(familylink.HasTestConn).TestConn()

	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to perform opt-in Play Store: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to create the instance of ARC: ", err)
	}
	defer a.Close(cleanupCtx)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create the keyboard: ", err)
	}
	defer kb.Close()

	app, err := apputil.NewApp(ctx, kb, tconn, a, "Family Link child and teen", "com.google.android.apps.kids.familylinkhelper")
	if err != nil {
		s.Fatal("Failed to create the instance of App: ", err)
	}
	defer app.Close(cleanupCtx, cr, s.HasError, s.OutDir())

	if err := app.Install(ctx); err != nil {
		s.Fatalf("Failed to install %s: %v", app.AppName, err)
	}

	// Launch the Settings and navigate to the Accounts page.
	settings, err := ossettings.LaunchAtPage(ctx, tconn, nodewith.Name("Accounts").Role(role.Link))
	if err != nil {
		s.Fatal("Failed to open Accounts page: ", err)
	}
	defer settings.Close(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	// Click the 'Parental controls' block in the Accounts page.
	if err := uiauto.New(tconn).LeftClick(nodewith.NameContaining("Parental controls Open").Ancestor(ossettings.WindowFinder))(ctx); err != nil {
		s.Fatal("Failed to click Parental controls: ", err)
	}
	// Defer close app window right after it's launched.
	defer func(ctx context.Context) {
		w, err := ash.GetARCAppWindowInfo(ctx, tconn, app.PkgName)
		if err != nil {
			s.Log("Failed to get ARC UI window info: ", err)
		}
		w.CloseWindow(ctx, tconn)
	}(cleanupCtx)

	// The app, Family Link child and teen, should be launched.
	if err := ash.WaitForVisible(ctx, tconn, app.PkgName); err != nil {
		s.Fatalf("Failed to open %s: %v", app.AppName, err)
	}
}
