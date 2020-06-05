// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/pointer"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
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

	// Verify default input method
	currentInputMethodID, err := vkb.GetCurrentInputMethod(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get current input method: ", err)
	}

	if currentInputMethodID != defaultInputMethodID {
		s.Fatalf("Failed to verify default input method in country %s. got %s; want %s", regionCode, currentInputMethodID, defaultInputMethodID)
	}

	// Verify virtual keyboard layout
	const identifier = "e14s-inputbox"
	html := fmt.Sprintf(`<input type="text" id="text" autocorrect="off" aria-label=%q/>`, identifier)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		io.WriteString(w, html)
	}))
	defer server.Close()

	conn, err := cr.NewConn(ctx, server.URL)
	if err != nil {
		s.Fatal("Creating renderer for test page failed: ", err)
	}
	defer conn.Close()

	// Create a touch controller.
	// Use pc tap event to trigger virtual keyboard instead of calling vkb.ShowVirtualKeyboard()
	pc, err := pointer.NewTouchController(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create a touch controller")
	}
	defer pc.Close()

	element, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: identifier}, 5*time.Second)
	if err != nil {
		s.Fatalf("Failed to find input element %s: %v", identifier, err)
	}

	s.Log("Click input field to trigger virtual keyboard shown up")
	if err := pointer.Click(ctx, pc, element.Location.CenterPoint()); err != nil {
		s.Fatal("Failed to click the input element: ", err)
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
