// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardQuickEmoji,
		Desc:         "Checks that right click input field and select emoji will trigger virtual keyboard",
		Contacts:     []string{"jopalmer@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.SystemEmojiPicker,
		Params: []testing.Param{{
			Name:              "stable",
			ExtraAttr:         []string{"group:input-tools-upstream"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model(pre.StableModels...), hwdep.SkipOnModel("kodama", "kefka")),
		}, {
			Name:              "unstable",
			ExtraHardwareDeps: pre.InputsUnstableModels,
		}}})
}

func VirtualKeyboardQuickEmoji(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome

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

	emojiMenuElement, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: "Emoji Search+Shift+Space"}, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to find Emoji menu item: ", err)
	}
	defer emojiMenuElement.Release(ctx)

	if err := emojiMenuElement.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click the input element: ", err)
	}

	if _, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: "Emoji Picker"}, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for emoji panel shown: ", err)
	}

	const emojiChar = "😂"
	emojiButton, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: emojiChar}, 20*time.Second)
	if err != nil {
		s.Fatalf("Failed to find emoji button %s: %v", emojiChar, err)
	}
	if err := emojiButton.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click on the emoji button")
	}

	if err := inputField.WaitForValueToBe(ctx, tconn, emojiChar); err != nil {
		s.Fatal("Failed to verify input: ", err)
	}
}
