// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/inputs/faillog"
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
		Timeout:      5 * time.Minute,
	})
}

func VirtualKeyboardTypingApps(ctx context.Context, s *testing.State) {
	// typingKeys indicates a key series that tapped on virtual keyboard.
	var typingKeys = []string{"g", "o"}

	const expectedTypingResult = "go"

	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-virtual-keyboard"), chrome.ExtraArgs("--force-tablet-mode=touch_view"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s, tconn)

	app := apps.Settings
	s.Logf("Launching %s", app.Name)
	if err := apps.Launch(ctx, tconn, app.ID); err != nil {
		s.Fatalf("Failed to launch %s: %c", app.Name, err)
	}
	if err := ash.WaitForApp(ctx, tconn, app.ID); err != nil {
		s.Fatalf("%s did not appear in shelf after launch: %v", app.Name, err)
	}

	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for animation finished: ", err)
	}

	s.Log("Find searchbox input element")
	params := ui.FindParams{
		Role: ui.RoleTypeSearchBox,
		Name: "Search settings",
	}
	element, err := ui.FindWithTimeout(ctx, tconn, params, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to find searchbox input field in settings: ", err)
	}

	s.Log("Click searchbox to trigger virtual keyboard")
	if err := element.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click the input element: ", err)
	}

	s.Log("Wait for virtual keyboard shown up")
	if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for virtual keyboard shown up: ", err)
	}

	s.Log("Wait for decoder running")
	if err := vkb.WaitForDecoderEnabled(ctx, cr, true); err != nil {
		s.Fatal("Failed to wait for virtual keyboard shown up: ", err)
	}

	if err := vkb.TapKeys(ctx, tconn, typingKeys); err != nil {
		s.Fatal("Failed to input with virtual keyboard: ", err)
	}

	// Value change can be a bit delayed after input.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		inputValueElement, err := element.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeStaticText}, time.Second)
		if err != nil {
			return err
		}
		if inputValueElement.Name != expectedTypingResult {
			return errors.Errorf("failed to input with virtual keyboard. Got: %s; Want: %s", inputValueElement.Name, expectedTypingResult)
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Second}); err != nil {
		s.Error("Failed to input with virtual keyboard: ", err)
	}
}
