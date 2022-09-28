// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualDesksTabletMode,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks New desk button for tablet mode",
		Contacts: []string{
			"hongyulong@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "no_kernel_upstream"},
		Timeout:      chrome.GAIALoginTimeout + arc.BootTimeout + 180*time.Second,
		VarDeps:      []string{"ui.gaiaPoolDefault"},
	})
}

func VirtualDesksTabletMode(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Ensure there is no window open before test starts.
	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to ensure no window is open: ", err)
	}

	ac := uiauto.New(tconn)

	if err := enterTabletModeWithOneDesk(ctx, tconn, ac); err != nil {
		s.Fatal("Failed to test when entering tablet mode with one desk: ", err)
	}

	// Create a new desk.
	if err := ash.CreateNewDesk(ctx, tconn); err != nil {
		s.Fatal("Failed to creat a new desk: ", err)
	}

	if err := enterTabletModeWithTwoDesks(ctx, tconn, ac); err != nil {
		s.Fatal("Failed to test when entering tablet mode with two desks: ", err)
	}
}

func enterTabletModeWithOneDesk(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context) error {
	// Check if there is only one desk before entering tablet mode.
	dc, err := ash.GetDeskCount(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to count desks")
	}
	if dc > 1 {
		return errors.Wrapf(err, "count desk in a wrong number, expected: 1, actually: %d", dc)
	}

	cleanupCtx := ctx
	// Enable tablet mode.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		return errors.Wrap(err, "failed to ensure tablet mode")
	}
	defer cleanup(cleanupCtx)

	// Enter overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to set overview mode")
	}
	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)

	// Check that there is no New desk button.
	newDeskButtonView := nodewith.ClassName("ZeroStateIconButton")
	if err := ac.Gone(newDeskButtonView)(ctx); err != nil {
		return errors.Wrap(err, "failed to verify no New desk button in tablet mode")
	}

	return nil
}

func enterTabletModeWithTwoDesks(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context) error {
	// Check if there is only two desk before entering tablet mode.
	dc, err := ash.GetDeskCount(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to count desks")
	}
	if dc != 2 {
		return errors.Wrapf(err, "count desk in a wrong number, expected: 2, actually: %d", dc)
	}

	cleanupCtx := ctx
	// Enable tablet mode.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		return errors.Wrap(err, "failed to ensure tablet mode")
	}
	defer cleanup(cleanupCtx)

	// Enter overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to set overview mode")
	}
	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)

	// The New desk button should be shown when there are more than 1 desk.
	newDeskExpandedButtonView := nodewith.ClassName("ExpandedDesksBarButton")
	if err := ac.Exists(newDeskExpandedButtonView)(ctx); err != nil {
		return errors.Wrap(err, "failed to find the New desk button when there are 2 desks")
	}

	// Create a new desk.
	if err := ash.CreateNewDesk(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to create a new deskß")
	}

	// // Verifies that there are 3 desks.
	deskMiniViewsInfo, err := ash.FindDeskMiniViews(ctx, ac)
	if err != nil {
		return errors.Wrap(err, "failed to find desks")
	}
	if len(deskMiniViewsInfo) != 3 {
		return errors.Wrapf(err, "got %v desks, but want 3 desks", len(deskMiniViewsInfo))
	}

	// Remove Desk2 and Desk3 and the New desk button is still shown in overview mode.
	if err := ash.CleanUpDesks(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to remove Desk2 and Desk3")
	}

	newDeskZeroStateButtonView := nodewith.ClassName("ZeroStateIconButton")
	if err := ac.WithTimeout(5 * time.Second).WaitUntilExists(newDeskZeroStateButtonView)(ctx); err != nil {
		return errors.Wrap(err, "failed to find the New desk button after clean up all desks in overview mode")
	}

	// Exit and re-enter overview mode in tablet mode. The New desk button isn't shown.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to exit overview mode")
	}
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to set overview mode")
	}
	if err := ac.Gone(newDeskZeroStateButtonView)(ctx); err != nil {
		return errors.Wrap(err, "failed to verify no New desk button in tablet mode")
	}

	return nil
}
