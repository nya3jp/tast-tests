// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FloatWindow,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test that the float shortcut works on a floatable window",
		Contacts: []string{
			"hewer@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "ash",
			Val:  browser.TypeAsh,
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

// FloatWindow floats an open app window using the keyboard shortcut.
func FloatWindow(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	bt := s.Param().(browser.Type)
	cr, _, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, lacrosfixt.NewConfig(),
		chrome.EnableFeatures("CrOSLabsFloatWindow"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)
	defer closeBrowser(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	ui := uiauto.New(tconn)

	// If we are in lacros-chrome, there is Chrome window open that should be closed.
	if bt == browser.TypeLacros {
		if err := ui.LeftClick(nodewith.Name("Close").First())(ctx); err != nil {
			s.Fatal("Failed to close Lacros browser: ", err)
		}
	}

	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the files app: ", err)
	}
	defer filesApp.Close(cleanupCtx)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to initialize keyboard: ", err)
	}
	if err := kb.Accel(ctx, "Search+Alt+F"); err != nil {
		s.Fatal("Failed to input float accelerator: ", err)
	}

	// Get files app window (should be the only window open).
	window, err := ash.WaitForAppWindow(ctx, tconn, apps.FilesSWA.ID)
	if err != nil {
		s.Fatal("Failed to fetch app dindows: ", err)
	}

	if window.State != ash.WindowStateFloated {
		s.Fatal("Window is not in the floated state: ", err)
	}

	if err := ui.LeftClick(nodewith.Name("Restore"))(ctx); err != nil {
		s.Fatal("Failed to unfloat the window: ", err)
	}

	window, err = ash.WaitForAppWindow(ctx, tconn, apps.FilesSWA.ID)
	if err != nil {
		s.Fatal("Failed to fetch app dindows: ", err)
	}

	if window.State != ash.WindowStateNormal {
		s.Fatal("Failed to return window to the normal state: ", err)
	}
}
