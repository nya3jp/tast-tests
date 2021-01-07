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
		Func:         DisableEnableArc,
		Desc:         "Verify PlayStore can be turned off and On from Settings ",
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

// Time to wait for UI elements to appear in Play Store and Chrome
const defaultTimeout = 30 * time.Second

func DisableEnableArc(ctx context.Context, s *testing.State) {

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
	}, &testing.PollOptions{Timeout: defaultTimeout})

	s.Log("Turn On Play Store from Settings")
	if err := turnOnPlayStore(ctx, tconn); err != nil {
		s.Fatal("Failed to Turn On Play Store: ", err)
	}

	s.Log("Verify Play Store is Enabled")
	playStoreState, err := optin.GetPlayStoreState(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check GooglePlayStore State: ", err)
	}
	if playStoreState["enabled"] == false {
		s.Fatal("Playstore Disabled ")
	}

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

	appsbutton, err := ui.FindWithTimeout(ctx, tconn, appParams, defaultTimeout)
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

	playstore, err := ui.FindWithTimeout(ctx, tconn, playStoreParams, defaultTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find GooglePlayStore button")
	}
	defer playstore.Release(ctx)

	if err := playstore.FocusAndWait(ctx, defaultTimeout); err != nil {
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

	rmplaystore, err := ui.FindWithTimeout(ctx, tconn, removePlayStoreParams, defaultTimeout)
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

	rmAndroidApps, err := ui.FindWithTimeout(ctx, tconn, removeAndroidAppParams, defaultTimeout)
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

func turnOnPlayStore(ctx context.Context, tconn *chrome.TestConn) error {

	// Find the "Turn on" button and click.
	playStoreParams := ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "Google Play Store",
	}

	turnOnArc, err := ui.FindWithTimeout(ctx, tconn, playStoreParams, defaultTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find Turn On")
	}
	defer turnOnArc.Release(ctx)

	if err := turnOnArc.FocusAndWait(ctx, defaultTimeout); err != nil {
		return errors.Wrap(err, "failed to call focus() on Turn on")
	}

	if err := turnOnArc.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click Turn On")
	}

	// Find the "More" button and click.
	moreParams := ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "More",
	}
	more, err := ui.FindWithTimeout(ctx, tconn, moreParams, defaultTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find More button")
	}
	defer more.Release(ctx)

	if err := more.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click More button")
	}

	// Find the "Accept" button and click.
	acceptParams := ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "Accept",
	}
	accept, err := ui.FindWithTimeout(ctx, tconn, acceptParams, defaultTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find Accept button")
	}
	defer accept.Release(ctx)

	if err := accept.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to Click Accept button")
	}

	if err = optin.WaitForPlayStoreReady(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for Play Store to be ready")
	}

	return nil

}
