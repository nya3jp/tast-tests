// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DisableArc,
		Desc:         "Verify PlayStore can be turned off in Settings ",
		Contacts:     []string{"rnanjappan@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: chrome.LoginTimeout + arc.BootTimeout + 120*time.Second,
		Vars:    []string{"arc.username", "arc.password"},
	})
}

func DisableArc(ctx context.Context, s *testing.State) {

	username := s.RequiredVar("arc.username")
	password := s.RequiredVar("arc.password")

	cr, err := chrome.New(ctx, chrome.GAIALogin(), chrome.Auth(username, password, "gaia-id"), chrome.ARCSupported(), chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Optin to PlayStore.
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}
	if err := optin.WaitForPlayStoreShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}
	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to close Play Store: ", err)
	}

	s.Log("Turn Play Store Off from Settings")
	if err := turnOffPlayStore(ctx, tconn); err != nil {
		s.Fatal("Failed to Turn Off Play Store: ", err)
	}

	s.Log("Verify Play Store is Off")
	testing.Poll(ctx, func(ctx context.Context) error {
		playStoreState, err := optin.GetPlayStoreState(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get some playstore state")
		}
		if playStoreState["enabled"] == true {
			return errors.New("Playstore is On Still")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second})

}

func turnOffPlayStore(ctx context.Context, tconn *chrome.TestConn) error {

	// Navigate to Android Settings.
	if err := apps.Launch(ctx, tconn, apps.Settings.ID); err != nil {
		return errors.Wrap(err, "failed to launch the Settings app")
	}

	appParams := ui.FindParams{
		Role: ui.RoleTypeHeading,
		Name: "Apps",
	}

	appsbutton, err := ui.FindWithTimeout(ctx, tconn, appParams, 30*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find Apps heading")
	}
	defer appsbutton.Release(ctx)

	if err := appsbutton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click the Apps heading")
	}

	// Find the "Google Play Store" button and click.
	playStoreParams := ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "Google Play Store",
	}

	playstore, err := ui.FindWithTimeout(ctx, tconn, playStoreParams, 30*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find GooglePlayStore button")
	}
	defer playstore.Release(ctx)

	if err := playstore.FocusAndWait(ctx, 30*time.Second); err != nil {
		return errors.Wrap(err, "failed to call focus() on GooglePlayStore")
	}

	if err := playstore.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click the GooglePlayStore button")
	}

	// Find the "Remove Google Play Store" button and click.
	removePlayStoreParams := ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "Remove Google Play Store",
	}

	rmplaystore, err := ui.FindWithTimeout(ctx, tconn, removePlayStoreParams, 30*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find Google Play Store")
	}
	defer rmplaystore.Release(ctx)

	if err := rmplaystore.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click Remove Google Play Store")
	}

	// Find the "Remove Android Apps" button and click.
	removeAndroidAppParams := ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "Remove Android apps",
	}

	rmAndroidApps, err := ui.FindWithTimeout(ctx, tconn, removeAndroidAppParams, 30*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find Remove Android Apps")
	}
	defer rmAndroidApps.Release(ctx)

	if err := rmAndroidApps.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click Remove Android Apps")
	}

	if err = ash.WaitForAppClosed(ctx, tconn, apps.PlayStore.ID); err != nil {
		return errors.Wrap(err, "failed to Close Play Store")
	}
	return nil

}
