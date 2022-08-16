// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type performanceTestParam struct {
	inputMethod ime.InputMethod
	keys        string
}

// TODO(b/241994527): Use better input data that better reflect real world usage.

// From 'Alice in Wonderland'
var enUSTestData = "Alice was beginning to get very tired of sitting by her sister on the bank, and of having nothing to do: once or twice she had peeped into the book her sister was reading, but it had no pictures or conversations in it, 'and what is the use of a book,' thought Alice 'without pictures or conversations?'"

// From
// http://google3/googledata/i18n/input/engine/eval/zh_t_i0_pinyin/sampled/2012-long-input.txt;l=1;rcl=48224509
// with random concatenation.
var pinyinTestData = "ke yi zuo ziji xiang zuodeshi chuan lai de ziji de shengyin henyou yisi de zhe yang xian ba zai zhe bian hai yu dao le zhe ming de zhu chiren li chen zhe ci lai shi weile xin chang pian de shi buguo da jia buyong dan xinwo hui jin kuaihao qi laide dan mei ci doumei you jihui quyou lan yi xiazai yan chu qian wojie shou le zhuan fang"

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardPerformance,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks the physical keyboard typing performance",
		Contacts:     []string{"shend@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational", "group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS, ime.ChinesePinyin}),
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name:    "en_us",
				Fixture: fixture.ClamshellNonVK,
				Val: performanceTestParam{
					inputMethod: ime.EnglishUS,
					keys:        enUSTestData,
				},
				ExtraAttr: []string{"group:input-tools-upstream"},
			},
			{
				Name:    "en_us_lacros",
				Fixture: fixture.LacrosClamshellNonVK,
				Val: performanceTestParam{
					inputMethod: ime.EnglishUS,
					keys:        enUSTestData,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name:    "pinyin",
				Fixture: fixture.ClamshellNonVK,
				Val: performanceTestParam{
					inputMethod: ime.ChinesePinyin,
					keys:        pinyinTestData,
				},
				ExtraAttr: []string{"group:input-tools-upstream"},
			},
			{
				Name:    "pinyin_lacros",
				Fixture: fixture.LacrosClamshellNonVK,
				Val: performanceTestParam{
					inputMethod: ime.ChinesePinyin,
					keys:        pinyinTestData,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
		},
	})
}

func PhysicalKeyboardPerformance(ctx context.Context, s *testing.State) {
	inputMethod := s.Param().(performanceTestParam).inputMethod
	testKeys := s.Param().(performanceTestParam).keys
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Add IME for testing.
	if err := inputMethod.InstallAndActivateUserAction(uc)(ctx); err != nil {
		s.Fatal("Failed to switch to Korean IME")
	}
	uc.SetAttribute(useractions.AttributeInputMethod, inputMethod.Name)

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

	var inputField = testserver.TextAreaInputField

	r := perfutil.NewRunner(cr.Browser())
	r.Runs = 3
	r.RunTracing = false

	r.RunMultiple(ctx, s, "", perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		if err := uiauto.UserAction(
			"PK input",
			uiauto.Combine("clear input field and type",
				its.ClearThenClickFieldAndWaitForActive(inputField),
				keyboard.TypeAction(testKeys),
			),
			uc, &useractions.UserActionCfg{
				Attributes: map[string]string{
					useractions.AttributeTestScenario: "type long text",
					useractions.AttributeFeature:      useractions.FeaturePKTyping,
					useractions.AttributeInputField:   string(inputField),
				},
			},
		)(ctx); err != nil {
			s.Fatalf("Failed to type in %s: %v", inputField, err)
		}
		return nil
	},
		"InputMethod.KeyEventLatency",
	), perfutil.StoreLatency)
}
