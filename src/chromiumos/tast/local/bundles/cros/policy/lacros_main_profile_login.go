// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
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
		Vars:         []string{lacrosfixt.LacrosDeployedBinary},
	})
}

func LacrosMainProfileLogin(ctx context.Context, s *testing.State) {
	// Reserve 10 seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Set up the browser.
	cr, l, _, err := lacros.Setup(ctx, s.FixtValue(), browser.TypeLacros)
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacros(cleanupCtx, l)

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	profileToolbarButton := nodewith.ClassName("AvatarToolbarButton").Role(role.Button).Focusable()
	profileMenu := nodewith.NameStartingWith("Accounts and sync").Role(role.Menu)
	loggedInUserEmail := nodewith.Name(cr.User()).Role(role.StaticText).Ancestor(profileMenu)
	syncIsOnMessage := nodewith.Name("Sync is on").Role(role.StaticText).Ancestor(profileMenu)

	if err := uiauto.Combine("open the toolbar and check that the user is logged in",
		ui.WaitUntilExists(profileToolbarButton),
		// Sync message may show an error in the beginning, but should change to 'sync is on'.
		ui.WithTimeout(time.Minute).LeftClickUntil(profileToolbarButton,
			uiauto.Combine("check that the user is logged in",
				ui.Exists(loggedInUserEmail),
				ui.Exists(syncIsOnMessage),
			),
		),
	)(ctx); err != nil {
		s.Fatal("Failed to check that the user is logged in: ", err)
	}
}
