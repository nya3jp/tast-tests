// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
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
		Func:         PhysicalKeyboardNumpad,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that numpad keys work",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS}),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Fixture: fixture.ClamshellNonVK,
			},
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
			},
		},
	})
}

func PhysicalKeyboardNumpad(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

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

	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	inputField := testserver.TextAreaInputField

	// Mapping of runes to event codes for common numpad keys.
	numpadKeyMapping := map[rune]input.EventCode{
		'0':  input.KEY_KP0,
		'1':  input.KEY_KP1,
		'2':  input.KEY_KP2,
		'3':  input.KEY_KP3,
		'4':  input.KEY_KP4,
		'5':  input.KEY_KP5,
		'6':  input.KEY_KP6,
		'7':  input.KEY_KP7,
		'8':  input.KEY_KP8,
		'9':  input.KEY_KP9,
		'*':  input.KEY_KPASTERISK,
		'+':  input.KEY_KPPLUS,
		'-':  input.KEY_KPMINUS,
		'/':  input.KEY_KPSLASH,
		'=':  input.KEY_KPEQUAL,
		'.':  input.KEY_KPDOT,
		'\n': input.KEY_KPENTER,
	}

	typeUsingNumpad := func(s string) uiauto.Action {
		return func(ctx context.Context) error {
			for i, r := range []rune(s) {
				eventCode, ok := numpadKeyMapping[r]
				if !ok {
					return errors.Errorf("unknown rune %q at position %d", r, i)
				}

				// Type the event code for the rune.
				if err := keyboard.TypeKey(ctx, eventCode); err != nil {
					return errors.Wrapf(err, "failed to type %q at position %d", r, i)
				}
			}
			return nil
		}
	}

	subtests := []util.InputEval{
		{
			TestName:     "Type numbers using numpad",
			InputFunc:    typeUsingNumpad("0123456789"),
			ExpectedText: "0123456789",
		}, {
			TestName:     "Type operators using numpad",
			InputFunc:    typeUsingNumpad("*+-/=."),
			ExpectedText: "*+-/=.",
		}, {
			TestName:     "Use Enter key from numpad",
			InputFunc:    typeUsingNumpad("\n"),
			ExpectedText: "\n",
		},
	}

	for _, subtest := range subtests {
		s.Run(ctx, subtest.TestName, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+subtest.TestName)

			if err := uiauto.UserAction(
				"Verify numpad keys are functional",
				its.ValidateInputOnField(inputField, subtest.InputFunc, subtest.ExpectedText),
				uc, &useractions.UserActionCfg{
					Attributes: map[string]string{
						useractions.AttributeTestScenario: subtest.TestName,
						useractions.AttributeInputField:   string(inputField),
						useractions.AttributeFeature:      useractions.FeaturePKTyping,
					},
				},
			)(ctx); err != nil {
				s.Fatalf("Failed to validate numpad keys in %s: %v", inputField, err)
			}
		})
	}
}
