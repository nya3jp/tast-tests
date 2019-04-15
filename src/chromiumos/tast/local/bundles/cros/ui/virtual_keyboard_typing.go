// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/local/bundles/cros/ui/vkb"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTyping,
		Desc:         "Checks that the virtual keyboard can type into a text field",
		Contacts:     []string{"essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome_login", "google_virtual_keyboard"},
	})
}

func VirtualKeyboardTyping(ctx context.Context, s *testing.State) {
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
	const html = `<input type="text" id="text" autocorrect="off" autofocus/>`
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

	// Press a sequence of keys.
	// TODO(https://crbug.com/934650): Test pressing "backspace" key as well
	// without causing flaky failures.
	keys := []string{
		"h", "e", "l", "l", "o", "space", "t", "a", "s", "t"}

	for _, key := range keys {
		if err := vkb.TapKey(ctx, kconn, key); err != nil {
			s.Fatalf("Failed to tap %q: %v", key, err)
		}
	}

	s.Log("Waiting for the text field to have the correct contents")
	if err := conn.WaitForExpr(ctx,
		`document.getElementById('text').value === 'hello tast'`); err != nil {
		s.Fatal("Failed to get the contents of the text field: ", err)
	}
}
