// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/bundles/cros/inputs/inputactions"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/imesettings"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type glideTypingTestParam struct {
	floatLayout bool
	inputMethod ime.InputMethod
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardGlideTyping,
		Desc:         "Test handwriting input functionality on virtual keyboard",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Attr:         []string{"group:mainline", "informational", "group:input-tools"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      time.Duration(3 * time.Minute),
		Params: []testing.Param{
			{
				Name: "tablet_docked",
				Pre:  pre.VKEnabledTabletReset,
				Val: glideTypingTestParam{
					floatLayout: false,
					inputMethod: ime.EnglishUS,
				},
			}, {
				Name: "tablet_floating",
				Pre:  pre.VKEnabledTabletReset,
				Val: glideTypingTestParam{
					floatLayout: true,
					inputMethod: ime.EnglishUSWithInternationalKeyboard,
				},
			}, {
				Name: "clamshell_a11y_docked",
				Pre:  pre.VKEnabledClamshellReset,
				Val: glideTypingTestParam{
					floatLayout: false,
					inputMethod: ime.Swedish,
				},
			},
			{
				Name: "clamshell_a11y_floating",
				Pre:  pre.VKEnabledClamshellReset,
				Val: glideTypingTestParam{
					floatLayout: true,
					inputMethod: ime.SpanishSpain,
				},
			},
		},
	})
}

func VirtualKeyboardGlideTyping(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn
	uc := s.PreValue().(pre.PreData).UserContext

	cleanupCtx := ctx
	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	inputMethod := s.Param().(glideTypingTestParam).inputMethod
	shouldFloatLayout := s.Param().(glideTypingTestParam).floatLayout

	if err := inputMethod.InstallAndActivate(tconn)(ctx); err != nil {
		s.Fatalf("Faield to install and activate input method %q: %v", inputMethod, err)
	}

	// Launch inputs test web server.
	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	// Select the input field being tested.
	vkbCtx := vkb.NewContext(cr, tconn)

	glideTypingWord, ok := data.TypingMessageHello.GetInputData(inputMethod)
	if !ok {
		s.Fatalf("Test Data for input method %v does not exist", inputMethod)
	}
	keySeq := glideTypingWord.CharacterKeySeq

	// Wrap a function to set float VK only once during the test.
	var isFloatLayout bool
	setFloatVK := func(ctx context.Context) error {
		if !isFloatLayout && shouldFloatLayout {
			isFloatLayout = true
			uc.RunAction(ctx, "switch VK to float mode", vkbCtx.SetFloatingMode(true), &useractions.UserActionCfg{
				ActionTags: []string{inputactions.ActionTagFloatVK},
				IfSuccessFunc: func(ctx context.Context) error {
					uc.SetAttribute(useractions.AttributeFloatVK, strconv.FormatBool(true))
					return nil
				},
			})
		}
		return nil
	}

	uc.AddTags([]string{inputactions.ActionTagVKGlideTyping})
	defer uc.RemoveTags([]string{inputactions.ActionTagVKGlideTyping})

	setGlideTyping := func(isEnabled bool) *useractions.UserAction {
		userActionName := "enable glide typing in OS settings"
		if !isEnabled {
			userActionName = "disable glide typing in OS settings"
		}

		return useractions.NewUserAction(
			userActionName,
			func(ctx context.Context) error {
				setting, err := imesettings.LaunchAtInputsSettingsPage(ctx, tconn, cr)
				if err != nil {
					return errors.Wrap(err, "failed to launch input settings")
				}
				return uiauto.Combine("change glide typing setting",
					setting.OpenInputMethodSetting(tconn, inputMethod),
					setting.ToggleGlideTyping(cr, isEnabled),
					setting.Close,
				)(ctx)
			},
			uc,
			nil,
		)
	}

	validateGlideTyping := func(actionDescription string, inputField testserver.InputField, isGlideTypingEnabled bool) *useractions.UserAction {
		return useractions.NewUserAction(
			actionDescription,
			uiauto.Combine(actionDescription,
				vkbCtx.HideVirtualKeyboard(),
				its.Clear(inputField),
				its.ClickFieldUntilVKShown(inputField),
				setFloatVK,
				util.GlideTyping(tconn, keySeq),
				// Check if input field text exactly matches glide typing result.
				// Select candidate from suggestion bar is also acceptable.
				func(ctx context.Context) error {
					if !isGlideTypingEnabled {
						// Should submit the last key if glide typing disabled.
						return its.ValidateResult(inputField, keySeq[len(keySeq)-1])(ctx)
					}
					if err := util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), glideTypingWord.ExpectedText)(ctx); err != nil {
						s.Log("Input field text does not match glide typing gesture")
						s.Log("Check if it is in suggestion bar")
						return uiauto.Combine("selecting candidate from suggestion bar",
							vkbCtx.SelectFromSuggestionIgnoringCase(glideTypingWord.ExpectedText),
							its.ValidateResult(inputField, glideTypingWord.ExpectedText),
						)(ctx)
					}
					return nil
				},
			),
			uc,
			&useractions.UserActionCfg{
				ActionAttributes: map[string]string{useractions.AttributeInputField: string(inputField)},
			},
		)
	}

	if err := validateGlideTyping(
		"glide typing should work in default settings",
		testserver.TextAreaInputField, true).Run(ctx); err != nil {
		s.Fatal("Failed to validate glide typing in default settings: ", err)
	}

	if err := validateGlideTyping("glide typing should not work on non-applicable fields",
		testserver.PasswordInputField, false).Run(ctx); err != nil {
		s.Fatal("Failed to validate glide typing on non-applicable fields: ", err)
	}

	// Disable glide typing in OS Settings.
	if err := setGlideTyping(false).Run(ctx); err != nil {
		s.Fatal("Failed to disable glide typing in OS settings: ", err)
	}

	// Glide typing should not work once disabled in setting.
	if err := validateGlideTyping("glide typing should not work once disabled",
		testserver.TextAreaInputField, false).Run(ctx); err != nil {
		s.Fatal("Failed to validate glide typing once disabled in setting: ", err)
	}

	// Re-enable glide typing in OS Settings.
	if err := setGlideTyping(true).Run(ctx); err != nil {
		s.Fatal("Failed to enable glide typing in OS settings: ", err)
	}

	if err := validateGlideTyping("glide typing should back work once enabled",
		testserver.TextAreaInputField, true).Run(ctx); err != nil {
		s.Fatal("Failed to validate glide typing after en-enabled in setting: ", err)
	}
}
