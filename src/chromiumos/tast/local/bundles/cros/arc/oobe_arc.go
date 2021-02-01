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
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OobeArc,
		Desc:         "Navigate through OOBE and Verify that PlayStore Settings Screen is launched at the end",
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

func OobeArc(ctx context.Context, s *testing.State) {

	username := s.RequiredVar("arc.username")
	password := s.RequiredVar("arc.password")

	cr, err := chrome.New(ctx, chrome.GAIALogin(), chrome.DontSkipOOBEAfterLogin(),
		chrome.ARCSupported(),
		chrome.Auth(username, password, "gaia-id"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	ui := uiauto.New(tconn)

	// To tap on Accept and continue/Got it whichever the OOBE flow displays.
	err = uiauto.Run(ctx, ui.LeftClick(nodewith.Name("Accept and continue").Role(role.Button)))
	if err != nil {
		s.Log("Failed to click Accept and continue : ", err)
		err = uiauto.Run(ctx, ui.LeftClick(nodewith.Name("Got it").Role(role.Button)))
		if err != nil {
			s.Fatal("Failed to click Got it : ", err)
		}
	}

	if err := uiauto.Run(ctx,
		ui.LeftClick(nodewith.Name("More").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Review Google Play options following setup").Role(role.CheckBox)),
		ui.LeftClick(nodewith.Name("Accept").Role(role.Button)),
		ui.LeftClick(nodewith.Name("No thanks").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Get started").Role(role.Button)),
	); err != nil {
		s.Fatal("Failed to smoke test the Files App: ", err)
	}

	s.Log("Verify Play Store is On")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		playStoreState, err := optin.GetPlayStoreState(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get some playstore state")
		}
		if playStoreState["enabled"] == false {
			return errors.New("Playstore is Off")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed to verify Play Store is On: ", err)
	}

	s.Log("Verify Play Store Settings is Launched")
	err = uiauto.Run(ctx, ui.WaitUntilExists(nodewith.Name("Remove Google Play Store").Role(role.Button)))
	if err != nil {
		s.Fatal("Failed to Launch Android Settings After OOBE Flow : ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

}
