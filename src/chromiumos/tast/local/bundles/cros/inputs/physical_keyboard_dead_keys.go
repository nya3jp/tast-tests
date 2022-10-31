// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// deadKeysTestCase struct encapsulates parameters for each Dead Keys test.
type pkDeadKeysTestCase struct {
	inputMethod          ime.InputMethod
	typingKeys           string
	expectedTypingResult string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardDeadKeys,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that dead keys on the physical keyboard work",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name:    "french",
				Fixture: fixture.ClamshellNonVK,
				Val: pkDeadKeysTestCase{
					inputMethod:          ime.FrenchFrance,
					typingKeys:           "[e",
					expectedTypingResult: "ê",
				},
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.FrenchFrance}),
			},
			{
				Name:    "us_intl_acute",
				Fixture: fixture.ClamshellNonVK,
				Val: pkDeadKeysTestCase{
					inputMethod:          ime.EnglishUSWithInternationalKeyboard,
					typingKeys:           "'a",
					expectedTypingResult: "á",
				},
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.EnglishUSWithInternationalKeyboard}),
			},
			{
				Name:    "us_intl_double",
				Fixture: fixture.ClamshellNonVK,
				Val: pkDeadKeysTestCase{
					inputMethod:          ime.EnglishUSWithInternationalKeyboard,
					typingKeys:           "''",
					expectedTypingResult: "´",
				},
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.EnglishUSWithInternationalKeyboard}),
			},
			{
				Name:    "french_lacros",
				Fixture: fixture.LacrosClamshellNonVK,
				Val: pkDeadKeysTestCase{
					inputMethod:          ime.FrenchFrance,
					typingKeys:           "[e",
					expectedTypingResult: "ê",
				},
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"group:input-tools-upstream"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.FrenchFrance}),
			},
			{
				Name:    "us_intl_acute_lacros",
				Fixture: fixture.LacrosClamshellNonVK,
				Val: pkDeadKeysTestCase{
					inputMethod:          ime.EnglishUSWithInternationalKeyboard,
					typingKeys:           "'a",
					expectedTypingResult: "á",
				},
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"group:input-tools-upstream"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUSWithInternationalKeyboard}),
			},
			{
				Name:    "us_intl_double_lacros",
				Fixture: fixture.LacrosClamshellNonVK,
				Val: pkDeadKeysTestCase{
					inputMethod:          ime.EnglishUSWithInternationalKeyboard,
					typingKeys:           "''",
					expectedTypingResult: "´",
				},
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUSWithInternationalKeyboard}),
			},
		},
	})
}

func PhysicalKeyboardDeadKeys(ctx context.Context, s *testing.State) {
	testCase := s.Param().(pkDeadKeysTestCase)

	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

	inputMethod := testCase.inputMethod
	if err := inputMethod.InstallAndActivateUserAction(uc)(ctx); err != nil {
		s.Fatalf("Failed to set input method to %s: %v: ", inputMethod, err)
	}
	uc.SetAttribute(useractions.AttributeInputMethod, inputMethod.Name)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	inputField := testserver.TextAreaInputField

	validateAction := uiauto.Combine("validate dead keys typing",
		its.ClickFieldAndWaitForActive(inputField),
		kb.TypeAction(testCase.typingKeys),
		util.WaitForFieldTextToBe(tconn, inputField.Finder(), testCase.expectedTypingResult),
	)

	if err := uiauto.UserAction(
		"PK dead keys input",
		validateAction,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeFeature:      useractions.FeatureDeadKeys,
				useractions.AttributeTestScenario: fmt.Sprintf(`type %q to get %q`, testCase.typingKeys, testCase.expectedTypingResult),
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to verify dead keys input: ", err)
	}
}
