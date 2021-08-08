// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"

	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// testParameters contains all the data needed to run a single test iteration.
type testParameters struct {
	regionCode             string
	defaultInputMethodID   string
	defaultInputMethodName string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: VirtualKeyboardSystemLanguages,
		Desc: "Launching ChromeOS in different languages defaults input method",
		Contacts: []string{
			"essential-inputs-team@google.com",
		},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:              "es",
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream"},
				Val: testParameters{
					regionCode:           "es",
					defaultInputMethodID: string(ime.INPUTMETHOD_XKB_ES_SPA),
				},
			}, {
				Name:              "es_informational",
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				ExtraAttr:         []string{"informational"},
				Val: testParameters{
					regionCode:           "es",
					defaultInputMethodID: string(ime.INPUTMETHOD_XKB_ES_SPA),
				},
			}, {
				Name:              "fr",
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream"},
				Val: testParameters{
					regionCode:           "fr",
					defaultInputMethodID: string(ime.INPUTMETHOD_XKB_FR_FRA),
				},
			}, {
				Name:              "fr_informational",
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				ExtraAttr:         []string{"informational"},
				Val: testParameters{
					regionCode:           "fr",
					defaultInputMethodID: string(ime.INPUTMETHOD_XKB_FR_FRA),
				},
			}, {
				Name:              "jp",
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream"},
				Val: testParameters{
					regionCode:           "jp",
					defaultInputMethodID: string(ime.INPUTMETHOD_XKB_JP_JPN),
				},
			}, {
				Name:              "jp_informational",
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				ExtraAttr:         []string{"informational"},
				Val: testParameters{
					regionCode:           "jp",
					defaultInputMethodID: string(ime.INPUTMETHOD_XKB_JP_JPN),
				},
			},
		},
	})
}

func VirtualKeyboardSystemLanguages(ctx context.Context, s *testing.State) {
	regionCode := s.Param().(testParameters).regionCode
	defaultInputMethodID := ime.ChromeIMEPrefix + s.Param().(testParameters).defaultInputMethodID

	cr, err := chrome.New(ctx, chrome.Region(regionCode), chrome.VKEnabled())
	if err != nil {
		s.Fatalf("Failed to start Chrome in region %s: %v", regionCode, err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Verify default input method
	currentInputMethodID, err := ime.CurrentInputMethod(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get current input method: ", err)
	}

	if currentInputMethodID != defaultInputMethodID {
		s.Fatalf("Failed to verify default input method in country %s. got %s; want %s", regionCode, currentInputMethodID, defaultInputMethodID)
	}
}
