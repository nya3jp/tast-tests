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
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// pkGoogleWorkspaceApptestCase struct encapsulates parameters for each test.
type pkGoogleWorkspaceApptestCase struct {
	feature string
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
				Name:    "deadkey_doc",
				Fixture: fixture.GoogleDocs,
				Val: pkGoogleWorkspaceApptestCase{
					feature: "deadkey",
				},
				ExtraAttr:        []string{"informational"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.FrenchFrance}),
			},
			{
				Name:    "deadkey_slide",
				Fixture: fixture.GoogleSlides,
				Val: pkGoogleWorkspaceApptestCase{
					feature: "deadkey",
				},
				ExtraAttr:        []string{"informational"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.FrenchFrance}),
			},
			{
				Name:    "deadkey_sheet",
				Fixture: fixture.GoogleSheets,
				Val: pkGoogleWorkspaceApptestCase{
					feature: "deadkey",
				},
				ExtraAttr:        []string{"informational"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.FrenchFrance}),
			},
			{
				Name:    "typing_doc",
				Fixture: fixture.GoogleDocs,
				Val: pkGoogleWorkspaceApptestCase{
					feature: "typing",
				},
				ExtraAttr:        []string{"informational"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.FrenchFrance}),
			},
			{
				Name:    "typing_slide",
				Fixture: fixture.GoogleSlides,
				Val: pkGoogleWorkspaceApptestCase{
					feature: "typing",
				},
				ExtraAttr:        []string{"informational"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.FrenchFrance}),
			},
		},
	})
}

func PhysicalKeyboardAppCompatGworkspace(ctx context.Context, s *testing.State) {
	testCase := s.Param().(pkGoogleWorkspaceApptestCase)

	cr := s.FixtValue().(fixture.GworkspaceFixtData).Chrome
	tconn := s.FixtValue().(fixture.GworkspaceFixtData).TestAPIConn
	uc := s.FixtValue().(fixture.GworkspaceFixtData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	switch testCase.feature {
	case "deadkey":
		var subtests = []util.AppCompatTestCase{
			{
				TestName:      "french dead key [",
				LanguageLabel: "FR",
				InputMethod:   ime.FrenchFrance,
				TypeKeys:      "[e",
				ExpectedText:  "ê",
			},
		}

		for _, subtest := range subtests {
			if err := util.InstallIME(ctx, uc, subtest.InputMethod); err != nil {
				s.Fatal("Fail to set input method: ", err)
			}

			action, err := util.TypingKeysAccordingToLanguageOnPK(ctx, tconn, ime.FrenchFrance, subtest.TypeKeys, subtest.ExpectedText)
			if err != nil {
				s.Fatal("fail to get keyboard")
			}

			s.Run(ctx, subtest.TestName, func(ctx context.Context, s *testing.State) {
				if err := uiauto.UserAction(
					subtest.TestName,
					action,
					uc, &useractions.UserActionCfg{
						Attributes: map[string]string{
							useractions.AttributeTestScenario: subtest.TestName,
							useractions.AttributeFeature:      useractions.FeaturePKTyping,
						},
					},
				)(ctx); err != nil {
					s.Fatalf("Failed to validate typing in test %s: %v", subtest.TestName, err)
				}
			})
			util.ClickEnterToStartNewLine(ctx)
		}
	case "typing":
		var subtests = []util.AppCompatTestCase{
			{
				TestName:      "french typing qawzmù,",
				LanguageLabel: "FR",
				InputMethod:   ime.FrenchFrance,
				TypeKeys:      "aqzw;'m",
				ExpectedText:  "qawzmù,",
			},
			{
				TestName:      "french typing ",
				LanguageLabel: "FR",
				InputMethod:   ime.FrenchFrance,
				TypeKeys:      ",h.h/",
				ExpectedText:  ";h:h!",
			},
			{
				TestName:      "french typing ",
				LanguageLabel: "FR",
				InputMethod:   ime.FrenchFrance,
				TypeKeys:      "125b67890",
				ExpectedText:  "&é(b-è_çà",
			},
			{
				TestName:      "french typing ",
				LanguageLabel: "FR",
				InputMethod:   ime.FrenchFrance,
				TypeKeys:      "3b4",
				ExpectedText:  `"b'`,
			},
		}

		for _, subtest := range subtests {
			if err := util.InstallIME(ctx, uc, subtest.InputMethod); err != nil {
				s.Fatal("Fail to set input method: ", err)
			}

			action, err := util.TypingKeysAccordingToLanguageOnPK(ctx, tconn, ime.FrenchFrance, subtest.TypeKeys, subtest.ExpectedText)
			if err != nil {
				s.Fatal("fail to get keyboard")
			}

			s.Run(ctx, subtest.TestName, func(ctx context.Context, s *testing.State) {
				if err := uiauto.UserAction(
					subtest.TestName,
					action,
					uc, &useractions.UserActionCfg{
						Attributes: map[string]string{
							useractions.AttributeTestScenario: subtest.TestName,
							useractions.AttributeFeature:      useractions.FeaturePKTyping,
						},
					},
				)(ctx); err != nil {
					s.Fatalf("Failed to validate typing in test %s: %v", subtest.TestName, err)
				}
			})
			util.ClickEnterToStartNewLine(ctx)
		}
	}
}
