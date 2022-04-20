// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
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

// deadKeysTestCase struct encapsulates parameters for each Dead Keys test.
type pkDeadKeysTestCase struct {
	inputMethod          ime.InputMethod
	typingKeys           string
	expectedTypingResult string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardDeadKeys,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that dead keys on the physical keyboard work",
		Contacts:     []string{"shend@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name: "french",
				Pre:  pre.NonVKClamshell,
				Val: pkDeadKeysTestCase{
					inputMethod:          ime.FrenchFrance,
					typingKeys:           "[e",
					expectedTypingResult: "ê",
				},
			},
			{
				Name: "us_intl_acute",
				Pre:  pre.NonVKClamshell,
				Val: pkDeadKeysTestCase{
					inputMethod:          ime.EnglishUSWithInternationalKeyboard,
					typingKeys:           "'a",
					expectedTypingResult: "á",
				},
			},
			{
				Name: "us_intl_double",
				Pre:  pre.NonVKClamshell,
				Val: pkDeadKeysTestCase{
					inputMethod:          ime.EnglishUSWithInternationalKeyboard,
					typingKeys:           "''",
					expectedTypingResult: "´",
				},
			},
			{
				Name:      "french_fixture",
				Fixture:   fixture.ClamshellNonVK,
				ExtraAttr: []string{"informational"},
				Val: pkDeadKeysTestCase{
					inputMethod:          ime.FrenchFrance,
					typingKeys:           "[e",
					expectedTypingResult: "ê",
				},
			},
			{
				Name:      "us_intl_acute_fixture",
				Fixture:   fixture.ClamshellNonVK,
				ExtraAttr: []string{"informational"},
				Val: pkDeadKeysTestCase{
					inputMethod:          ime.EnglishUSWithInternationalKeyboard,
					typingKeys:           "'a",
					expectedTypingResult: "á",
				},
			},
			{
				Name:      "us_intl_double_fixture",
				Fixture:   fixture.ClamshellNonVK,
				ExtraAttr: []string{"informational"},
				Val: pkDeadKeysTestCase{
					inputMethod:          ime.EnglishUSWithInternationalKeyboard,
					typingKeys:           "''",
					expectedTypingResult: "´",
				},
			},
		},
	})
}

func PhysicalKeyboardDeadKeys(ctx context.Context, s *testing.State) {
	testCase := s.Param().(pkDeadKeysTestCase)

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

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	inputMethod := testCase.inputMethod
	if err := inputMethod.InstallAndActivate(tconn)(ctx); err != nil {
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
