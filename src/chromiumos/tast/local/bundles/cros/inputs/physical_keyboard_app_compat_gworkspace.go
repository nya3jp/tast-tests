// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// pkTestValues struct encapsulates parameters for each test.
type pkTestValues struct {
	regionLanguage string          // region and language to test
	inputMethod    ime.InputMethod // input method
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardAppCompatGworkspace,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that dead keys on the physical keyboard work",
		Contacts:     []string{"xiuwen@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name:    "docs_french",
				Fixture: fixture.GoogleDocs,
				Val: pkTestValues{
					regionLanguage: "Frence-French",
					inputMethod:    ime.FrenchFrance,
				},
				ExtraAttr:        []string{"informational"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.FrenchFrance}),
			},
			{
				Name:    "sheets_french",
				Fixture: fixture.GoogleSheets,
				Val: pkTestValues{
					regionLanguage: "Frence-French",
					inputMethod:    ime.FrenchFrance,
				},
				ExtraAttr:        []string{"informational"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.FrenchFrance}),
			},
			{
				Name:    "slides_french",
				Fixture: fixture.GoogleSlides,
				Val: pkTestValues{
					regionLanguage: "Frence-French",
					inputMethod:    ime.FrenchFrance,
				},
				ExtraAttr:        []string{"informational"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.FrenchFrance}),
			},
		},
	})
}

func PhysicalKeyboardAppCompatGworkspace(ctx context.Context, s *testing.State) {
	testCase := s.Param().(pkTestValues)

	cr := s.FixtValue().(fixture.WorkspaceFixtData).Chrome
	tconn := s.FixtValue().(fixture.WorkspaceFixtData).TestAPIConn
	uc := s.FixtValue().(fixture.WorkspaceFixtData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	inputMethod := testCase.inputMethod
	if err := inputMethod.InstallAndActivateUserAction(uc)(ctx); err != nil {
		s.Fatal("Fail to set input method: ", err)
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	ud := uidetection.NewDefault(tconn).WithTimeout(time.Minute).WithScreenshotStrategy(uidetection.ImmediateScreenshot)

	languageTests := map[string][]util.AppCompatTestCase{
		"Frence-French": {
			{
				TestName:    "test number key 0 to 9",
				Description: "test type number key from 0 to 9",
				UserActions: uiauto.Combine("user type text &é(b-",
					keyboard.AccelAction("Enter"),
					keyboard.TypeAction("0 1 2 3 4 5 6 7 8 9 0"),
					ud.WaitUntilExists(uidetection.Word("à", uidetection.DisableApproxMatch(true)).First()), // verify letter one by one
					ud.WaitUntilExists(uidetection.Word("&", uidetection.DisableApproxMatch(true)).First()),
					ud.WaitUntilExists(uidetection.Word("é", uidetection.DisableApproxMatch(true)).First()),
					ud.WaitUntilExists(uidetection.Word(`"`, uidetection.DisableApproxMatch(true)).First()),
					ud.WaitUntilExists(uidetection.Word("'", uidetection.DisableApproxMatch(true)).First()),
					ud.WaitUntilExists(uidetection.Word("(", uidetection.DisableApproxMatch(true)).First()),
					ud.WaitUntilExists(uidetection.Word("-", uidetection.DisableApproxMatch(true)).First()),
					ud.WaitUntilExists(uidetection.Word("è", uidetection.DisableApproxMatch(true)).First()),
					ud.WaitUntilExists(uidetection.Word("ç", uidetection.DisableApproxMatch(true)).First()),
				),
			},
			{
				TestName:    "test dead key [",
				Description: "test type dead key [ on keyboard",
				UserActions: uiauto.Combine("user type text héllo",
					keyboard.AccelAction("Enter"),
					keyboard.TypeAction("h[ello"),
					ud.WaitUntilExists(uidetection.Word("héllo", uidetection.DisableApproxMatch(true)).First()),
				),
			},
		},
	}

	for _, subtest := range languageTests[testCase.regionLanguage] {
		s.Run(ctx, subtest.TestName, func(ctx context.Context, s *testing.State) {
			if err := uiauto.UserAction(
				subtest.TestName,
				subtest.UserActions,
				uc, &useractions.UserActionCfg{
					Attributes: map[string]string{
						useractions.AttributeTestScenario: subtest.TestName,
						useractions.AttributeFeature:      useractions.FeaturePKTyping,
					},
				},
			)(ctx); err != nil {
				s.Fatalf("Failed to validate %s typing in test %s: %v", testCase.regionLanguage, subtest.TestName, err)
			}
		})
	}
}
