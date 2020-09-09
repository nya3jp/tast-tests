// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTypingApps,
		Desc:         "Checks that the virtual keyboard works in apps",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:essential-inputs"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Pre:          pre.VKEnabled(),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name:              "stable",
			ExtraHardwareDeps: pre.InputsStableModels,
		}, {
			Name:              "unstable",
			ExtraHardwareDeps: pre.InputsUnstableModels,
			ExtraAttr:         []string{"informational"},
		}}})
}

func VirtualKeyboardTypingApps(ctx context.Context, s *testing.State) {
	// typingKeys indicates a key series that tapped on virtual keyboard.
	const typingKeys = "go"

	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(ctx)

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

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
	searchInputElement, err := ui.FindWithTimeout(ctx, tconn, params, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to find searchbox input field in settings: ", err)
	}
	defer searchInputElement.Release(ctx)

	s.Log("Click searchbox to trigger virtual keyboard")
	if err := searchInputElement.LeftClick(ctx); err != nil {
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

	if err := vkb.TapKeys(ctx, tconn, strings.Split(typingKeys, "")); err != nil {
		s.Fatal("Failed to input with virtual keyboard: ", err)
	}

	// Hide virtual keyboard to submit candidate
	if err := vkb.HideVirtualKeyboard(ctx, tconn); err != nil {
		s.Fatal("Failed to hide virtual keyboard: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := searchInputElement.Update(ctx); err != nil {
			return errors.Wrap(err, "failed to update the node's location")
		}

		if searchInputElement.Value != typingKeys {
			return errors.Errorf("failed to input with virtual keyboard. Got: %s; Want: %s", searchInputElement.Value, typingKeys)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Error("Failed to input with virtual keyboard: ", err)
	}
}
