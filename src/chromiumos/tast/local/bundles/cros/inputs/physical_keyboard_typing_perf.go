// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type typingPerfTestParam struct {
	inputMethod ime.InputMethod
	keys        string
}

// TODO(b/241994527): Use better input data that better reflect real world usage.

// From 'Alice in Wonderland'
var enUSTestData = "Alice was beginning to get very tired of sitting by her sister on the bank, and of having nothing to do: once or twice she had peeped into the book her sister was reading, but it had no pictures or conversations in it, 'and what is the use of a book,' thought Alice 'without pictures or conversations?'"

// From go/3828217 with random concatenation.
var pinyinTestData = "ke yi zuo ziji xiang zuodeshi chuan lai de ziji de shengyin henyou yisi de zhe yang xian ba zai zhe bian hai yu dao le zhe ming de zhu chiren li chen zhe ci lai shi weile xin chang pian de shi buguo da jia buyong dan xinwo hui jin kuaihao qi laide dan mei ci doumei you jihui quyou lan yi xiazai yan chu qian wojie shou le zhuan fang you de shi hou sihu zong shiganjueshi jian guodetai kuai lebu guohai shixiwang zai mingnian nengyouxin de zhuan jidai gei dajia xinqing zaileng qi fangli kaishi chen dian jian qiyi gebeikefangzai er bian tingshuyuda hai deshengyin rang womenyiqi zai weilai de lv cheng zhong bi ci doubuyao fangqi zhun que di shuoying gai shi bian han lengle dang ran zhe doushi zai bu kun de qian ti xia wan chengde"

// From go/3921098 with random concatenation.
var japaneseTestData = "wagahai wa nekodearu. na ma e wa mada nai. doko de uma reta ka tonto kentou ga tsukanu. nani demo usugurai jimejime shita tokoro de nya nya naite ita koto dake haki oku shite iru. nani demo usugurai jimejime shita tokoro de nya nya naite ita koto dake haki oku shite iru. wagahai wa koko de hajimete nin gen to iu mono o mita. shikamo ato de kiku to soreha sho seitoiu ni n gen chi yuudeichibandouakunashuzokudeattasouda. kono sho sei to iu no wa tokidoki wareware o toraete nite kuu to iu hanashidearu. shikashi sono touji wa nani to iu kou mo nakattakara betsudan kowashi ito mo omowanakatta. tada kare notenohirani nose rarete suu to mochiage rareta toki nandaka fuwafuwa shita kanji ga atta bakaridearu."

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardTypingPerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks the physical keyboard typing performance",
		Contacts:     []string{"shend@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		HardwareDeps: hwdep.D(pre.PhysicalKeyboardPerfModels),
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS, ime.ChinesePinyin}),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name:    "en_us",
				Fixture: fixture.ClamshellNonVKRestart,
				Val: typingPerfTestParam{
					inputMethod: ime.EnglishUS,
					keys:        enUSTestData,
				},
			},
			{
				Name:    "en_us_lacros",
				Fixture: fixture.LacrosClamshellNonVKRestart,
				Val: typingPerfTestParam{
					inputMethod: ime.EnglishUS,
					keys:        enUSTestData,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name:    "pinyin",
				Fixture: fixture.ClamshellNonVKRestart,
				Val: typingPerfTestParam{
					inputMethod: ime.ChinesePinyin,
					keys:        pinyinTestData,
				},
			},
			{
				Name:    "pinyin_lacros",
				Fixture: fixture.LacrosClamshellNonVKRestart,
				Val: typingPerfTestParam{
					inputMethod: ime.ChinesePinyin,
					keys:        pinyinTestData,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name:    "ja",
				Fixture: fixture.ClamshellNonVKRestart,
				Val: typingPerfTestParam{
					inputMethod: ime.Japanese,
					keys:        japaneseTestData,
				},
			},
		},
	})
}

func PhysicalKeyboardTypingPerf(ctx context.Context, s *testing.State) {
	inputMethod := s.Param().(typingPerfTestParam).inputMethod
	testKeys := s.Param().(typingPerfTestParam).keys
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

	latencyHistogram, err := metrics.GetHistogram(ctx, tconn, "InputMethod.KeyEventLatency")
	if err != nil {
		s.Fatal("Failed to get histograms: ", err)
	}

	pv := perf.NewValues()

	// Record the mean.
	mean, err := latencyHistogram.Mean()
	if err != nil {
		s.Fatal("Failed to get mean: ", err)
	}
	pv.Set(perf.Metric{
		Name:      "key_latency_mean",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, mean)

	// Record percentage of 'fast' key events. We define 'fast' as < 5ms.
	percentFast, err := util.PercentSamplesBelow(latencyHistogram, 5)
	if err != nil {
		s.Fatal("Failed to get percentage of fast key events: ", err)
	}
	pv.Set(perf.Metric{
		Name:      "key_latency_fast",
		Unit:      "percent",
		Direction: perf.BiggerIsBetter,
	}, percentFast)

	// Record percentage of 'acceptable' key events. We define 'acceptable' as < 15ms.
	percentAcceptable, err := util.PercentSamplesBelow(latencyHistogram, 15)
	if err != nil {
		s.Fatal("Failed to get percentage of acceptable key events: ", err)
	}
	pv.Set(perf.Metric{
		Name:      "key_latency_acceptable",
		Unit:      "percent",
		Direction: perf.BiggerIsBetter,
	}, percentAcceptable)

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
