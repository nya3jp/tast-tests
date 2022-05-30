// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/chromiumos/tast/ctxutil"
	"go.chromium.org/chromiumos/tast-tests/local/bundles/cros/inputs/fixture"
	"go.chromium.org/chromiumos/tast-tests/local/bundles/cros/inputs/pre"
	"go.chromium.org/chromiumos/tast-tests/local/bundles/cros/inputs/testserver"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/ime"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/faillog"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/useractions"
	"go.chromium.org/chromiumos/tast-tests/local/input"
	"go.chromium.org/chromiumos/tast/testing"
	"go.chromium.org/chromiumos/tast/testing/hwdep"
)

// pkShapeBasedChineseTestCase struct encapsulates parameters for each test.
type pkShapeBasedChineseTestCase struct {
	inputMethod       ime.InputMethod
	typingKeys        string
	expectedResult    string
	expectedCandidate string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardShapeBasedChineseTyping,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that shape-based Chinese physical keyboard works",
		Contacts:     []string{"shend@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name:    "array",
				Fixture: fixture.ClamshellNonVK,
				Val: pkShapeBasedChineseTestCase{
					inputMethod:    ime.ChineseArray,
					typingKeys:     "aaa lbj mc gds exxw pf alpe ajr .aad ame ",
					expectedResult: "三節外也關由面再行列",
				},
			},
			{
				Name:    "cangjie",
				Fixture: fixture.ClamshellNonVK,
				Val: pkShapeBasedChineseTestCase{
					inputMethod:    ime.ChineseCangjie,
					typingKeys:     "a jwj yrhhi hui hxyc oiar grmbc ",
					expectedResult: "日車謝鬼與倉頡",
				},
			},
			{
				Name:    "dayi",
				Fixture: fixture.ClamshellNonVK,
				Val: pkShapeBasedChineseTestCase{
					inputMethod:    ime.ChineseDayi,
					typingKeys:     "1 j 123 asox db/ ",
					expectedResult: "言月詐做易",
				},
			},
			{
				Name:    "quick",
				Fixture: fixture.ClamshellNonVK,
				Val: pkShapeBasedChineseTestCase{
					inputMethod:    ime.ChineseQuick,
					typingKeys:     "a jw yr an is ",
					expectedResult: "日富這門成",
				},
			},
			{
				Name:    "wubi",
				Fixture: fixture.ClamshellNonVK,
				Val: pkShapeBasedChineseTestCase{
					inputMethod:    ime.ChineseWubi,
					typingKeys:     "yge yygy ggll yygt gg tt ",
					expectedResult: "请文一方五笔",
				},
			},
			{
				Name:    "array_lacros",
				Fixture: fixture.LacrosClamshellNonVK,
				Val: pkShapeBasedChineseTestCase{
					inputMethod:    ime.ChineseArray,
					typingKeys:     "aaa lbj mc gds exxw pf alpe ajr .aad ame ",
					expectedResult: "三節外也關由面再行列",
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name:    "cangjie_lacros",
				Fixture: fixture.LacrosClamshellNonVK,
				Val: pkShapeBasedChineseTestCase{
					inputMethod:    ime.ChineseCangjie,
					typingKeys:     "a jwj yrhhi hui hxyc oiar grmbc ",
					expectedResult: "日車謝鬼與倉頡",
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name:    "dayi_lacros",
				Fixture: fixture.LacrosClamshellNonVK,
				Val: pkShapeBasedChineseTestCase{
					inputMethod:    ime.ChineseDayi,
					typingKeys:     "1 j 123 asox db/ ",
					expectedResult: "言月詐做易",
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name:    "quick_lacros",
				Fixture: fixture.LacrosClamshellNonVK,
				Val: pkShapeBasedChineseTestCase{
					inputMethod:    ime.ChineseQuick,
					typingKeys:     "a jw yr an is ",
					expectedResult: "日富這門成",
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name:    "wubi_lacros",
				Fixture: fixture.LacrosClamshellNonVK,
				Val: pkShapeBasedChineseTestCase{
					inputMethod:    ime.ChineseWubi,
					typingKeys:     "yge yygy ggll yygt gg tt ",
					expectedResult: "请文一方五笔",
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
		},
	})
}

func PhysicalKeyboardShapeBasedChineseTyping(ctx context.Context, s *testing.State) {
	testCase := s.Param().(pkShapeBasedChineseTestCase)

	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext
	uc.SetTestName(s.TestName())

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	im := testCase.inputMethod

	s.Log("Set current input method to: ", im)
	if err := im.InstallAndActivate(tconn)(ctx); err != nil {
		s.Fatalf("Failed to set input method to %v: %v: ", im, err)
	}
	uc.SetAttribute(useractions.AttributeInputMethod, im.Name)

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

	inputField := testserver.TextAreaInputField

	validateAction := its.ValidateInputOnField(inputField, kb.TypeAction(testCase.typingKeys), testCase.expectedResult)

	if err := uiauto.UserAction(
		"Shape-based Chinese PK input",
		validateAction,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeTestScenario: fmt.Sprintf(`type %q to get %q`, testCase.typingKeys, testCase.expectedResult),
				useractions.AttributeFeature:      useractions.FeaturePKTyping,
				useractions.AttributeInputField:   string(inputField),
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to verify shape-based typing: ", err)
	}
}
