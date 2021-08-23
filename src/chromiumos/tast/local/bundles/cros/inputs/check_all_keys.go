// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckAllKeys,
		Desc: "Checks virtual keyboard functionalities of language EnglishUS, EnglishUK, Alphanumeric With JapaneseKeyboard, Japanese With US Keyboard and Japanese",
		Contacts: []string{
			"lance.wang@cienet.com", // Author
			"shengjun@google.com",   // PoC
			"cienet-development@googlegroups.com",
			"essential-inputs-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational", "group:input-tools"},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.VKEnabledTabletReset,
		Timeout:      15 * time.Minute,
	})
}

// CheckAllKeys tests virtual keyboard features for different languages.
func CheckAllKeys(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	vkbCtx := vkb.NewContext(cr, tconn)

	its, err := testserver.LaunchInMode(ctx, cr, tconn, false)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer func(ctx context.Context) {
		faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
		its.ClosePage(ctx)
		its.Close()
	}(cleanupCtx)

	for _, kb := range []ime.InputMethod{
		ime.Japanese,
		ime.JapaneseWithUSKeyboard,
		ime.AlphanumericWithJapaneseKeyboard,
		ime.EnglishUS,
		ime.EnglishUK,
	} {
		f := func(ctx context.Context, s *testing.State) {
			testing.ContextLog(ctx, "Switching input method to ", kb.Name)
			if err := kb.InstallAndActivate(tconn)(ctx); err != nil {
				s.Fatalf("Failed to switch to input mehtod [%s]: %v", kb.Name, err)
			}

			for _, detail := range inputTests(kb) {
				if len(detail.characters) == 0 || detail.expectedText == "" {
					continue
				}

				s.Log("Verifying vk basic input functionality")
				if err := uiauto.Combine("validate virtual keyboard input functionality",
					its.Clear(testserver.TextAreaInputField),
					its.ClickFieldAndWaitForActive(testserver.TextAreaInputField),
					switchInputOption(vkbCtx, detail.chType),
					vkbCtx.TapKeysIgnoringCase(detail.characters),
					util.WaitForFieldTextToBeIgnoringCase(tconn, testserver.TextAreaInputField.Finder(), detail.expectedText),
				)(ctx); err != nil {
					s.Fatal("Failed to complete tests: ", err)
				}

				s.Log("Verifying vk additional functionality")
				uc := useractions.NewUserContext(s.TestName(), cr, tconn, s.OutDir(), nil, []useractions.ActionTag{useractions.ActionTagEssentialInputs})
				micNodeFinder := vkb.NodeFinder.ClassName("voice-mic-img").First()
				if err := uiauto.Combine("validate additional virtual keyboard functionalities",
					vkbCtx.SetFloatingMode(uc, true).Run,
					vkbCtx.SwitchToVoiceInput(),
					vkbCtx.TapNode(micNodeFinder),
					vkbCtx.SetFloatingMode(uc, false).Run,
				)(ctx); err != nil {
					s.Fatal("Failed to complete tests: ", err)
				}
			}
		}

		if !s.Run(ctx, fmt.Sprintf("verify virtual keyboard subcase %q", kb.Name), f) {
			s.Errorf("Failed to complete test of verifying virtual keyboard %q", kb.Name)
		}
	}
}

const (
	allLetters   = "qwertyuiopasdfghjklzxcvbnm"
	allNumbers   = "1234567890"
	basicSymbols = "@#$%&-+()\\=*\"':;!?_/,."
	extraSymbols = "~`|•√π÷×¶Δ£¢€¥^°={}\\©®™℅[]¡¿<>,."
)

type characterType int

const (
	letter characterType = iota
	number
	symbol
	symbolExtra
)

func switchInputOption(vkbCtx *vkb.VirtualKeyboardContext, chType characterType) uiauto.Action {
	switch chType {
	case number, symbol:
		return vkbCtx.TapKey("switch to symbols")
	case symbolExtra:
		return uiauto.Combine("switch to more symbols",
			vkbCtx.TapKey("switch to symbols"),
			vkbCtx.TapNode(vkb.KeyFinder.Name("switch to more symbols").First()), // Multiple nodes exist, cannot use vkbCtx.TapKey.
		)
	}
	return func(context.Context) error { return nil }
}

type testDetail struct {
	chType       characterType
	characters   []string
	expectedText string
}

func inputTests(inputMethod ime.InputMethod) []*testDetail {
	return []*testDetail{
		letterTest(inputMethod),
		numberTest(inputMethod),
		symbolsBasicTest(inputMethod),
		symbolsExtraTest(inputMethod),
	}
}

func letterTest(inputMethod ime.InputMethod) *testDetail {
	td := &testDetail{
		chType:       letter,
		characters:   strings.Split(allLetters, ""),
		expectedText: allLetters,
	}

	switch inputMethod {
	case ime.JapaneseWithUSKeyboard:
		// To include all English letters into test, we use a long string here.
		td.characters = strings.Split("simidakontonnomonsyouhusonnarukyoukinoutuwawakiagarihiteisisibirematatakinemuriwosamatageruhakousurutetunoouzyotaezuzikaisurudorononingyouketugouseyohanpatuseyotinimitionorenomuryokuwosirexilevi", "")
		td.expectedText = "しみだこんとんおもんしょうふそんあるきょうきのうつわわきあがりひていししびれまたたきねむりをさまたげるはこうするてつのおうじょたえずじかいするどろのにんぎょうけつごうせよはんぱつせよちにみちおのれのむりょくをしれぃぇゔぃ"
	case ime.Japanese:
		td.expectedText = "ｑうぇｒちゅいおぱｓｄｆｇｈｊｋｌｚｘｃｖｂんｍ"
	}

	return td
}

func numberTest(inputMethod ime.InputMethod) *testDetail {
	td := &testDetail{
		chType:       number,
		characters:   strings.Split(allNumbers, ""),
		expectedText: allNumbers,
	}

	switch inputMethod {
	case ime.JapaneseWithUSKeyboard, ime.Japanese:
		td.expectedText = "１２３４５６７８９０"
	}

	return td
}

func symbolsBasicTest(inputMethod ime.InputMethod) *testDetail {
	td := &testDetail{
		chType:       symbol,
		characters:   strings.Split(basicSymbols, ""),
		expectedText: basicSymbols,
	}

	switch inputMethod {
	case ime.EnglishUK:
		symbols := strings.ReplaceAll(basicSymbols, "$", "£")
		td.characters = strings.Split(symbols, "")
		td.expectedText = symbols
	case ime.JapaneseWithUSKeyboard:
		symbols := strings.ReplaceAll(basicSymbols, ",.", "、。")
		td.characters = strings.Split(symbols, "")
		td.expectedText = "＠＃＄％＆ー＋（）￥＝＊”’：；！？＿・、。"
	case ime.Japanese:
		symbols := strings.ReplaceAll(basicSymbols, ",.", "、。")
		td.characters = strings.Split(symbols, "")
		td.expectedText = "＠＃＄％＆ー＋（）￥＝＊”’：；！？ー・、。"
	}

	return td
}

func symbolsExtraTest(inputMethod ime.InputMethod) *testDetail {
	td := &testDetail{
		chType:       symbolExtra,
		characters:   strings.Split(extraSymbols, ""),
		expectedText: extraSymbols,
	}

	switch inputMethod {
	case ime.EnglishUK:
		symbols := strings.ReplaceAll(extraSymbols, "£¢€¥", "€¥$¢")
		td.characters = strings.Split(symbols, "")
		td.expectedText = symbols
	case ime.JapaneseWithUSKeyboard:
		symbols := strings.ReplaceAll(extraSymbols, ",.", "、。")
		td.characters = strings.Split(symbols, "")
		td.expectedText = "〜｀｜•√π÷×¶Δ£¢€¥＾°＝｛｝￥©®™℅「」¡¿＜＞、。"
	case ime.Japanese:
		symbols := strings.ReplaceAll(extraSymbols, ",.", "、。")
		td.characters = strings.Split(symbols, "")
		td.expectedText = "〜｀｜•√π÷×¶Δ£¢€￥＾°＝｛｝￥©®™℅「」¡¿＜＞、。"
	}

	return td
}
