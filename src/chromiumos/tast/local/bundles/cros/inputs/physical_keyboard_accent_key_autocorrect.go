// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/imesettings"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type inputWord struct {
	inputString         string // string to type
	expectedString      string // expected result
	autocorrectedString string // autocorrected result
	autocorrect         bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardAccentKeyAutocorrect,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that physical keyboard with accent keys can perform typing with autocorrects",
		Contacts: []string{
			"vivian.tsai@cienet.com", // Author
			"shengjun@google.com",    // PoC
			"cienet-development@googlegroups.com",
			"essential-inputs-team@google.com",
		},
		Attr:         []string{"group:mainline", "group:input-tools", "group:input-tools-upstream", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Pre:          pre.NonVKClamshellReset,
		Params: []testing.Param{
			{
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			}, {
				Name:              "unstable",
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			}},
	})
}

// PhysicalKeyboardAccentKeyAutocorrect checks that physical keyboard with accent keys can perform typing with autocorrects.
func PhysicalKeyboardAccentKeyAutocorrect(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn
	uc := s.PreValue().(pre.PreData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	inputMethod := ime.FrenchFrance
	s.Logf("Set current input method to: %q", inputMethod)

	if err := inputMethod.InstallAndActivate(tconn)(ctx); err != nil {
		s.Fatalf("Failed to set input method to %q: %v: ", inputMethod, err)
	}
	uc.SetAttribute(useractions.AttributeInputMethod, inputMethod.Name)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Fail to launch inputs test server: ", err)
	}
	defer its.Close()
	defer its.ClosePage(cleanupCtx)

	inputField := testserver.TextAreaInputField
	uc.SetAttribute(useractions.AttributeInputField, string(inputField))

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_dump")

	tests := []inputWord{
		{"2", "é", "é", false},         // normal
		{"d2jq", "déja", "déjà", true}, // misspelt
	}
	for _, sample := range tests {
		if sample.autocorrect {
			if err := imesettings.SetPKAutoCorrection(uc, inputMethod, imesettings.AutoCorrectionModest)(ctx); err != nil {
				s.Fatal("Failed to set PK auto correction: ", err)
			}
			defer func(ctx context.Context) {
				if err := inputMethod.ResetSettings(tconn)(ctx); err != nil {
					// Only log errors in cleanup.
					s.Log("Failed to reset IME settings: ", err)
				}

				inputMethod.Remove(tconn)(ctx)
			}(cleanupCtx)
		}

		if err := validateAutocorrectAccentKey(ctx, uc, its, kb, tconn, sample); err != nil {
			s.Fatal("Failed to validate PK autocorrect with accent keys: ", err)
		}
	}
}

// validateAutocorrectAccentKey types misspelt word and validate if auto-correction works.
func validateAutocorrectAccentKey(ctx context.Context, uc *useractions.UserContext, its *testserver.InputsTestServer, keyboard *input.KeyboardEventWriter, tconn *chrome.TestConn, sample inputWord) error {
	inputField := testserver.TextAreaInputField

	action := uiauto.Combine("validate PK autocorrect with accent keys",
		its.Clear(inputField),
		its.ClickFieldAndWaitForActive(inputField),
		keyboard.TypeAction(sample.inputString),
		util.WaitForFieldTextToBe(tconn, inputField.Finder(), sample.expectedString),
		keyboard.TypeAction(" "),
		util.WaitForFieldTextToBe(tconn, inputField.Finder(), sample.autocorrectedString+" "),
	)

	return uiauto.UserAction("Input PK autocorrect with accent keys",
		action,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeTestScenario: fmt.Sprintf(`output string is %q`, sample.autocorrectedString),
			},
		})(ctx)
}
