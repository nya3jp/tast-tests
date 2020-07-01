// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardSuggestions,
		Desc:         "Checks that the virtual keyboard suggestions work for various languages",
		Contacts:     []string{"shend@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      3 * time.Minute,
	})
}

func VirtualKeyboardSuggestions(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-virtual-keyboard"), chrome.ExtraArgs("--force-tablet-mode=touch_view"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Show a page with a text field that autofocuses. Turn off autocorrect as it
	// can interfere with the test.
	const html = `<input type="text" id="target" autocorrect="off" autofocus aria-label="e14s-inputbox"/>`
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

	// Wait for the text field to focus.
	if err := conn.WaitForExpr(ctx,
		`document.getElementById('target') === document.activeElement`); err != nil {
		s.Fatal("Failed to wait for text field to focus: ", err)
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

	kconn, err := vkb.UIConn(ctx, cr)
	if err != nil {
		s.Fatal("Failed to create connection to virtual keyboard UI: ", err)
	}
	defer kconn.Close()

	// The input method ID is from:
	// src/chrome/browser/resources/chromeos/input_method/google_xkb_manifest.json
	// Each input method should only have one test case.
	testCases := []struct {
		InputMethod        string
		Keys               []string
		ExpectedSuggestion string
		LanguageLabel      string
	}{
		{"xkb:us::eng", []string{"a"}, "a", "US"},
		{"nacl_mozc_us", []string{"o"}, "お", "あ"},
	}

	for _, testCase := range testCases {
		s.Log("Testing ", testCase.InputMethod)

		if err := vkb.SetCurrentInputMethod(ctx, tconn, testCase.InputMethod); err != nil {
			s.Error("Failed to set input method: ", err)
			continue
		}

		s.Log("Clear text field before test")
		if err := conn.Exec(ctx,
			`document.getElementById('target').value='';`); err != nil {
			s.Error("Failed to clear text field: ", err)
			continue
		}

		params := ui.FindParams{
			Name: testCase.LanguageLabel,
		}
		if err := ui.WaitUntilExists(ctx, tconn, params, 3*time.Second); err != nil {
			s.Errorf("Failed to switch to language %s: %v", testCase.InputMethod, err)
			continue
		}

		// Need to hide and show virtual keyboard again to re-init suggestions after changing input method.
		if err := vkb.HideVirtualKeyboard(ctx, tconn); err != nil {
			s.Error("Failed to show the virtual keyboard: ", err)
			continue
		}

		if err := vkb.WaitUntilHidden(ctx, tconn); err != nil {
			s.Error("Failed to wait for the virtual keyboard to hide: ", err)
			continue
		}

		if err := vkb.ShowVirtualKeyboard(ctx, tconn); err != nil {
			s.Error("Failed to show the virtual keyboard: ", err)
			continue
		}

		s.Log("Waiting for the virtual keyboard to show")
		if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
			s.Error("Failed to wait for the virtual keyboard to show: ", err)
			continue
		}

		s.Log("Waiting for the virtual keyboard to render buttons")
		if err := vkb.WaitUntilButtonsRender(ctx, tconn); err != nil {
			s.Error("Failed to wait for the virtual keyboard to render: ", err)
			continue
		}

		if err := vkb.TapKeys(ctx, tconn, testCase.Keys); err != nil {
			s.Error("Failed to type: ", err)
			continue
		}

		s.Log("Waiting for the decoder to provide suggestions")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			suggestions, err := vkb.GetSuggestions(ctx, kconn)
			s.Log("Suggestions: ", suggestions)
			if err != nil {
				return err
			}
			for _, suggestion := range suggestions {
				if suggestion == testCase.ExpectedSuggestion {
					return nil
				}
			}
			return errors.Errorf("%q not found in suggestions", testCase.ExpectedSuggestion)
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			s.Error("Failed to wait for suggestions to appear: ", err)
			continue
		}
	}
}
