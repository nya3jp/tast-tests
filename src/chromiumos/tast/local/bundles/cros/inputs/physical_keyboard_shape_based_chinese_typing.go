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
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
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
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that shape-based Chinese physical keyboard works",
		Contacts:     []string{"shend@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.NonVKClamshell,
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name: "array",
				Val: pkShapeBasedChineseTestCase{
					inputMethod:    ime.ChineseArray,
					typingKeys:     "aaa lbj mc gds exxw pf alpe ajr .aad ame ",
					expectedResult: "三節外也關由面再行列",
				},
			},
			{
				Name: "cangjie",
				Val: pkShapeBasedChineseTestCase{
					inputMethod:    ime.ChineseCangjie,
					typingKeys:     "a jwj yrhhi hui hxyc oiar grmbc ",
					expectedResult: "日車謝鬼與倉頡",
				},
			},
			{
				Name: "dayi",
				Val: pkShapeBasedChineseTestCase{
					inputMethod:    ime.ChineseDayi,
					typingKeys:     "1 j 123 asox db/ ",
					expectedResult: "言月詐做易",
				},
			},
			{
				Name: "quick",
				Val: pkShapeBasedChineseTestCase{
					inputMethod:    ime.ChineseQuick,
					typingKeys:     "a jw yr an is ",
					expectedResult: "日富這門成",
				},
			},
			{
				Name: "wubi",
				Val: pkShapeBasedChineseTestCase{
					inputMethod:    ime.ChineseWubi,
					typingKeys:     "yge yygy ggll yygt gg tt ",
					expectedResult: "请文一方五笔",
				},
			},
		},
	})
}

func PhysicalKeyboardShapeBasedChineseTyping(ctx context.Context, s *testing.State) {
	testCase := s.Param().(pkShapeBasedChineseTestCase)

	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn
	uc := s.PreValue().(pre.PreData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

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

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Fail to launch inputs test server: ", err)
	}
	defer its.Close()

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
