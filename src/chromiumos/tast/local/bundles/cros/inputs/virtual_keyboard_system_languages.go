// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

// testParameters contains all the data needed to run a single test iteration.
type testParameters struct {
	regionCode              string
	defaultInputMethodID    string
	defaultInputMethodLabel string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: VirtualKeyboardSystemLanguages,
		Desc: "Launching ChromeOS in different languages defaults input method",
		Contacts: []string{
			"essential-inputs-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name: "es",
				Val: testParameters{
					regionCode:              "es",
					defaultInputMethodID:    "_comp_ime_jkghodnilhceideoidjikpgommlajknkxkb:es::spa",
					defaultInputMethodLabel: "ES",
				},
			}, {
				Name: "fr",
				Val: testParameters{
					regionCode:              "fr",
					defaultInputMethodID:    "_comp_ime_jkghodnilhceideoidjikpgommlajknkxkb:fr::fra",
					defaultInputMethodLabel: "FR",
				},
			}, {
				Name: "jp",
				Val: testParameters{
					regionCode:              "jp",
					defaultInputMethodID:    "_comp_ime_jkghodnilhceideoidjikpgommlajknkxkb:jp::jpn",
					defaultInputMethodLabel: "JA",
				},
			},
		},
	})
}

func VirtualKeyboardSystemLanguages(ctx context.Context, s *testing.State) {
	regionCode := s.Param().(testParameters).regionCode
	defaultInputMethodID := s.Param().(testParameters).defaultInputMethodID
	defaultInputMethodLabel := s.Param().(testParameters).defaultInputMethodLabel

	cr, err := chrome.New(ctx, chrome.Region(regionCode), chrome.ExtraArgs("--enable-virtual-keyboard"))
	if err != nil {
		s.Fatalf("Failed to start Chrome in region %s: %v", regionCode, err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer tconn.Close()

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

	s.Log("Waiting for the virtual keyboard to render buttons")
	if err := vkb.WaitUntilButtonsRender(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to render: ", err)
	}

	keyboard, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeKeyboard}, 3*time.Second)
	if err != nil {
		s.Fatal("Virtual keyboard does not show")
	}
	defer keyboard.Release(ctx)

	if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{Name: defaultInputMethodLabel}, 1*time.Second); err != nil {
		s.Fatalf("Failed to find %s in language menu on virtual keyboard: %v", defaultInputMethodLabel, err)
	}
}
