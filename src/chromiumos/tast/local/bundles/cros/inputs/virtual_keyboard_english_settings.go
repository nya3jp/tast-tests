// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardEnglishSettings,
		Desc:         "Checks that the input settings works in Chrome",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Pre:          pre.VKEnabledTablet,
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
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Revert settings to default after testing.
	defer func() {
		if err := tconn.Eval(cleanupCtx, `chrome.inputMethodPrivate.setSettings(
			"xkb:us::eng",
			{"virtualKeyboardEnableCapitalization": false,
			"virtualKeyboardAutoCorrectionLevel": 1})`, nil); err != nil {
			s.Log("Failed to revert language settings")
		}
	}()

	its, err := testserver.Launch(ctx, cr)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	// Use text input field as testing target.
	inputField := testserver.TextInputField

	type testData struct {
		name                  string
		capitalizationEnabled bool
		keySeq                []string
		expectedText          string
	}

	subTests := []testData{
		{
			name:                  "capitalizationEnabled",
			capitalizationEnabled: true,
			keySeq:                strings.Split("Hello", ""),
			expectedText:          "Hello",
		}, {
			name:                  "capitalizationDisabled",
			capitalizationEnabled: false,
			keySeq:                strings.Split("hello", ""),
			expectedText:          "hello",
		},
	}

	for _, subTest := range subTests {
		s.Run(ctx, subTest.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

			// TODO(b/172498469): Change settings via Chrome OS settings page after we migrate settings to there.
			if err := tconn.Eval(ctx, fmt.Sprintf(`chrome.inputMethodPrivate.setSettings(
				"xkb:us::eng",
				{"virtualKeyboardEnableCapitalization": %t,
				"virtualKeyboardAutoCorrectionLevel": 1})`, subTest.capitalizationEnabled), nil); err != nil {
				s.Fatal("Failed to set settings: ", err)
			}

			s.Log("Wait for decoder running")
			if err := vkb.WaitForDecoderEnabled(ctx, cr, true); err != nil {
				s.Fatal("Failed to wait for virtual keyboard shown up: ", err)
			}

			// Clear text before starting input.
			if err := its.Clear(inputField)(ctx); err != nil {
				s.Fatal("Failed to clear input field: ", err)
			}

			if err := inputField.ClickUntilVKShown(ctx, tconn); err != nil {
				s.Fatal("Failed to click input field to show virtual keyboard: ", err)
			}
			defer vkb.HideVirtualKeyboard(ctx, tconn)

			if err := vkb.TapKeys(ctx, tconn, subTest.keySeq); err != nil {
				s.Fatal("Failed to input with virtual keyboard: ", err)
			}

			if err := inputField.WaitForValueToBe(ctx, tconn, subTest.expectedText); err != nil {
				s.Fatal("Failed to verify input: ", err)
			}
		})
	}
}
