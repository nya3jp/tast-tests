// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Keyboard,
		Desc:         "Demonstrates injecting keyboard events",
		Contacts:     []string{"ricardoq@chromium.org", "tast-owners@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func Keyboard(ctx context.Context, s *testing.State) {
	const (
		html        = "<!DOCTYPE html><input id='text' type='text' autofocus>"
		elementExpr = "document.getElementById('text')"
		valueExpr   = elementExpr + ".value"
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		io.WriteString(w, html)
	}))
	defer server.Close()

	s.Log("Loading input page")
	cr := s.PreValue().(*chrome.Chrome)
	conn, err := cr.NewConn(ctx, server.URL)
	if err != nil {
		s.Fatal("Creating renderer failed: ", err)
	}
	defer conn.Close()

	// waitForStringExpr waits for expr to evaluate to expected with short timeout.
	waitForStringExpr := func(expr, expected string) error {
		s.Log("Waiting for text from ", expr)
		return testing.Poll(ctx, func(ctx context.Context) error {
			var s string
			if err := conn.Eval(ctx, expr, &s); err != nil {
				return err
			}
			if s != expected {
				return errors.Errorf("%s = %q; want %q", expr, s, expected)
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second})
	}

	s.Log("Waiting for focus")
	if err := conn.WaitForExpr(ctx, elementExpr+" === document.activeElement"); err != nil {
		s.Fatal("Failed waiting for focus: ", err)
	}

	s.Log("Finding and opening keyboard device")
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer ew.Close()

	const inputText = "Hello, world!"
	s.Logf("Injecting keyboard events for %q", inputText)
	if err = ew.Type(ctx, inputText); err != nil {
		s.Fatal("Failed to write events: ", err)
	}
	if err := waitForStringExpr(valueExpr, inputText); err != nil {
		// Stop test here for getting screenshot of this wrong state.
		// TODO(yamaguchi): Change back to Error() once the flakiness is fixed.
		s.Fatal("Failed to get input text (this can be flaky): ", err)
	}

	const (
		pageText = "mittens"
		dataURL  = "data:text/plain," + pageText
		bodyExpr = "document.body.innerText"
	)
	s.Logf("Navigating to %q via omnibox", dataURL)
	if err := ew.Accel(ctx, "Ctrl+L"); err != nil {
		s.Fatal("Failed to write events: ", err)
	}
	if err := ew.Type(ctx, dataURL+"\n"); err != nil {
		s.Fatal("Failed to write events: ", err)
	}
	if err := waitForStringExpr(bodyExpr, pageText); err != nil {
		s.Error("Failed to get page text: ", err)
	}

	// Not all Chromebooks have the same layout for the function keys.
	layout, err := input.KeyboardTopRowLayout(ctx, ew)
	if err != nil {
		s.Fatal("Failed to get keyboard mapping: ", err)
	}

	key := layout.ZoomToggle
	// If the key is empty it means it is not mapped
	if key != "" {
		if err := ew.Accel(ctx, key); err != nil {
			s.Fatal("Failed to write events: ", err)
		}
	}
}
