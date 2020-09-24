// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	arcui "chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ChromeIntentPicker,
		Desc:     "Installs an ARC app and opens it from Chrome intent picker",
		Contacts: []string{"benreich@chromium.org", "mxcai@chromium.org"},
		Attr:     []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arc.ChromeIntentPicker.username", "arc.ChromeIntentPicker.password"},
	})
}

func ChromeIntentPicker(ctx context.Context, s *testing.State) {
	const facebookLitePackageName = "com.facebook.lite"
	const uiTimeout = 15 * time.Second

	username := s.RequiredVar("arc.ChromeIntentPicker.username")
	password := s.RequiredVar("arc.ChromeIntentPicker.password")

	// Setup Chrome.
	cr, err := chrome.New(ctx, chrome.GAIALogin(), chrome.Auth(username, password, "gaia-id"), chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Setup Chrome Test API Connection
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Optin to Play Store.
	s.Log("Opting into Play Store")
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}
	if err := optin.WaitForPlayStoreShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	d, err := arcui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	if err := installFacebookLite(ctx, tconn, a, d, facebookLitePackageName); err != nil {
		s.Fatal("Failed installing Facebook Lite: ", err)
	}

	// Navigate to URL
	conn, err := cr.NewConn(ctx, "")
	if err != nil {
		s.Fatal("Failed to create renderer: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := conn.Navigate(ctx, "https://www.facebook.com/groupcall/LINK"); err != nil {
		s.Fatal("Failed to navigate to the url: ", err)
	}

	// Locate and click on the intent picker button.
	params := chromeui.FindParams{
		ClassName: "IntentPickerView",
		Role:      chromeui.RoleTypeButton,
	}
	install, err := chromeui.FindWithTimeout(ctx, tconn, params, uiTimeout)
	if err != nil {
		s.Fatal("Failed to find intent picker button: ", err)
	}
	defer install.Release(ctx)

	if err := install.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click intent picker button: ", err)
	}

	// Make sure the Lite application has been selected.
	params = chromeui.FindParams{
		Name: "Lite",
		Role: chromeui.RoleTypeButton,
	}
	facebookLiteIntent, err := chromeui.FindWithTimeout(ctx, tconn, params, uiTimeout)
	if err != nil {
		s.Fatal("Failed to find the Facebook Lite intent: ", err)
	}
	defer facebookLiteIntent.Release(ctx)

	// Make sure the Lite application has been selected.
	params = chromeui.FindParams{
		ClassName: "MdTextButton",
		Name:      "Open",
		Role:      chromeui.RoleTypeButton,
	}
	openIntent, err := chromeui.FindWithTimeout(ctx, tconn, params, uiTimeout)
	if err != nil {
		s.Fatal("Failed to find the Open button: ", err)
	}
	defer openIntent.Release(ctx)

	testing.Sleep(ctx, 5*time.Second)

	if err := openIntent.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click the Open button: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			return w.ARCPackageName == facebookLitePackageName
		}); err != nil {
			return errors.Wrap(err, "failed identifying facebook lite window")
		}

		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second, Interval: 2 * time.Second}); err != nil {
		s.Fatal("Failed waiting for facebook lite window to appear: ", err)
	}
}

func installFacebookLite(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *arcui.Device, appPkgName string) error {
	if err := power.TurnOnDisplay(ctx); err != nil {
		return err
	}
	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		return err
	}
	if err := playstore.InstallApp(ctx, a, d, appPkgName, 3); err != nil {
		return err
	}

	return apps.Close(ctx, tconn, apps.PlayStore.ID)
}
