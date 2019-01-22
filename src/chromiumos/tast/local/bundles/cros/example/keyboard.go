// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE

package example

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Keyboard,
		Desc:         "Demonstrates injecting keyboard events",
		Contacts:     []string{"derat@chromium.org", "tast-users@chromium.org"},
		Attr:         []string{"informational"},
		Pre:          chrome.LoggedIn(),
		SoftwareDeps: []string{"chrome_login"},
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

	// getText waits for expr to evaluate to a string of the given length and returns the string.
	getText := func(expr string, length int) (string, error) {
		s.Log("Waiting for text from ", expr)
		if err := conn.WaitForExpr(ctx, fmt.Sprintf("%s.length === %d", expr, length)); err != nil {
			return "", errors.Wrapf(err, "waiting for %s failed", expr)
		}
		var actual string
		if err := conn.Eval(ctx, expr, &actual); err != nil {
			return "", errors.Wrapf(err, "evaluating %s failed", expr)
		}
		s.Logf("Got text %q from %s", actual, expr)
		return actual, nil
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
	// TODO(derat): The text typed above seems to sometimes not show up; try to figure out why.
	// Maybe there's a small delay within Blink between document.activeElement being updated and keyboard
	// events actually being directed to the element.
	if actual, err := getText(valueExpr, len(inputText)); err != nil {
		s.Error("Failed to get input text (this can be flaky): ", err)
	} else if actual != inputText {
		s.Errorf("Got input text %q; typed %q (non-QWERTY layout or Caps Lock?)", actual, inputText)
	}

	const (
		pageText = "mittens"
		dataURL  = "data:text/plain," + pageText
		bodyExpr = "document.body.innerText"
	)
	s.Logf("Navigating to %q via omnibox", dataURL)
	ew.Accel(ctx, "Ctrl+L")
	ew.Type(ctx, dataURL+"\n")
	if actual, err := getText(bodyExpr, len(pageText)); err != nil {
		s.Error("Failed to get page text: ", err)
	} else if actual != pageText {
		s.Errorf("Got page text %q; want %q", actual, pageText)
	}
}
