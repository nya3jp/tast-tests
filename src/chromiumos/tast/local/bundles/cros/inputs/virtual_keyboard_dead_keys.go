// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

// deadKeysTestCase struct encapsulates parameters for each Dead Keys test.
type deadKeysTestCase struct {
	inputMethodID        string
	hasDecoder           bool
	useA11yVk            bool
	typingKeys           []string
	expectedTypingResult string
}

// Combining diacritic Unicode characters used as key caps of VK dead keys.
const (
	acuteAccent = "\u0301"
	circumflex  = "\u0302"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardDeadKeys,
		Desc:         "Checks that dead keys on the virtual keyboard work",
		Contacts:     []string{"tranbaoduy@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name:              "french_stable",
				ExtraHardwareDeps: pre.InputsStableModels,
				ExtraAttr:         []string{"group:input-tools-upstream"},
				// "French - French keyboard" input method uses a compact-layout VK for
				// non-a11y mode where there's no dead keys, and a full-layout VK for
				// a11y mode where there's dead keys. To test dead keys on the VK of
				// this input method, a11y mode must be enabled.
				Pre: pre.VKEnabledClamshell,
				Val: deadKeysTestCase{
					// "French - French keyboard" input method is decoder-backed. Dead keys
					// are implemented differently from those of a no-frills input method.
					inputMethodID: "xkb:fr::fra",
					hasDecoder:    true,
					// TODO(b/162292283): Make vkb.TapKeys() less flaky when the VK changes
					// based on Shift and Caps states, then add Shift and Caps related
					// typing sequences to the test case.
					typingKeys:           []string{circumflex, "a"},
					expectedTypingResult: "â",
				},
			}, {
				Name:              "french_unstable",
				ExtraHardwareDeps: pre.InputsUnstableModels,
				ExtraAttr:         []string{"informational"},
				Pre:               pre.VKEnabledClamshell,
				Val: deadKeysTestCase{
					inputMethodID:        "xkb:fr::fra",
					hasDecoder:           true,
					typingKeys:           []string{circumflex, "a"},
					expectedTypingResult: "â",
				},
			}, {
				Name:              "french_exp",
				Pre:               pre.VKEnabledClamshellExp,
				ExtraSoftwareDeps: []string{"gboard_decoder"},
				ExtraAttr:         []string{"informational", "group:input-tools-upstream"},
				Val: deadKeysTestCase{
					inputMethodID:        "xkb:fr::fra",
					hasDecoder:           true,
					typingKeys:           []string{circumflex, "a"},
					expectedTypingResult: "â",
				},
			}, {
				Name:              "catalan_stable",
				ExtraHardwareDeps: pre.InputsStableModels,
				ExtraAttr:         []string{"group:input-tools-upstream"},
				// "Catalan keyboard" input method uses the same full-layout VK (that
				// has dead keys) for both a11y & non-a11y. Just use non-a11y here.
				Pre: pre.VKEnabledTablet,
				Val: deadKeysTestCase{
					// "Catalan keyboard" input method is no-frills. Dead keys are
					// implemented differently from those of a decoder-backed input method.
					inputMethodID: "xkb:es:cat:cat",
					hasDecoder:    false,

					// TODO(b/162292283): Make vkb.TapKeys() less flaky when the VK changes
					// based on Shift and Caps states, then add Shift and Caps related
					// typing sequences to the test case.
					typingKeys:           []string{acuteAccent, "a"},
					expectedTypingResult: "á",
				},
			}, {
				Name:              "catalan_unstable",
				ExtraHardwareDeps: pre.InputsUnstableModels,
				ExtraAttr:         []string{"informational"},
				Pre:               pre.VKEnabledTablet,
				Val: deadKeysTestCase{
					inputMethodID:        "xkb:es:cat:cat",
					hasDecoder:           false,
					typingKeys:           []string{acuteAccent, "a"},
					expectedTypingResult: "á",
				},
			}, {
				Name:              "catalan_exp",
				ExtraSoftwareDeps: []string{"gboard_decoder"},
				ExtraAttr:         []string{"informational", "group:input-tools-upstream"},
				Pre:               pre.VKEnabledTabletExp,
				Val: deadKeysTestCase{
					inputMethodID:        "xkb:es:cat:cat",
					hasDecoder:           false,
					typingKeys:           []string{acuteAccent, "a"},
					expectedTypingResult: "á",
				},
			},
		}})
}

func VirtualKeyboardDeadKeys(ctx context.Context, s *testing.State) {
	testCase := s.Param().(deadKeysTestCase)

	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	its, err := testserver.Launch(ctx, cr)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	s.Log("Set input method to: ", testCase.inputMethodID)
	if err := ime.AddAndSetInputMethod(ctx, tconn, ime.IMEPrefix+testCase.inputMethodID); err != nil {
		s.Fatalf("Failed to set input method to %q: %v", testCase.inputMethodID, err)
	}

	inputField := testserver.TextAreaNoCorrectionInputField

	if err := inputField.ClickUntilVKShown(ctx, tconn); err != nil {
		s.Fatal("Failed to click input field to show virtual keyboard: ", err)
	}

	if testCase.hasDecoder {
		s.Log("Wait for decoder running")
		if err := vkb.WaitForDecoderEnabled(ctx, cr, true); err != nil {
			s.Fatal("Failed to wait for decoder running: ", err)
		}
	}

	if err := vkb.TapKeys(ctx, tconn, testCase.typingKeys); err != nil {
		s.Fatal("Failed to input with virtual keyboard: ", err)
	}

	if err := inputField.WaitForValueToBe(ctx, tconn, testCase.expectedTypingResult); err != nil {
		s.Fatal("Failed to verify input: ", err)
	}
}
