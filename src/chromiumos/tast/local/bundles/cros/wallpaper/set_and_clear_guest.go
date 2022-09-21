// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wallpaper

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ash/ashproc"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/procutil"
	"chromiumos/tast/local/wallpaper"
	"chromiumos/tast/local/wallpaper/constants"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SetAndClearGuest,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test setting guest wallpaper is cleared on next login",
		Contacts: []string{
			"cowmoo@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

func SetAndClearGuest(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.GuestLogin())

	if err != nil {
		s.Fatal("Failed to create chrome instance: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test api connection: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	// Force Chrome to be in clamshell mode to make sure wallpaper preview is not enabled.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure DUT is not in tablet mode: ", err)
	}
	defer cleanup(ctx)

	ui := uiauto.New(tconn)
	if err := uiauto.Combine("Verify and set wallpaper",
		verifyDefaultWallpaper(ui),
		selectAndVerifyWallpaper(ui),
	)(ctx); err != nil {
		s.Fatal("Failed to verify and set wallpaper: ", err)
	}

	if err := logOutWithKeyboardShortcut()(ctx); err != nil {
		s.Fatal("Failed to log out: ", err)
	}

	// KeepState is necessary because otherwise wallpaper is cleared even if it would not be on a real device.
	if cr, err = chrome.New(ctx, chrome.KeepState(), chrome.GuestLogin()); err != nil {
		s.Fatal("Failed to restart Chrome: ", err)
	}
	defer cr.Close(ctx)

	if tconn, err = cr.TestAPIConn(ctx); err != nil {
		s.Fatal("Failed to re-establish test API connection: ", err)
	}
	ui = uiauto.New(tconn)

	if err := uiauto.NamedAction("Verify default wallpaper after sign-in", verifyDefaultWallpaper(ui))(ctx); err != nil {
		s.Fatal("Failed to re-verify default wallpaper after logout and login: ", err)
	}
}

func verifyDefaultWallpaper(ui *uiauto.Context) uiauto.Action {
	return uiauto.Combine("open wallpaper picker and verify default wallpaper",
		wallpaper.OpenWallpaperPicker(ui),
		wallpaper.WaitForWallpaperWithName(ui, "Default Wallpaper"),
		wallpaper.CloseWallpaperPicker(),
	)
}

func selectAndVerifyWallpaper(ui *uiauto.Context) uiauto.Action {
	return uiauto.Combine("select and verify wallpaper",
		wallpaper.OpenWallpaperPicker(ui),
		wallpaper.SelectCollection(ui, constants.ElementCollection),
		wallpaper.SelectImage(ui, constants.LightElementImage),
		wallpaper.WaitForWallpaperWithName(ui, constants.LightElementImage),
		wallpaper.CloseWallpaperPicker(),
	)
}

func logOutWithKeyboardShortcut() uiauto.Action {
	return func(ctx context.Context) error {

		process, err := ashproc.Root()
		if err != nil {
			return errors.Wrap(err, "failed to get Chrome process")
		}

		kb, err := input.Keyboard(ctx)
		defer kb.Close()
		if err != nil {
			return errors.Wrap(err, "failed to get keyboard")
		}
		if err := kb.Accel(ctx, "Ctrl+Shift+Q"); err != nil {
			return errors.Wrap(err, "failed to send first Ctrl+Shift+Q")
		}
		if err := kb.Accel(ctx, "Ctrl+Shift+Q"); err != nil {
			return errors.Wrap(err, "failed to send second Ctrl+Shift+Q")
		}
		// Wait for existing Chrome process to stop.
		if err := procutil.WaitForTerminated(ctx, process, 30*time.Second); err != nil {
			return errors.Wrap(err, "timeout waiting for Chrome to shutdown")
		}
		// Chrome is supposed to automatically restart.
		if _, err := ashproc.WaitForRoot(ctx, 30*time.Second); err != nil {
			return errors.Wrap(err, "failed waiting for Chrome to restart")
		}

		return nil
	}
}
