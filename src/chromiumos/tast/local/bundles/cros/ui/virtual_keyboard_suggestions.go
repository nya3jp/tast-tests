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
	"chromiumos/tast/local/bundles/cros/ui/vkb"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     VirtualKeyboardSuggestions,
		Desc:     "Checks that the virtual keyboard displays suggestions",
		Contacts: []string{"essential-inputs-team@google.com"},
		Attr:     []string{"informational"},
		// "cros_internal" is needed to use the official proprietary virtual keyboard.
		SoftwareDeps: []string{"chrome_login", "cros_internal"},
	})
}

func VirtualKeyboardSuggestions(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-virtual-keyboard"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// TODO(https://crbug.com/930775): Test languages officially supported by
	// Chrome OS by iterating through them and testing each decoder one by one.
	const xkbExtensionID = "_comp_ime_jkghodnilhceideoidjikpgommlajknk"
	const inputMethodIDEnUS = xkbExtensionID + "xkb:us::eng"
	if err := vkb.SetCurrentInputMethod(ctx, tconn, inputMethodIDEnUS); err != nil {
		s.Fatal("Failed to set the input method: ", err)
	}

	// Show a page with a text field that autofocuses.
	const html = `<input type="text" id="text" autofocus/>`
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
		`document.getElementById('text') === document.activeElement`); err != nil {
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
		s.Fatal("Creating connection to virtual keyboard UI failed: ", err)
	}
	defer kconn.Close()

	// The IME decoder, which provides the suggestions for the virtual keyboard,
	// ignores key presses until it is fully loaded. Thus, this test presses keys
	// periodically until the decoder is ready and suggestions are shown.
	s.Log("Waiting for the decoder to provide suggestions")
	err = testing.Poll(ctx, func(ctx context.Context) error {
		if err := vkb.TapKey(ctx, kconn, "a"); err != nil {
			return err
		}
		suggestions, err := vkb.GetSuggestions(ctx, kconn)
		if err != nil {
			return err
		}
		if len(suggestions) == 0 {
			return errors.New("no suggestions found")
		}
		return nil
	}, nil)

	if err != nil {
		s.Fatal("Failed to wait for suggestions to appear: ", err)
	}
}
