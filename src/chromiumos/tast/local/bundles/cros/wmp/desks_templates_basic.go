// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/bundles/cros/wmp/wmputils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DesksTemplatesBasic,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks desks can be saved as a desk template",
		Contacts: []string{
			"yzd@chromium.org",
			"chromeos-wmp@google.com",
			"cros-commercial-productivity-eng@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm", "no_kernel_upstream"},
		Timeout:      chrome.GAIALoginTimeout + arc.BootTimeout + 180*time.Second,
		SearchFlags: []*testing.StringPair{{
			Key: "feature_id",
			// Setup workspace templates.
			Value: "screenplay-a28f92cc-e3f3-47e7-8234-f7508f7722fe",
		}},
		VarDeps: []string{"ui.gaiaPoolDefault"},
		Params: []testing.Param{{
			Val: browser.TypeAsh,
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

func DesksTemplatesBasic(ctx context.Context, s *testing.State) {
	// TODO(b/238645466): Remove `no_kernel_upstream` from SoftwareDeps once kernel_uprev boards are more stable.
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Set up the browser.
	bt := s.Param().(browser.Type)
	cr, _, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, lacrosfixt.NewConfig(),
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.EnableFeatures("DesksTemplates", "EnableSavedDesks"),
		chrome.DisableFeatures("DeskTemplateSync"),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)
	defer closeBrowser(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	defer ash.CleanUpDesks(cleanupCtx, tconn)
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	ac := uiauto.New(tconn)

	// Setup for launching ARC apps.
	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store and Close: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(cleanupCtx)

	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("Failed to wait for ARC Intent Helper: ", err)
	}

	// Opens PlayStore, Browser and Files.
	browserApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find the primary browser app info: ", err)
	}
	appsList := []apps.App{browserApp, apps.FilesSWA, apps.PlayStore}

	if err := wmputils.OpenApps(ctx, tconn, ac, appsList); err != nil {
		s.Fatal("Failed to open apps: ", err)
	}

	// Enter overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}
	if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		s.Fatal("Failed to wait for the animation to be completed: ", err)
	}
	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)

	// Save current desk as `Template 1` of type `Template`.
	if err := ash.SaveCurrentDesk(ctx, ac, ash.Template, "Template 1"); err != nil {
		s.Fatal("Failed to save current desk as 'Template 1' of type 'Template': ", err)
	}

	// Verify saved desk.
	if err := ash.VerifySavedDesk(ctx, ac, []string{"Template 1"}); err != nil {
		s.Fatal("Failed to verify saved desk: ", err)
	}

	// Exit overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}
	if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		s.Fatal("Failed to wait for the animation to be completed: ", err)
	}

	// Verify window count.
	if err := wmputils.VerifyWindowCount(ctx, tconn, len(appsList)); err != nil {
		s.Fatal("Failed to verify window count: ", err)
	}

	// Enter overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}
	if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		s.Fatal("Failed to wait for the animation to be completed: ", err)
	}

	// Save current desk as `Saved Desk 1` of type `SaveAndRecall`.
	if err := ash.SaveCurrentDesk(ctx, ac, ash.SaveAndRecall, "Saved Desk 1"); err != nil {
		s.Fatal("Failed to save current desk as 'Saved Desk 1' of type 'SaveAndRecall': ", err)
	}

	// Exit and reenter library page.
	if err := ash.ExitAndReenterLibrary(ctx, ac, tconn); err != nil {
		s.Fatal("Failed to exit and reenter library page: ", err)
	}

	// Verify saved desk.
	if err := ash.VerifySavedDesk(ctx, ac, []string{"Template 1", "Saved Desk 1"}); err != nil {
		s.Fatal("Failed to verify saved desk: ", err)
	}

	// Exit overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}
	if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		s.Fatal("Failed to wait for the animation to be completed: ", err)
	}

	// Verify window count.
	if err := wmputils.VerifyWindowCount(ctx, tconn, 0); err != nil {
		s.Fatal("Failed to verify window count: ", err)
	}

	// Exit and reenter library page.
	if err := ash.ExitAndReenterLibrary(ctx, ac, tconn); err != nil {
		s.Fatal("Failed to exit and reenter library page: ", err)
	}

	// Verify that there are two saved desks.
	savedDeskViewInfo, err := ash.FindSavedDesks(ctx, ac)
	if err != nil {
		s.Fatal("Failed to find saved desks: ", err)
	}
	if len(savedDeskViewInfo) != 2 {
		s.Fatalf("Found inconsistent number of desks(s): got %v, want 2", len(savedDeskViewInfo))
	}

	// Exit overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("Failed to exit overview mode: ", err)
	}
	if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		s.Fatal("Failed to wait for overview animation to be completed: ", err)
	}

	// Verify that the app windows are closed.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to count windows: ", err)
	}
	if len(ws) != 0 {
		for _, window := range ws {
			s.Log("Found window ", window)
		}
		s.Fatalf("Found inconsistent number of window(s): got %v, want 0", len(ws))
	}
}
