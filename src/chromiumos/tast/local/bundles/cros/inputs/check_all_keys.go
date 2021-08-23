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
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CheckAllKeys,
		Desc:         "Checks virtual keyboard functionalities of language US, UK, JP-Romaji and JP-KANA",
		Contacts:     []string{"lance.wang@cienet.com", "cienet-development@googlegroups.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(pre.InputsUnstableModels),
		Timeout:      15 * time.Minute,
	})
}

type inputMethod string

const (
	enUs    inputMethod = "EnglishUS"
	enUK    inputMethod = "EnglishUK"
	jpUskb  inputMethod = "Japanese With US Keyboard"
	jpAlkb  inputMethod = "Alphanumeric With JapaneseKeyboard"
	jpKana  inputMethod = "Japanese Kana"
	jpRoman inputMethod = "Japanese Roman"
)

// CheckAllKeys tests virtual keyboard features for different languages.
func CheckAllKeys(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.VKEnabled())
	if err != nil {
		s.Fatal("Failed to Chrome login: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connection: ", err)
	}

	type test struct {
		ime     ime.InputMethod
		details []*testDetail
	}

	vkbCtx := vkb.NewContext(cr, tconn)
	tests := map[inputMethod]test{
		enUs:    {ime.EnglishUS, inputTests(enUs)},
		enUK:    {ime.EnglishUK, inputTests(enUK)},
		jpUskb:  {ime.JapaneseWithUSKeyboard, inputTests(jpUskb)},
		jpAlkb:  {ime.AlphanumericWithJapaneseKeyboard, inputTests(jpAlkb)},
		jpKana:  {ime.Japanese, inputTests(jpKana)},
		jpRoman: {ime.Japanese, inputTests(jpRoman)},
	}

	for inputmethod, test := range tests {
		f := func(ctx context.Context, s *testing.State) {
			cleanupSubtestsCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
			defer cancel()

			testing.ContextLog(ctx, "Switching input method to ", test.ime.Name)
			if err := test.ime.InstallAndActivate(tconn)(ctx); err != nil {
				s.Fatalf("Failed to switch to input mehtod [%s]: %v", test.ime.Name, err)
			}

			if err := setupJpKeyboardFormat(ctx, cr, tconn, vkbCtx, inputmethod); err != nil {
				s.Fatal("Failed to setup Japanese keyboard format: ", err)
			}

			its, err := testserver.LaunchInMode(ctx, cr, tconn, false)
			if err != nil {
				s.Fatal("Failed to launch inputs test server: ", err)
			}
			defer func(ctx context.Context) {
				faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)
				faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
				its.ClosePage(ctx)
				its.Close()
			}(cleanupSubtestsCtx)

			for _, detail := range test.details {
				if len(detail.characters) == 0 || detail.expectedText == "" {
					continue
				}

				testing.ContextLog(ctx, "Verifying vk basic input functionality")
				if err := uiauto.Combine("validate virtual keyboard input functionality",
					its.Clear(testserver.TextAreaInputField),
					its.ClickFieldAndWaitForActive(testserver.TextAreaInputField),
					switchInputOption(vkbCtx, detail.chType),
					vkbCtx.TapKeysIgnoringCase(detail.characters),
					util.WaitForFieldTextToBeIgnoringCase(tconn, testserver.TextAreaInputField.Finder(), detail.expectedText),
				)(ctx); err != nil {
					s.Fatal("Failed to complete tests: ", err)
				}

				uc := useractions.NewUserContext(s.TestName(), cr, tconn, s.OutDir(), nil, []useractions.ActionTag{useractions.ActionTagEssentialInputs})

				if detail.checkSuggestions {
					testing.ContextLog(ctx, "Checking input suggestions")
					candidates, err := vkbCtx.GetSuggestions(ctx)
					if err != nil {
						s.Fatal("Failed to get input suggestions: ", err)
					}
					if detail.expectSuggestionsShows && len(candidates) == 0 {
						s.Fatal("Failed to complete tests: input suggestions malfunction")
					}
				}

				testing.ContextLog(ctx, "Verifying vk additional functionality")
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

		if !s.Run(ctx, fmt.Sprintf("verify virtual keyboard subcase %q", inputmethod), f) {
			s.Errorf("Failed to complete test of verifying virtual keyboard %q", inputmethod)
		}
	}
}

func setupJpKeyboardFormat(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, vkbCtx *vkb.VirtualKeyboardContext, subtest inputMethod) error {
	expr := `
		document.getElementById('preedit_method').value = '%s';
		document.getElementById('preedit_method').dispatchEvent(new Event('change'));
	`
	switch subtest {
	case jpKana:
		expr = fmt.Sprintf(expr, "KANA")
	case jpRoman:
		expr = fmt.Sprintf(expr, "ROMAN")
	default:
		return nil
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	conn, err := cr.NewConn(ctx, "chrome-extension://jkghodnilhceideoidjikpgommlajknk/mozc_option.html")
	if err != nil {
		return errors.Wrap(err, "failed to connect to mozc extension")
	}
	defer conn.Close()
	defer conn.CloseTarget(cleanupCtx)

	settingPageHeaderFinder := nodewith.Role(role.Heading).Name("Japanese input settings")
	if err := uiauto.New(tconn).LeftClickUntil(settingPageHeaderFinder, vkbCtx.WaitUntilHidden())(ctx); err != nil {
		return err
	}

	return conn.Eval(ctx, expr, nil)
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

	checkSuggestions       bool
	expectSuggestionsShows bool
}

func inputTests(subtest inputMethod) []*testDetail {
	return []*testDetail{
		letterTest(subtest),
		numberTest(subtest),
		symbolsBasicTest(subtest),
		symbolsExtraTest(subtest),
	}
}

func letterTest(st inputMethod) *testDetail {
	td := &testDetail{
		chType:                 letter,
		characters:             strings.Split(allLetters, ""),
		expectedText:           allLetters,
		checkSuggestions:       true,
		expectSuggestionsShows: true,
	}

	switch st {
	case jpUskb:
		// To include all English letters into test, we use a long string here.
		td.characters = strings.Split("simidakontonnomonsyouhusonnarukyoukinoutuwawakiagarihiteisisibirematatakinemuriwosamatageruhakousurutetunoouzyotaezuzikaisurudorononingyouketugouseyohanpatuseyotinimitionorenomuryokuwosirexilevi", "")
		td.expectedText = "しみだこんとんおもんしょうふそんあるきょうきのうつわわきあがりひていししびれまたたきねむりをさまたげるはこうするてつのおうじょたえずじかいするどろのにんぎょうけつごうせよはんぱつせよちにみちおのれのむりょくをしれぃぇゔぃ"
		td.expectSuggestionsShows = false
	case jpAlkb:
		td.expectSuggestionsShows = false
	case jpKana:
		td.characters = strings.Split("あいうえおかきくけこさしすせそたちつてとなにぬねのはひふへほまみむめもやゆよらりるれろわん゛゜ー", "")
		td.expectedText = "あいうえおかきくけこさしすせそたちつてとなにぬねのはひふへほまみむめもやゆよらりるれろわん゛゜ー"
	case jpRoman:
		td.expectedText = "ｑうぇｒちゅいおぱｓｄｆｇｈｊｋｌｚｘｃｖｂんｍ"
	}

	return td
}

func numberTest(st inputMethod) *testDetail {
	td := &testDetail{
		chType:           number,
		characters:       strings.Split(allNumbers, ""),
		expectedText:     allNumbers,
		checkSuggestions: false,
	}

	switch st {
	case jpUskb, jpRoman:
		td.expectedText = "１２３４５６７８９０"
	case jpKana:
		td.expectedText = "" // There are no "Numbers" in Kana mode.
	}

	return td
}

func symbolsBasicTest(st inputMethod) *testDetail {
	td := &testDetail{
		chType:           symbol,
		characters:       strings.Split(basicSymbols, ""),
		expectedText:     basicSymbols,
		checkSuggestions: false,
	}

	switch st {
	case enUK:
		symbols := strings.ReplaceAll(basicSymbols, "$", "£")
		td.characters = strings.Split(symbols, "")
		td.expectedText = symbols
	case jpUskb:
		symbols := strings.ReplaceAll(basicSymbols, ",.", "、。")
		td.characters = strings.Split(symbols, "")
		td.expectedText = "＠＃＄％＆ー＋（）￥＝＊”’：；！？＿・、。"
	case jpKana:
		td.characters = []string{} // There are no "Symbols" in Kana mode.
		td.expectedText = ""       // There are no "Symbols" in Kana mode.
	case jpRoman:
		symbols := strings.ReplaceAll(basicSymbols, ",.", "、。")
		td.characters = strings.Split(symbols, "")
		td.expectedText = "＠＃＄％＆ー＋（）￥＝＊”’：；！？ー・、。"
	}

	return td
}

func symbolsExtraTest(st inputMethod) *testDetail {
	td := &testDetail{
		chType:           symbolExtra,
		characters:       strings.Split(extraSymbols, ""),
		expectedText:     extraSymbols,
		checkSuggestions: false,
	}

	switch st {
	case enUK:
		symbols := strings.ReplaceAll(extraSymbols, "£¢€¥", "€¥$¢")
		td.characters = strings.Split(symbols, "")
		td.expectedText = symbols
	case jpUskb:
		symbols := strings.ReplaceAll(extraSymbols, ",.", "、。")
		td.characters = strings.Split(symbols, "")
		td.expectedText = "〜｀｜•√π÷×¶Δ£¢€¥＾°＝｛｝￥©®™℅「」¡¿＜＞、。"
	case jpKana:
		td.characters = []string{} // There are no "Extra Symbols" in Kana mode.
		td.expectedText = ""       // There are no "Extra Symbols" in Kana mode.
	case jpRoman:
		symbols := strings.ReplaceAll(extraSymbols, ",.", "、。")
		td.characters = strings.Split(symbols, "")
		td.expectedText = "〜｀｜•√π÷×¶Δ£¢€￥＾°＝｛｝￥©®™℅「」¡¿＜＞、。"
	}

	return td
}
