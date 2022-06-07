// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LacrosMainProfileLogin,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Browser profile gets auto-created for the user, user is automatically logged into the profile",
		Contacts: []string{
			"anastasiian@chromium.org", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.LacrosPolicyLoggedInRealUser,
		Timeout:      2*chrome.LoginTimeout + time.Minute,
	})
}

func LacrosMainProfileLogin(ctx context.Context, s *testing.State) {
	profileMenu := nodewith.NameStartingWith("Accounts and sync").Role(role.Menu)

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// the button that should be pressed to accept of decline sync.
		syncButton *nodewith.Finder
		// the sync message that is expected to be shown at the end of the test.
		syncMessage *nodewith.Finder
	}{
		{
			name:        "no_policy_sync_on",
			syncButton:  nodewith.Name("Yes, I'm in").Role(role.Button),
			syncMessage: nodewith.Name("Sync is on").Role(role.StaticText).Ancestor(profileMenu),
		},
		{
			name:        "no_policy_sync_off",
			syncButton:  nodewith.Name("No thanks").Role(role.Button),
			syncMessage: nodewith.Name("Sync is off").Role(role.StaticText).Ancestor(profileMenu),
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Reserve 30 seconds for various cleanup.
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
			defer cancel()

			// Start chrome.
			cr, err := chrome.New(ctx, s.FixtValue().(*fixtures.PolicyRealUserFixtData).Opts()...)
			if err != nil {
				s.Fatal("Failed to start Chrome: ", err)
			}
			defer cr.Close(cleanupCtx)

			// Connect to Test API to use it with the UI library.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to connect Test API: ", err)
			}
			defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

			// Launch Lacros.
			lacros, err := lacros.Launch(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to launch lacros-chrome: ", err)
			}
			defer lacros.Close(cleanupCtx)

			ui := uiauto.New(tconn)

			welcomeButton := nodewith.Name("Let's go").Role(role.Button)
			if err := uiauto.Combine("accept or decline sync",
				ui.WaitUntilExists(welcomeButton),
				ui.LeftClickUntil(welcomeButton, ui.Exists(param.syncButton)),
				ui.LeftClick(param.syncButton),
			)(ctx); err != nil {
				s.Fatal("Failed to accept or decline sync: ", err)
			}

			profileToolbarButton := nodewith.ClassName("AvatarToolbarButton").Role(role.Button).Focusable()
			loggedInUserEmail := nodewith.Name(cr.User()).Role(role.StaticText).Ancestor(profileMenu)

			if err := uiauto.Combine("open the toolbar and check that the sync is on",
				ui.WaitUntilExists(profileToolbarButton),
				// Sync message may show an error in the beginning, but should change to 'sync is on/off'.
				ui.WithTimeout(time.Minute).LeftClickUntil(profileToolbarButton,
					uiauto.Combine("check that the user is logged in",
						ui.Exists(loggedInUserEmail),
						ui.Exists(param.syncMessage),
					),
				),
			)(ctx); err != nil {
				s.Fatal("Failed to check that the sync is on: ", err)
			}
		})
	}
}
