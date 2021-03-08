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
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
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
			Fixture:           "familyLinkParentArcLogin",
		}, {
			Name:              "unicorn",
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "familyLinkUnicornArcLogin",
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "familyLinkParentArcLogin",
		}, {
			Name:              "unicorn_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "familyLinkUnicornArcLogin",
		}},
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
	})
}

func EnableArc(ctx context.Context, s *testing.State) {

	tconn := s.FixtValue().(*familylink.FixtData).TestConn

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

	settings, err := ossettings.LaunchAtPage(ctx, tconn, nodewith.Name("Apps").Role(role.Heading))
	if err != nil {
		return errors.Wrap(err, "failed to Open Apps Settings Page")
	}

	ui := uiauto.New(tconn)
	playStoreButton := nodewith.Name("Google Play Store").Role(role.Button)
	if err := uiauto.Combine("enable Play Store",
		settings.FocusAndWait(playStoreButton),
		settings.LeftClick(playStoreButton),
		ui.LeftClick(nodewith.Name("More").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Accept").Role(role.Button)),
	)(ctx); err != nil {
		return err
	}

	if err := optin.WaitForPlayStoreReady(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for Play Store to be ready")
	}

	return nil

}
