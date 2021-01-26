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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
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
	// TODO(b/178232263): This is a temporary work around to ensure Play Store closes.
	// Look into why apps.Close is failing with Play Store on occasion.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if visible, err := ash.AppShown(ctx, tconn, apps.PlayStore.ID); err != nil {
			return testing.PollBreak(err)
		} else if visible {
			apps.Close(ctx, tconn, apps.PlayStore.ID)
			return errors.New("app is not closed yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		s.Fatal("Failed to close Play Store: ", err)
	}

	s.Log("Turn Play Store Off from Settings")
	if err := turnOffPlayStore(ctx, tconn); err != nil {
		s.Fatal("Failed to Turn Off Play Store: ", err)
	}

	s.Log("Verify Play Store is Off")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		playStoreState, err := optin.GetPlayStoreState(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get some playstore state")
		}
		if playStoreState["enabled"] == true {
			return errors.New("Playstore is On Still")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed to verify Play Store is off: ", err)
	}

}

func turnOffPlayStore(ctx context.Context, tconn *chrome.TestConn) error {
	// Navigate to Android Settings.
	if err := apps.Launch(ctx, tconn, apps.Settings.ID); err != nil {
		return errors.Wrap(err, "failed to launch the Settings app")
	}

	ui := uiauto.New(tconn)
	playStoreButton := nodewith.Name("Google Play Store").Role(role.Button)
	return uiauto.Run(ctx,
		ui.LeftClickUntil(nodewith.Name("Apps").Role(role.Heading), ui.Exists(playStoreButton)),
		ui.FocusAndWait(playStoreButton),
		ui.LeftClick(playStoreButton),
		ui.LeftClick(nodewith.Name("Remove Google Play Store").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Remove Android apps").Role(role.Button)),
	)
}
