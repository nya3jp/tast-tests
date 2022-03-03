// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/imesettings"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardKoreanTyping,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that physical keyboard can perform basic typing in korean",
		Contacts:     []string{"jopalmer@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "group:input-tools-upstream"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Pre:          pre.NonVKClamshell,
		Timeout:      5 * time.Minute,
	})
}

type koreanKeyboardLayout string

const (
	koreanInputType2Set        koreanKeyboardLayout = "2 Set / 두벌식"
	koreanInputType3Set390     koreanKeyboardLayout = "3 Set (390) / 세벌식 (390)"
	koreanInputType3SetFinal   koreanKeyboardLayout = "3 Set (Final) / 세벌식 (최종)"
	koreanInputType3SetNoShift koreanKeyboardLayout = "3 Set (No Shift) / 세벌식 (순아래)"
)

func PhysicalKeyboardKoreanTyping(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn
	uc := s.PreValue().(pre.PreData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	ui := uiauto.New(tconn)

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Add IME for testing.
	if err := ime.Korean.InstallAndActivate(tconn)(ctx); err != nil {
		s.Fatal("Failed to switch to Korean IME")
	}
	uc.SetAttribute(useractions.AttributeInputMethod, ime.Korean.Name)

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Fail to launch inputs test server: ", err)
	}
	defer its.Close()

	var subtests = []struct {
		testName       string
		keyboardLayout koreanKeyboardLayout // layout should match the name in IME setting.
		inputFunc      uiauto.Action
		expectedText   string
	}{
		{
			// Note: this only works because underlying layout is the US (qwerty) xkb
			// layout. That may change in the future (ref b/199024864).
			testName:       "2 set",
			keyboardLayout: koreanInputType2Set,
			inputFunc:      keyboard.TypeAction("gks"),
			expectedText:   "한",
		},
		{
			// Note: Options other than 2 set are supported at low priority. In fact,
			// these examples may not be even correct, but these tests will still detect
			// any change in behavior.
			testName:       "3 set 390 (1)",
			keyboardLayout: koreanInputType3Set390,
			inputFunc:      keyboard.TypeAction("kR"),
			expectedText:   "걔",
		},
		{
			// Note: Options other than 2 set are supported at low priority. In fact,
			// these examples may not be even correct, but these tests will still detect
			// any change in behavior.
			testName:       "3 set 390 (2)",
			keyboardLayout: koreanInputType3Set390,
			inputFunc:      keyboard.TypeAction("jfs1"),
			expectedText:   "않",
		},
		{
			// Note: Options other than 2 set are supported at low priority. In fact,
			// these examples may not be even correct, but these tests will still detect
			// any change in behavior.
			testName:       "3 set final (1)",
			keyboardLayout: koreanInputType3SetFinal,
			inputFunc:      keyboard.TypeAction("kG"),
			expectedText:   "걔",
		},
		{
			// Note: Options other than 2 set are supported at low priority. In fact,
			// these examples may not be even correct, but these tests will still detect
			// any change in behavior.
			testName:       "3 set final (2)",
			keyboardLayout: koreanInputType3SetFinal,
			inputFunc:      keyboard.TypeAction("ifS"),
			expectedText:   "많",
		},
		{
			// Note: Options other than 2 set are supported at low priority. In fact,
			// these examples may not be even correct, but these tests will still detect
			// any change in behavior.
			testName:       "3 set No shift (1)",
			keyboardLayout: koreanInputType3SetNoShift,
			inputFunc:      keyboard.TypeAction("kR"),
			expectedText:   "개",
		},
		{
			// Note: Options other than 2 set are supported at low priority. In fact,
			// these examples may not be even correct, but these tests will still detect
			// any change in behavior.
			testName:       "3 set No shift (2)",
			keyboardLayout: koreanInputType3SetNoShift,
			inputFunc:      keyboard.TypeAction("jfs1"),
			expectedText:   "않",
		},
		{
			testName:       "ENTER key to submit",
			keyboardLayout: koreanInputType2Set,
			inputFunc: uiauto.Combine("type Korean and press enter",
				keyboard.TypeAction("gks"),
				keyboard.AccelAction("Enter"),
			),
			expectedText: "한\n",
		},
	}

	currentKeyboardLayout := "unknown"
	var inputField = testserver.TextAreaInputField
	for _, subtest := range subtests {
		s.Run(ctx, subtest.testName, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+subtest.testName)

			// Change layout in IME settings only if required.
			if currentKeyboardLayout != string(subtest.keyboardLayout) {
				if err := imesettings.SetKoreanKeyboardLayout(uc, string(subtest.keyboardLayout))(ctx); err != nil {
					s.Fatalf("Failed to set keyboard layout to %q: %v", subtest.keyboardLayout, err)
				}
				currentKeyboardLayout = string(subtest.keyboardLayout)
			}

			if err := uiauto.UserAction(
				"Korean PK input",
				its.ValidateInputOnField(inputField, subtest.inputFunc, subtest.expectedText),
				uc, &useractions.UserActionCfg{
					Attributes: map[string]string{
						useractions.AttributeTestScenario: subtest.testName,
						useractions.AttributeInputField:   string(inputField),
						useractions.AttributeFeature:      useractions.FeaturePKTyping,
					},
				},
			)(ctx); err != nil {
				s.Fatalf("Failed to validate keys input in %s: %v", inputField, err)
			}
		})
	}

	testName := "ENTER key on Omnibox"
	s.Run(ctx, testName, func(ctx context.Context, s *testing.State) {
		defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+testName)

		// Change layout in IME settings only if required.
		if currentKeyboardLayout != string(koreanInputType2Set) {
			if err := imesettings.SetKoreanKeyboardLayout(uc, string(koreanInputType2Set))(ctx); err != nil {
				s.Fatalf("Failed to set keyboard layout to %q: %v", koreanInputType2Set, err)
			}
		}

		omniboxFinder := nodewith.HasClass("OmniboxViewViews")
		validateOmniboxAction := uiauto.Combine("verify enter key on omnibox",
			ui.LeftClick(omniboxFinder),
			keyboard.TypeAction("gks"),
			keyboard.AccelAction("Enter"),
			util.WaitForFieldTextToSatisfy(tconn, omniboxFinder, "google URL", func(text string) bool {
				return strings.Contains(text, "google.com")
			}),
		)

		if err := uiauto.UserAction(
			"Korean PK input",
			validateOmniboxAction,
			uc, &useractions.UserActionCfg{
				Attributes: map[string]string{
					useractions.AttributeTestScenario: testName,
					useractions.AttributeInputField:   "Omnibox",
					useractions.AttributeFeature:      useractions.FeaturePKTyping,
				},
			},
		)(ctx); err != nil {
			s.Fatal("Failed to validate korean PK input in omnibox: ", err)
		}
	})
}
