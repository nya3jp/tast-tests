// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/ossettings"
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
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: chrome.LoginTimeout + arc.BootTimeout + 120*time.Second,
		Vars:    []string{"arc.username", "arc.password"},
	})
}

func EnableArc(ctx context.Context, s *testing.State) {

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

	// Launch Chrome OS Settings Apps Page.
	appsParam := ui.FindParams{
		Role: ui.RoleTypeHeading,
		Name: "Apps",
	}

	if err := ossettings.LaunchAtPage(
		ctx,
		tconn,
		appsParam,
	); err != nil {
		return errors.Wrap(err, "failed to Open Apps Settings Page")
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
