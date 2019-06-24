// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
// Copied from

package ui

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	// Library for raw input
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"


)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RulebasedArabic,
		Desc:         "Checks that the PK can type into a text field",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", "Arabic IME"},
	})
}

func RulebasedArabic(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-virtual-keyboard"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Set the input method to arabic
	const m17nExtensionID = "_comp_ime_jkghodnilhceideoidjikpgommlajknk"
	const arabicInputMethod = m17nExtensionID + "vkd_ar"
	if err := ime.SetCurrentInputMethod(ctx, tconn, arabicInputMethod); err != nil {
		s.Fatal("Failed to set the input method: ", err)
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

	// TODO() The ime.SetCurrentInputMethod takes some time to finish setting the
	// IME correctly
	time.Sleep(time.Second * 1)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// Press a sequence of keys.
	// without causing flaky failures.
	const keystrokes = "google"
	if err := kb.Type(ctx, keystrokes); err != nil {
		s.Fatalf("Failed to type %q: %v", keystrokes, err)
	}

	// In order to use GetText() after timeout, we should have shorter timeout than ctx.
	s.Log("Waiting for the text field to have the correct contents")
	if err := conn.WaitForExpr(ctx,
		`document.getElementById('text').value === 'لخخلمث'`); err != nil {
		s.Fatal("Failed to get the contents of the text field: ", err)
	}

}
