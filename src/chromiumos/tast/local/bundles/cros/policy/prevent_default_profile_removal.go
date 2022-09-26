// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PreventDefaultProfileRemoval,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Attempts to remove the default user profile in Lacros",
		Contacts: []string{
			"eariassoto@google.com", // Test author
		},
		// TODO(b/229003599): This test is failing on tablets and Chromeboxes.
		// We are disabling until the root cause is fixed.
		// Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      fixture.LacrosPolicyLoggedIn,
	})
}

func PreventDefaultProfileRemoval(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Open an empty Lacros window.
	l, err := lacros.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open lacros: ", err)
	}
	defer l.Close(ctx)

	// Dump the UI tree before we close lacros.
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	// Start interacting with the UI
	ui := uiauto.New(tconn)
	buttonWith := nodewith.Role(role.Button).Focusable()

	if err := uiauto.Combine("open profile settings",
		ui.LeftClick(buttonWith.ClassName("AvatarToolbarButton")),
		ui.LeftClick(buttonWith.Name("Manage profiles")),
		ui.LeftClick(buttonWith.Name("More actions")),
		ui.LeftClick(nodewith.Role(role.MenuItem).Focusable().ClassName("dropdown-item").Name("Delete")),
	)(ctx); err != nil {
		s.Fatal("Failed to manipulate ui: ", err)
	}

	if err := ui.WaitUntilExists(nodewith.Role(role.Dialog).Name("Can't delete this profile"))(ctx); err != nil {
		s.Fatal("Expected error dialog that the profile cannot be deleted: ", err)
	}
}
