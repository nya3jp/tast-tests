// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/wmp/wmputils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeskTemplatesDelete,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks desk templates can be delete",
		Contacts: []string{
			"yongshun@chromium.org",
			"yzd@chromium.org",
			"zhumatthew@chromium.org",
			"chromeos-wmp@google.com",
			"cros-commercial-productivity-eng@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm", "no_kernel_upstream"},
		Timeout:      chrome.GAIALoginTimeout + arc.BootTimeout + 180*time.Second,
		SearchFlags: []*testing.StringPair{{
			Key: "feature_id",
			// Delete workspace template.
			Value: "screenplay-02ff6408-5cb0-481c-bd6b-c170831d45ca",
		}},
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

func DeskTemplatesDelete(ctx context.Context, s *testing.State) {
	// TODO(b/238645466): Remove `no_kernel_upstream` from SoftwareDeps once kernel_uprev boards are more stable.
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.EnableFeatures("DesksTemplates", "EnableSavedDesks"),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

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

	// Enter overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}
	if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		s.Fatal("Failed to wait for the animation to be completed: ", err)
	}
	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)

	// Wait for saved desk sync.
	ash.WaitForSavedDeskSync(ctx, ac)

	// Exit overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}
	if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		s.Fatal("Failed to wait for the animation to be completed: ", err)
	}

	// Open Chrome and Files.
	appsList := []apps.App{apps.Chrome, apps.FilesSWA}
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

	// Save current desk as `Template 1` of type `Template`.
	if err := ash.SaveCurrentDesk(ctx, ac, ash.Template, "Template 1"); err != nil {
		s.Fatal("Failed to save current desk as 'Template 1' of type 'Template': ", err)
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

	// Close all existing windows.
	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to close all windows: ", err)
	}

	// Enter overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}
	if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		s.Fatal("Failed to wait for the animation to be completed: ", err)
	}

	// Enter library page.
	if err := ash.EnterLibraryPage(ctx, ac); err != nil {
		s.Fatal("Failed to enter library page: ", err)
	}

	// Additional desks may be synced to this test device. Verify that there is at least one saved desk.
	savedDeskViewInfo, err := ash.FindSavedDesks(ctx, ac)
	if err != nil {
		s.Fatal("Failed to find saved desks: ", err)
	}
	if len(savedDeskViewInfo) < 1 {
		s.Fatalf("Found inconsistent number of desks(s): got %v, want 1+", len(savedDeskViewInfo))
	}

	// Exit overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}
	if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		s.Fatal("Failed to wait for the animation to be completed: ", err)
	}

	// The for-loop below is for deleting the one desk which is saved in this test and also potential synced desks for test accounts.
	libraryButtonVisible := true
	for libraryButtonVisible {
		// Enter overview mode.
		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			s.Fatal("Failed to set overview mode: ", err)
		}
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			s.Fatal("Failed to wait for the animation to be completed: ", err)
		}

		// Check if library button is visible.
		if libraryButtonVisible, err = ash.IsLibraryButtonVisible(ctx, ac); err != nil {
			s.Fatal("Failed to check if library is visible: ", err)
		}

		// Enter library page, and delete all saved desks.
		if libraryButtonVisible {
			if err := ash.EnterLibraryPage(ctx, ac); err != nil {
				s.Fatal("Failed to enter library page: ", err)
			}
			if err := ash.DeleteAllSavedDesks(ctx, ac, tconn); err != nil {
				s.Fatal("Fail to clean up desk templates: ", err)
			}
		}

		// Exit overview mode.
		if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
			s.Fatal("Failed to set overview mode: ", err)
		}
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			s.Fatal("Failed to wait for the animation to be completed: ", err)
		}
	}
}
