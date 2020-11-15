// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardQuickEmoji,
		Desc:         "Checks that right click input field and select emoji will trigger virtual keyboard",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "group:input-tools-upstream"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Pre:          chrome.LoggedIn(),
		Params: []testing.Param{{
			Name:              "stable",
			ExtraHardwareDeps: hwdep.D(hwdep.Model(pre.StableModels...), hwdep.SkipOnModel("kodama", "kefka")),
		}, {
			Name:              "unstable",
			ExtraHardwareDeps: pre.InputsUnstableModels,
			ExtraAttr:         []string{"informational"},
		}}})
}

func VirtualKeyboardQuickEmoji(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	its, err := testserver.Launch(ctx, cr)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	inputField := testserver.TextInputField

	inputFieldNode, err := inputField.GetNode(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find input field: ", err)
	}
	defer inputFieldNode.Release(ctx)

	if err := inputFieldNode.RightClick(ctx); err != nil {
		s.Fatal("Failed to right click the input element: ", err)
	}

	emojiMenuElement, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: "Emoji"}, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to find Emoji menu item: ", err)
	}
	defer emojiMenuElement.Release(ctx)

	if err := emojiMenuElement.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click the input element: ", err)
	}

	if _, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: "emoji keyboard shown"}, 20*time.Second); err != nil {
		s.Fatal("Failed to wait for emoji panel shown: ", err)
	}

	if err := vkb.WaitLocationStable(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for virtual keyboard shown: ", err)
	}

	const emojiChar = "😂"
	if err := vkb.TapKey(ctx, tconn, emojiChar); err != nil {
		s.Fatalf("Failed to tap key %s: %v", emojiChar, err)
	}

	if err := inputField.WaitForValueToBe(ctx, tconn, emojiChar); err != nil {
		s.Fatal("Failed to verify input: ", err)
	}

	// Hide virtual keyboard and click input field again should not trigger vk.
	if err := vkb.HideVirtualKeyboard(ctx, tconn); err != nil {
		s.Fatal("Failed to hide virtual keyboard: ", err)
	}

	// Verify virtual keyboard status is not persisted on clamshell.
	// Should not test it on tablet devices. It depends on the physical conditions of the running device.
	// For more details, refer to b/169527206.
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}

	if !tabletModeEnabled {
		if err := inputField.Click(ctx, tconn); err != nil {
			s.Fatal("Failed to click the input element: ", err)
		}

		// Check virtual keyboard is not shown in the following 10 seconds.
		testing.Poll(ctx, func(pollCtx context.Context) error {
			// Note: do not use internal pollCtx but use the external context, as the
			// last iteration it may hit the context deadline exceeded error.
			if isVKShown, err := vkb.IsShown(ctx, tconn); err != nil {
				s.Fatal("Failed to check vk visibility: ", err)
			} else if isVKShown {
				s.Fatal("Virtual keyboard is still enabled after quick emoji input")
			}
			return errors.New("continuously check until timeout")
		}, &testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: 10 * time.Second})
	}
}
