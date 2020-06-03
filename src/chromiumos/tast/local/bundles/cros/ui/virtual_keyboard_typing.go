// Copyright 2018 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTyping,
		Desc:         "Checks that the virtual keyboard works in Chrome",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Pre:          chrome.VKEnabled(),
		Timeout:      5 * time.Minute,
	})
}

func VirtualKeyboardTyping(ctx context.Context, s *testing.State) {
	// typingKeys indicates a key series that tapped on virtual keyboard.
	var typingKeys = []string{"h", "e", "l", "l", "o", "space", "t", "a", "s", "t"}

	const expectedTypingResult = "hello tast"

	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Virtual keyboard is mostly used in tablet mode.
	s.Log("Setting device to tablet mode")
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure tablet mode enabled: ", err)
	}
	defer cleanup(ctx)

	// Create a touch controller.
	// Use pc tap event to trigger virtual keyboard instead of calling vkb.ShowVirtualKeyboard()
	pc, err := pointer.NewTouchController(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create a touch controller")
	}
	defer pc.Close()

	s.Log("Start a local server to test chrome")
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

	element, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: identifier}, 5*time.Second)
	if err != nil {
		s.Fatalf("Failed to find input element %s: %v", identifier, err)
	}

	s.Log("Click input field to trigger virtual keyboard shown up")
	if err := pointer.Click(ctx, pc, element.Location.CenterPoint()); err != nil {
		s.Fatal("Failed to click the input element: ", err)
	}

	s.Log("Input with virtual keyboard")
	if err := vkb.InputWithVirtualKeyboard(ctx, tconn, typingKeys); err != nil {
		s.Fatal("Failed to type on virtual keyboard: ", err)
	}

	inputValueElement, err := element.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeStaticText}, time.Second)
	if err != nil {
		s.Fatal("Failed to find searchbox value element: ", err)
	}

	if inputValueElement.Name != expectedTypingResult {
		s.Fatalf("Failed to input with virtual keyboard. Got: %s; Want: %s", inputValueElement.Name, expectedTypingResult)
	}
}
