// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardEnglishTyping,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that physical keyboard can perform basic typing",
		Contacts:     []string{"shend@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Pre:       pre.NonVKClamshell,
				ExtraAttr: []string{"group:input-tools-upstream"},
			},
			{
				Name:      "fixture",
				Fixture:   fixture.ClamshellNonVK,
				ExtraAttr: []string{"informational"},
			},
		},
	})
}

func PhysicalKeyboardEnglishTyping(ctx context.Context, s *testing.State) {
	var cr *chrome.Chrome
	var tconn *chrome.TestConn
	var uc *useractions.UserContext
	if strings.Contains(s.TestName(), "fixture") {
		cr = s.FixtValue().(fixture.FixtData).Chrome
		tconn = s.FixtValue().(fixture.FixtData).TestAPIConn
		uc = s.FixtValue().(fixture.FixtData).UserContext
		uc.SetTestName(s.TestName())
	} else {
		cr = s.PreValue().(pre.PreData).Chrome
		tconn = s.PreValue().(pre.PreData).TestAPIConn
		uc = s.PreValue().(pre.PreData).UserContext
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Add IME for testing.
	im := ime.EnglishUS

	s.Log("Set current input method to: ", im)
	if err := im.InstallAndActivate(tconn)(ctx); err != nil {
		s.Fatalf("Failed to set input method to %v: %v: ", im, err)
	}
	uc.SetAttribute(useractions.AttributeInputMethod, im.Name)

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

	var subtests = []util.InputEval{
		{
			TestName:     "Mixed (alphanumeric, symbols, enter) typing",
			InputFunc:    keyboard.TypeAction("Hello!\nTesting 123."),
			ExpectedText: "Hello!\nTesting 123.",
		}, {
			TestName: "Backspace to delete",
			InputFunc: uiauto.Combine("type a string and Backspace",
				keyboard.TypeAction("abc"),
				keyboard.AccelAction("Backspace"),
			),
			ExpectedText: "ab",
		}, {
			TestName: "Ctrl+Backspace",
			InputFunc: uiauto.Combine("type a string and Ctrl+Backspace",
				keyboard.TypeAction("hello world"),
				keyboard.AccelAction("Ctrl+Backspace"),
			),
			ExpectedText: "hello ",
		}, {
			TestName: "Editing middle of text",
			InputFunc: uiauto.Combine("type strings and edit in the middle of text",
				keyboard.TypeAction("abc"),
				keyboard.AccelAction("Left"),
				keyboard.AccelAction("Backspace"),
				keyboard.TypeAction("bc ab"),
			),
			ExpectedText: "abc abc",
		},
	}

	var inputField = testserver.TextAreaInputField

	for _, subtest := range subtests {
		s.Run(ctx, subtest.TestName, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+subtest.TestName)

			if err := uiauto.UserAction(
				"Engish PK input",
				its.ValidateInputOnField(inputField, subtest.InputFunc, subtest.ExpectedText),
				uc, &useractions.UserActionCfg{
					Attributes: map[string]string{
						useractions.AttributeTestScenario: subtest.TestName,
						useractions.AttributeInputField:   string(inputField),
						useractions.AttributeFeature:      useractions.FeaturePKTyping,
					},
				},
			)(ctx); err != nil {
				s.Fatalf("Failed to validate keys input in %s: %v", inputField, err)
			}

			if err := its.ValidateInputOnField(inputField, subtest.InputFunc, subtest.ExpectedText)(ctx); err != nil {
				s.Fatalf("Failed to validate %s: %v", subtest.TestName, err)
			}
		})
	}
}
