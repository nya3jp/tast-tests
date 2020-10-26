// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"

	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
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
		Attr:         []string{"group:mainline", "group:essential-inputs"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:              "es_stable",
				ExtraHardwareDeps: pre.InputsStableModels,
				Val: testParameters{
					regionCode:             "es",
					defaultInputMethodID:   "xkb:es::spa",
					defaultInputMethodName: "abrir menú de teclado", // label displayed as ES
				},
			}, {
				Name:              "es_unstable",
				ExtraHardwareDeps: pre.InputsUnstableModels,
				ExtraAttr:         []string{"informational"},
				Val: testParameters{
					regionCode:             "es",
					defaultInputMethodID:   "xkb:es::spa",
					defaultInputMethodName: "abrir menú de teclado", // label displayed as ES
				},
			}, {
				Name:              "fr_stable",
				ExtraHardwareDeps: pre.InputsStableModels,
				Val: testParameters{
					regionCode:             "fr",
					defaultInputMethodID:   "xkb:fr::fra",
					defaultInputMethodName: "ouvrir le menu du clavier", // label displayed as FR
				},
			}, {
				Name:              "fr_unstable",
				ExtraHardwareDeps: pre.InputsUnstableModels,
				ExtraAttr:         []string{"informational"},
				Val: testParameters{
					regionCode:             "fr",
					defaultInputMethodID:   "xkb:fr::fra",
					defaultInputMethodName: "ouvrir le menu du clavier", // label displayed as FR
				},
			}, {
				Name:              "jp_stable",
				ExtraHardwareDeps: pre.InputsStableModels,
				Val: testParameters{
					regionCode:             "jp",
					defaultInputMethodID:   "xkb:jp::jpn",
					defaultInputMethodName: "キーボード メニューを開く", // label displayed as JA
				},
			}, {
				Name:              "jp_unstable",
				ExtraHardwareDeps: pre.InputsUnstableModels,
				ExtraAttr:         []string{"informational"},
				Val: testParameters{
					regionCode:             "jp",
					defaultInputMethodID:   "xkb:jp::jpn",
					defaultInputMethodName: "キーボード メニューを開く", //label displayed as JA
				},
			},
		},
	})
}

func VirtualKeyboardSystemLanguages(ctx context.Context, s *testing.State) {
	regionCode := s.Param().(testParameters).regionCode
	defaultInputMethodID := s.Param().(testParameters).defaultInputMethodID
	defaultInputMethodName := s.Param().(testParameters).defaultInputMethodName

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
	currentInputMethodID, err := vkb.GetCurrentInputMethod(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get current input method: ", err)
	}

	if currentInputMethodID != defaultInputMethodID {
		s.Fatalf("Failed to verify default input method in country %s. got %s; want %s", regionCode, currentInputMethodID, defaultInputMethodID)
	}

	if err := vkb.ShowVirtualKeyboard(ctx, tconn); err != nil {
		s.Fatal("Failed to show the virtual keyboard: ", err)
	}

	s.Log("Waiting for the virtual keyboard to show")
	if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to show: ", err)
	}

	languageMenuNode, err := vkb.DescendantNode(ctx, tconn, ui.FindParams{ClassName: "sk label-key language-menu"})
	if err != nil {
		s.Fatal("Failed to find language menu node: ", err)
	}
	defer languageMenuNode.Release(ctx)

	if languageMenuNode.Name != defaultInputMethodName {
		s.Errorf("unepxected language menu name: got %q; want %q", languageMenuNode.Name, defaultInputMethodName)
	}
}
