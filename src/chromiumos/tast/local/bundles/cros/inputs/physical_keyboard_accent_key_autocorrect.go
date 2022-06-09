// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/imesettings"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardAccentKeyAutocorrect,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that physical keyboard with accent keys can perform typing with autocorrects",
		Contacts: []string{
			"vivian.tsai@cienet.com", // Author
			"shengjun@google.com",    // PoC
			"cienet-development@googlegroups.com",
			"essential-inputs-team@google.com",
		},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome"},
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.FrenchFrance}),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Fixture:           fixture.ClamshellNonVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			},
			{
				Name:              "informational",
				Fixture:           fixture.ClamshellNonVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			},
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraSoftwareDeps: []string{"lacros"},
			},
		},
	})
}

// PhysicalKeyboardAccentKeyAutocorrect checks that physical keyboard with accent keys can perform typing with autocorrects.
func PhysicalKeyboardAccentKeyAutocorrect(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

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

	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

	defer func(ctx context.Context) {
		if err := inputMethod.ResetSettings(tconn)(ctx); err != nil {
			// Only log errors in cleanup.
			s.Log("Failed to reset IME settings: ", err)
		}
	}(cleanupCtx)

	for _, sample := range []struct {
		inputString         string // string to type
		expectedString      string // expected result
		autocorrectedString string // autocorrected result
		autoCorrectLevel    imesettings.AutoCorrectionLevel
	}{
		{"2", "é", "é", imesettings.AutoCorrectionOff},             //normal
		{"d2jq", "déja", "déjà", imesettings.AutoCorrectionModest}, //misspelt
	} {
		f := func(ctx context.Context, s *testing.State) {
			inputField := testserver.TextAreaInputField

			action := uiauto.Combine("validate PK autocorrect with accent keys",
				imesettings.SetPKAutoCorrection(uc, inputMethod, sample.autoCorrectLevel),
				// TODO(b/157686038): remove sleep.
				// imesettings.setAutoCorrection: sleep for 5s may be too short to set auto correction for some DUTs which causes auto correction to fail.
				uiauto.Sleep(15*time.Second),
				its.ClearThenClickFieldAndWaitForActive(inputField),
				kb.TypeAction(sample.inputString),
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), sample.expectedString),
				kb.AccelAction("space"),
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), sample.autocorrectedString+" "),
			)

			if err := uiauto.UserAction("input PK autocorrect with accent keys",
				action,
				uc,
				&useractions.UserActionCfg{
					Attributes: map[string]string{
						useractions.AttributeInputField:   string(inputField),
						useractions.AttributeTestScenario: fmt.Sprintf(`output string is %q`, sample.autocorrectedString),
					}},
			)(ctx); err != nil {
				s.Fatalf("Failed to validate PK autocorrect with accent keys %s: %v", sample.expectedString, err)
			}

		}
		s.Run(ctx, fmt.Sprintf("test of accent key: %s", sample.expectedString), f)
	}
}
