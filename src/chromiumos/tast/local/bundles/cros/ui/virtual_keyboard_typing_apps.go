// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/ui/faillog"
	"chromiumos/tast/local/bundles/cros/ui/pointer"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTypingApps,
		Desc:         "Checks that the virtual keyboard works in apps",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Pre:          chrome.VKEnabled(),
		Timeout:      5 * time.Minute,
	})
}

func VirtualKeyboardTypingApps(ctx context.Context, s *testing.State) {
	// typingKeys indicates a key series that tapped on virtual keyboard.
	var typingKeys = []string{"h", "e", "l", "l", "o", "space", "t", "a", "s", "t"}

	const expectedTypingResult = "hello tast"

	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s, tconn)

	// Virtual keyboard is mostly used in tablet mode.
	s.Log("Setting device to tablet mode")
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure tablet mode enabled: ", err)
	}
	defer cleanup(ctx)

	// Create a touch controller.
	// Use pc tap event to trigger virtual keyboard instead of calling vkb.ShowVirtualKeyboard()
	pc, err := pointer.NewTouchController(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create a touch controller")
	}
	defer pc.Close()

	app := apps.Settings
	s.Logf("Launching %s", app.Name)
	if err := apps.Launch(ctx, tconn, app.ID); err != nil {
		s.Fatalf("Failed to launch %s: %c", app.Name, err)
	}
	if err := ash.WaitForApp(ctx, tconn, app.ID); err != nil {
		s.Fatalf("%s did not appear in shelf after launch: %v", app.Name, err)
	}

	s.Log("Find searchbox input element")
	params := ui.FindParams{
		Role: ui.RoleTypeSearchBox,
		Name: "Search settings",
	}
	element, err := ui.FindWithTimeout(ctx, tconn, params, 3*time.Second)
	if err != nil {
		s.Fatal("Failed to find searchbox input field in settings: ", err)
	}

	s.Log("Click searchbox to trigger virtual keyboard")
	if err := pointer.Click(ctx, pc, element.Location.CenterPoint()); err != nil {
		s.Fatal("Failed to click the input element: ", err)
	}

	s.Log("Input with virtual keyboard")
	if err := vkb.InputWithVirtualKeyboard(ctx, tconn, typingKeys); err != nil {
		s.Fatal("Failed to type on virtual keyboard: ", err)
	}

	inputValueElement, err := element.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeStaticText}, time.Second)
	if err != nil {
		s.Fatal("Failed to find searchbox value element: ", err)
	}

	if inputValueElement.Name != expectedTypingResult {
		s.Fatalf("Failed to input with virtual keyboard. Got: %s; Want: %s", inputValueElement.Name, expectedTypingResult)
	}
}
