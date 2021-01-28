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
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EnableArc,
		Desc:         "Verify PlayStore can be turned On from Settings ",
		Contacts:     []string{"rnanjappan@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:              "normal",
			ExtraSoftwareDeps: []string{"android_p"},
			Val:               "normal",
		}, {
			Name:              "unicorn",
			ExtraSoftwareDeps: []string{"android_p"},
			Val:               "unicorn",
		}, {
			Name:              "normal_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               "normal",
		}, {
			Name:              "unicorn_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               "unicorn",
		}},
		Timeout: chrome.LoginTimeout + arc.BootTimeout + 120*time.Second,
		Vars:    []string{"arc.parentUser", "arc.parentPassword", "arc.childUser", "arc.childPassword"},
	})
}

func EnableArc(ctx context.Context, s *testing.State) {

	parentUser := s.RequiredVar("arc.parentUser")
	parentPass := s.RequiredVar("arc.parentPassword")
	childUser := s.RequiredVar("arc.childUser")
	childPass := s.RequiredVar("arc.childPassword")
	var cr *chrome.Chrome
	var err error

	accountType := s.Param().(string)
	if accountType == "normal" {
		cr, err = chrome.New(ctx, chrome.GAIALogin(),
			chrome.Auth(parentUser, parentPass, "gaia-id"),
			chrome.ARCSupported(),
			chrome.ExtraArgs(arc.DisableSyncFlags()...))
	} else {
		cr, err = chrome.New(ctx, chrome.GAIALogin(),
			chrome.Auth(childUser, childPass, "gaia-id"),
			chrome.ParentAuth(parentUser, parentPass), chrome.ARCSupported())
	}

	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

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

func turnOnPlayStore(ctx context.Context, tconn *chrome.TestConn) error {

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

	condition := func(ctx context.Context) (bool, error) {
		return ui.Exists(ctx, tconn, appParams)
	}

	opts := testing.PollOptions{Timeout: 10 * time.Second, Interval: 2 * time.Second}
	if err := appsbutton.LeftClickUntil(ctx, condition, &opts); err != nil {
		return errors.Wrap(err, "failed to click the Apps heading")
	}

	// Find the "Turn on" button and click.
	playStoreParams := ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "Google Play Store",
	}

	turnOnArc, err := ui.FindWithTimeout(ctx, tconn, playStoreParams, 30*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find Turn On")
	}
	defer turnOnArc.Release(ctx)

	if err := turnOnArc.FocusAndWait(ctx, 30*time.Second); err != nil {
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
	more, err := ui.FindWithTimeout(ctx, tconn, moreParams, 30*time.Second)
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
	accept, err := ui.FindWithTimeout(ctx, tconn, acceptParams, 30*time.Second)
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
