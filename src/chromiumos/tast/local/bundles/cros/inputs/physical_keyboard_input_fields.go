// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
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
		Func:         PhysicalKeyboardInputFields,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that physical keyboard works on different input fields",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Timeout:      5 * time.Minute,
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS}),
		Params: []testing.Param{
			{
				Name:      "us_en",
				Fixture:   fixture.ClamshellNonVK,
				ExtraAttr: []string{"group:input-tools-upstream"},
				Val:       ime.EnglishUS,
			},
			{
				Name:              "us_en_lacros",
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraAttr:         []string{"informational"},
				Val:               ime.EnglishUS,
				ExtraSoftwareDeps: []string{"lacros_stable"},
			},
		},
	})
}

func PhysicalKeyboardInputFields(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Add IME for testing.
	im := s.Param().(ime.InputMethod)

	s.Log("Set current input method to: ", im)
	if err := im.InstallAndActivateUserAction(uc)(ctx); err != nil {
		s.Fatalf("Failed to set input method to %v: %v: ", im, err)
	}
	uc.SetAttribute(useractions.AttributeInputMethod, im.Name)

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

	var subtests []testserver.FieldInputEval

	switch im {
	case ime.EnglishUS:
		subtests = []testserver.FieldInputEval{
			{
				InputField:   testserver.TextAreaInputField,
				InputFunc:    keyboard.TypeAction(`1234567890-=!@#$%^&*()_+[]{};'\:"|,./<>?~`),
				ExpectedText: `1234567890-=!@#$%^&*()_+[]{};'\:"|,./<>?~`,
			}, {
				InputField:   testserver.TextInputField,
				InputFunc:    keyboard.TypeAction("qwertyuiopasdfghjklzxcvbnmQWERTYUIOPASDFGHJKLZXCVBNM"),
				ExpectedText: "qwertyuiopasdfghjklzxcvbnmQWERTYUIOPASDFGHJKLZXCVBNM",
			},
		}
		break
	default:
		s.Fatalf("%s is not supported", im)
	}

	for _, subtest := range subtests {
		s.Run(ctx, string(subtest.InputField), func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+string(subtest.InputField))
			inputField := subtest.InputField

			if err := uiauto.UserAction(
				"Verify PK keys are functional",
				its.ValidateInputOnField(inputField, subtest.InputFunc, subtest.ExpectedText),
				uc, &useractions.UserActionCfg{
					Attributes: map[string]string{
						useractions.AttributeInputField: string(inputField),
						useractions.AttributeFeature:    useractions.FeaturePKTyping,
					},
				},
			)(ctx); err != nil {
				s.Fatalf("Failed to validate keys input in %s: %v", inputField, err)
			}
		})
	}
}
