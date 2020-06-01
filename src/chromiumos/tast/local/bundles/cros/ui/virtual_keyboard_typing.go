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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/ui/faillog"
	"chromiumos/tast/local/bundles/cros/ui/pointer"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTyping,
		Desc:         "Checks that the virtual keyboard can type into a text field",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Pre:          chrome.VKEnabled(),
		Timeout:      5 * time.Minute,
	})
}

// typingKeys indicates a key series that tapped on virtual keyboard.
var typingKeys = []string{"h", "e", "l", "l", "o", "space", "t", "a", "s", "t"}

const expectedTypingResult = "hello tast"

func VirtualKeyboardTyping(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s, tconn)

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
		s.Error("Failed to create a touch controller")
	}
	defer pc.Close()

	kconn, err := vkb.UIConn(ctx, cr)
	if err != nil {
		s.Fatal("creating connection to virtual keyboard UI failed: ", err)
	}
	defer kconn.Close()

	testCases := []string{
		"Chrome",
		"Settings",
	}

	for _, testCase := range testCases {
		switch testCase {
		case "Chrome":
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
				s.Error("Creating renderer for test page failed: ", err)
				break
			}
			defer conn.Close()

			element, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: identifier}, 5*time.Second)
			if err != nil {
				s.Errorf("Failed to find input element %s: %v", identifier, err)
				break
			}

			s.Log("Click input field to trigger virtual keyboard shown up")
			if err := pointer.Click(ctx, pc, element.Location.CenterPoint()); err != nil {
				s.Error("Failed to click the input element: ", err)
				break
			}

			s.Log("Input with virtual keyboard")
			if err := inputWithVirtualKeyboard(ctx, tconn, kconn, typingKeys); err != nil {
				s.Error("Failed to type on virtual keyboard: ", err)
				break
			}

			if err := assertInputValue(ctx, element, expectedTypingResult); err != nil {
				s.Error("Failed to assert input result: ", err)
				break
			}
		case "Settings":
			app := apps.Settings
			s.Logf("Launching %s", app.Name)
			if err := apps.Launch(ctx, tconn, app.ID); err != nil {
				s.Errorf("Failed to launch %s: %s", app.Name, err)
				break
			}
			if err := ash.WaitForApp(ctx, tconn, app.ID); err != nil {
				s.Errorf("%s did not appear in shelf after launch: %v", app.Name, err)
				break
			}

			s.Log("Find searchbox input element")
			params := ui.FindParams{
				Role: ui.RoleTypeSearchBox,
				Name: "Search settings",
			}
			element, err := ui.FindWithTimeout(ctx, tconn, params, 3*time.Second)
			if err != nil {
				s.Error("Failed to find searchbox input field in settings: ", err)
				break
			}

			s.Log("Click searchbox to trigger virtual keyboard")
			if err := pointer.Click(ctx, pc, element.Location.CenterPoint()); err != nil {
				s.Error("Failed to click the input element: ", err)
				break
			}

			s.Log("Input with virtual keyboard")
			if err := inputWithVirtualKeyboard(ctx, tconn, kconn, typingKeys); err != nil {
				s.Error("Failed to type on virtual keyboard: ", err)
				break
			}

			if err := assertInputValue(ctx, element, expectedTypingResult); err != nil {
				s.Error("Failed to assert input result: ", err)
				break
			}
		}
	}
}

func inputWithVirtualKeyboard(ctx context.Context, tconn *chrome.TestConn, kconn *chrome.Conn, keys []string) error {
	if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for the virtual keyboard to show")
	}

	if err := vkb.WaitUntilButtonsRender(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for the virtual keyboard to render")
	}

	if err := vkb.TapKeys(ctx, kconn, keys); err != nil {
		return errors.Wrapf(err, "failed to tap keys %v: %v", keys, err)
	}

	if err := vkb.HideVirtualKeyboard(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to hide the virtual keyboard")
	}
	return nil
}

// assertInputValue provely works for Chrome and Settings search input. Not guaranteed for other input elements.
func assertInputValue(ctx context.Context, element *ui.Node, expectedValue string) error {
	inputValueElement, err := element.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeStaticText}, time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find searchbox value element")
	}

	if inputValueElement.Name != expectedValue {
		return errors.Errorf("failed to input with virtual keyboard. Got: %s; Want: %s", inputValueElement.Name, expectedValue)
	}
	return nil
}
