// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardSuggestionsInformational,
		Desc:         "Checks that the virtual keyboard suggestions work for various languages (for informational tests that are not stable yet)",
		Contacts:     []string{"shend@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
	})
}

func VirtualKeyboardSuggestionsInformational(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-virtual-keyboard"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Show a page with a text field that autofocuses. Turn off autocorrect as it
	// can interfere with the test.
	const html = `<input type="text" id="target" autocorrect="off" autofocus/>`
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
	}{
		{"nacl_mozc_jp", []string{"n", "i", "h", "o", "n", "g", "o"}, "日本語"},
		{"nacl_mozc_us", []string{"n", "i", "h", "o", "n", "g", "o"}, "日本語"},
	}

	const xkbExtensionID = "_comp_ime_jkghodnilhceideoidjikpgommlajknk"

	for _, testCase := range testCases {
		s.Log("Testing ", testCase.InputMethod)

		if err := vkb.SetCurrentInputMethod(ctx, tconn, xkbExtensionID+testCase.InputMethod); err != nil {
			s.Error("Failed to set input method: ", err)
			continue
		}

		if err := vkb.TapKeys(ctx, kconn, testCase.Keys); err != nil {
			s.Error("Failed to type: ", err)
			continue
		}

		s.Log("Waiting for the decoder to provide suggestions")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			suggestions, err := vkb.GetSuggestions(ctx, kconn)
			if err != nil {
				return err
			}
			for _, suggestion := range suggestions {
				if suggestion == testCase.ExpectedSuggestion {
					return nil
				}
			}
			return errors.Errorf("%q not found in suggestions", testCase.ExpectedSuggestion)
		}, nil); err != nil {
			s.Error("Failed to wait for suggestions to appear: ", err)
			continue
		}

		// Tap enter to exit composition mode.
		if err := vkb.TapKey(ctx, kconn, "enter"); err != nil {
			s.Error("Failed to tap enter: ", err)
			continue
		}
	}
}
