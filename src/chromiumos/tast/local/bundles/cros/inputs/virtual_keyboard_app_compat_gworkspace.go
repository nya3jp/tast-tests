// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// vkTestValues struct encapsulates parameters for each vkTestValues.
type vkTestValues struct {
	regionLanguage string          // region and language to test
	labelOnVK      string          // language label shown on virtual keyboard
	inputMethod    ime.InputMethod // input method

}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardAppCompatGworkspace,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test inputs feature on virtual keyboard for google workspace",
		Contacts:     []string{"xiuwen@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.FrenchFrance, ime.EnglishUS}),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name:              "docs_french",
				Fixture:           fixture.GoogleDocs,
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				Val: vkTestValues{
					regionLanguage: "Frence-French",
					inputMethod:    ime.FrenchFrance,
					labelOnVK:      "FR",
				},
			},
			{
				Name:              "docs_english",
				Fixture:           fixture.GoogleDocs,
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				Val: vkTestValues{
					regionLanguage: "US-English",
					inputMethod:    ime.EnglishUS,
					labelOnVK:      "US",
				},
			},
			{
				Name:              "slides_french",
				Fixture:           fixture.GoogleSlides,
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				Val: vkTestValues{
					regionLanguage: "Frence-French",
					inputMethod:    ime.FrenchFrance,
					labelOnVK:      "FR",
				},
			},
			{
				Name:              "slides_english",
				Fixture:           fixture.GoogleSlides,
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				Val: vkTestValues{
					regionLanguage: "US-English",
					inputMethod:    ime.EnglishUS,
					labelOnVK:      "US",
				},
			},
			{
				Name:              "sheets_french",
				Fixture:           fixture.GoogleSheets,
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				Val: vkTestValues{
					regionLanguage: "Frence-French",
					inputMethod:    ime.FrenchFrance,
					labelOnVK:      "FR",
				},
			},
			{
				Name:              "sheets_english",
				Fixture:           fixture.GoogleSheets,
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				Val: vkTestValues{
					regionLanguage: "US-English",
					inputMethod:    ime.EnglishUS,
					labelOnVK:      "US",
				},
			},
		},
	})
}

func VirtualKeyboardAppCompatGworkspace(ctx context.Context, s *testing.State) {
	testCase := s.Param().(vkTestValues)
	cr := s.FixtValue().(fixture.WorkspaceFixtData).Chrome
	tconn := s.FixtValue().(fixture.WorkspaceFixtData).TestAPIConn
	uc := s.FixtValue().(fixture.WorkspaceFixtData).UserContext
	vkbCtx := vkb.NewContext(cr, tconn)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	if err := vkbCtx.ShowVirtualKeyboard()(ctx); err != nil {
		s.Fatal("Failed to show the virtual keyboard: ", err)
	}

	inputMethod := testCase.inputMethod
	if err := inputMethod.InstallAndActivateUserAction(uc)(ctx); err != nil {
		s.Fatal("Fail to set input method: ", err)
	}

	uc.SetAttribute(useractions.AttributeInputMethod, inputMethod.Name)
	ui := uiauto.New(tconn)
	languageLabelFinder := vkb.NodeFinder.Name(testCase.labelOnVK).First()
	ud := uidetection.NewDefault(tconn).WithTimeout(time.Minute).WithScreenshotStrategy(uidetection.ImmediateScreenshot)

	langueageTests := map[string][]util.AppCompatTestCase{
		"Frence-French": {
			{
				TestName:    "accent key",
				Description: "user type héllo",
				UserActions: uiauto.Combine("user type text héllo",
					vkbCtx.TapKey("enter"),
					ui.WaitUntilExists(languageLabelFinder),
					vkbCtx.TapKeyIgnoringCase("h"),
					vkbCtx.TapAccentKey("e", "é"),
					vkbCtx.TapKeys(strings.Split("llo", "")),
					ud.WaitUntilExists(uidetection.Word("héllo", uidetection.DisableApproxMatch(true)).First()),
				),
			},
			{
				TestName:    "glide typing",
				Description: "user to glide typing to input word bonjour",
				UserActions: uiauto.Combine("test",
					vkbCtx.TapKey("enter"),
					vkbCtx.GlideTyping(strings.Split("bonjour", ""), ud.WaitUntilExists(uidetection.Word("bonjour", uidetection.DisableApproxMatch(true)).First())),
				),
			},
		},
		"US-English": {
			{
				TestName:    "normal typing",
				Description: "user typing word English",
				UserActions: uiauto.Combine("testing typing, user type text English",
					vkbCtx.TapKey("enter"),
					vkbCtx.TapKeyIgnoringCase("e"),
					vkbCtx.TapKeys(strings.Split("nglish", "")),
					ud.WaitUntilExists(uidetection.Word("English", uidetection.DisableApproxMatch(true)).First()),
				),
			},
		},
	}

	for _, subtest := range langueageTests[testCase.regionLanguage] {
		s.Run(ctx, subtest.TestName, func(ctx context.Context, s *testing.State) {
			if err := uiauto.UserAction(
				subtest.TestName,
				subtest.UserActions,
				uc, &useractions.UserActionCfg{
					Attributes: map[string]string{
						useractions.AttributeTestScenario: subtest.TestName,
						useractions.AttributeFeature:      useractions.FeatureVKTyping,
					},
				},
			)(ctx); err != nil {
				s.Fatalf("Failed to validate %s typing in test %s: %v", testCase.regionLanguage, subtest.TestName, err)
			}
		})
	}
}
