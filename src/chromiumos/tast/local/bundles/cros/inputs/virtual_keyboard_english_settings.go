// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardEnglishSettings,
		Desc:         "Checks that the input settings works in Chrome",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:essential-inputs", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Pre:          pre.VKEnabledTablet(),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name:              "stable",
			ExtraHardwareDeps: pre.InputsStableModels,
		}, {
			Name:              "unstable",
			ExtraHardwareDeps: pre.InputsUnstableModels,
		}},
	})
}

func VirtualKeyboardEnglishSettings(ctx context.Context, s *testing.State) {
	var typingKeys = []string{"space", "g", "o", "."}
	const expectedTypingResult = " Go. go."

	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	s.Log("Start a local server to test chrome")
	its, err := testserver.Launch(ctx, cr)
	if err != nil {
		s.Fatal("Fail to launch inputs test server: ", err)
	}
	defer its.Close()

	inputField := testserver.TextAreaInputField

	s.Log("Wait for decoder running")
	if err := vkb.WaitForDecoderEnabled(ctx, cr, true); err != nil {
		s.Fatal("Failed to wait for virtual keyboard shown up: ", err)
	}

	// TODO(jiwan): Change settings via Chrome OS settings page after we migrate settings to there.
	if err := tconn.Eval(ctx, `chrome.inputMethodPrivate.setSettings(
		"xkb:us::eng",
		{"virtualKeyboardEnableCapitalization": true,
		"virtualKeyboardAutoCorrectionLevel": 1})`, nil); err != nil {
		s.Fatal("Failed to set settings: ", err)
	}

	s.Log("Click input to trigger virtual keyboard")
	if err := inputField.ClickUntilVKShown(ctx, tconn); err != nil {
		s.Fatal("Failed to click input field to show virtual keyboard: ", err)
	}

	if err := vkb.TapKeys(ctx, tconn, typingKeys); err != nil {
		s.Fatal("Failed to input with virtual keyboard: ", err)
	}

	if err := tconn.Eval(ctx, `chrome.inputMethodPrivate.setSettings(
		"xkb:us::eng",
		{"virtualKeyboardEnableCapitalization": false,
		"virtualKeyboardAutoCorrectionLevel": 1})`, nil); err != nil {
		s.Fatal("Failed to set settings: ", err)
	}

	// Hide and Trigger VK again to update settings.
	if err := vkb.HideVirtualKeyboard(ctx, tconn); err != nil {
		s.Fatal("Failed to hide virtual keyboard: ", err)
	}
	if err := inputField.ClickUntilVKShown(ctx, tconn); err != nil {
		s.Fatal("Failed to click input field to show virtual keyboard: ", err)
	}

	if err := vkb.TapKeys(ctx, tconn, typingKeys); err != nil {
		s.Fatal("Failed to input with virtual keyboard: ", err)
	}

	if err := inputField.WaitForValueToBe(ctx, tconn, expectedTypingResult); err != nil {
		s.Fatal("Failed to verify input: ", err)
	}
}
