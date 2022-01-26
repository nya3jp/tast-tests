// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/ctxutil"
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
		Func: PreventDefaultProfileRemoval,
		Desc: "Attempts to remove the default user profile in Lacros",
		Contacts: []string{
			"gflegar@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      fixture.LacrosPolicyLoggedIn,
	})
}

func PreventDefaultProfileRemoval(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	f := s.FixtValue().(lacrosfixt.FixtValue)

	// Open an empty Lacros window.
	l, err := lacros.Launch(ctx, f)
	if err != nil {
		s.Fatal("Failed to open lacros: ", err)
	}
	defer l.Close(ctx)

	// Dump the UI tree before we close lacros.
	defer func() {
		faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, f.Chrome(), "ui_tree")
		testing.Sleep(ctx, 5*time.Second)
		faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, f.Chrome(), "ui_tree2")
	}()

	// Start interacting with the UI
	ui := uiauto.New(f.TestAPIConn())
	buttonWith := nodewith.Role(role.Button).Focusable()

	// TODO(crbug/xxxx): The UI tree reports the location of some buttons wrongly.
	// This helper function offsets the click location to compensate for that.
	// Remove once the UI tree reports the locations correctly.
	leftClickWithOffset := func(finder *nodewith.Finder) func(context.Context) error {
		return func(ctx context.Context) error {
			if err := ui.MouseMoveTo(finder, 1*time.Second)(ctx); err != nil {
				return err
			}
			testing.Sleep(ctx, 1*time.Second)
			l, err := ui.Location(ctx, finder)
			if err != nil {
				return err
			}
			if err := ui.MouseClickAtLocation(0 /*leftClick*/, l.WithOffset(-50, -30).CenterPoint())(ctx); err != nil {
				return err
			}
			return nil
		}
	}

	if err := uiauto.Combine("open profile settings",
		ui.LeftClick(buttonWith.ClassName("AvatarToolbarButton")),
		ui.LeftClick(buttonWith.Name("Manage profiles")),
		// TODO(crbug/xxxx): Replace with ui.LeftClick once the UI tree reports the
		// location correctly.
		leftClickWithOffset(buttonWith.Name("More actions")),
	)(ctx); err != nil {
		s.Fatal("Failed to manipulate ui: ", err)
	}

	testing.Sleep(ctx, 5*time.Second)
	s.Fatal("I would like a UI dump")
}
