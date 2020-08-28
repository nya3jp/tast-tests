// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
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
		Func:         VirtualKeyboardDeadKeys,
		Desc:         "Checks that dead keys on the virtual keyboard work",
		Contacts:     []string{"tranbaoduy@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      5 * time.Minute,
	})
}

func VirtualKeyboardDeadKeys(ctx context.Context, s *testing.State) {
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

	s.Log("Start a local server for the test page")
	const identifier = "e14s-inputbox"
	html := fmt.Sprintf(`<input type="text" id="text" autocorrect="off" autocapitalize="off" aria-label=%q/>`, identifier)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		io.WriteString(w, html)
	}))
	defer server.Close()

	conn, err := cr.NewConn(ctx, "")
	if err != nil {
		s.Fatal("Creating renderer failed: ", err)
	}
	defer conn.Close()

	type DeadKeysTestCase struct {
		inputMethodID        string
		hasDecoder           bool
		useA11yVk            bool
		typingKeys           []string
		expectedTypingResult string
	}

	TestDeadKeys := func(testCase DeadKeysTestCase) {
		if err := conn.Navigate(ctx, server.URL); err != nil {
			s.Fatal("Failed to navigate to test page at ["+server.URL+"]: ", err)
		}

		s.Log("Set input method to: " + testCase.inputMethodID)
		if err := vkb.SetCurrentInputMethod(ctx, tconn, testCase.inputMethodID); err != nil {
			s.Fatal("Failed to set input method to ["+testCase.inputMethodID+"]: ", err)
		}

		if testCase.useA11yVk {
			s.Log("Enabling the accessibility keyboard")
		} else {
			s.Log("Disabling the accessibility keyboard")
		}
		if err := tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.setWhitelistedPref)`, "settings.a11y.virtual_keyboard", testCase.useA11yVk); err != nil {
			s.Fatal("Failed to enable the accessibility keyboard: ", err)
		}

		element, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: identifier}, 5*time.Second)
		if err != nil {
			s.Fatalf("Failed to find input element %s: %v", identifier, err)
		}
		defer element.Release(ctx)

		s.Log("Click input element to trigger virtual keyboard")
		if err := element.LeftClick(ctx); err != nil {
			s.Fatal("Failed to click the input element: ", err)
		}

		s.Log("Wait for virtual keyboard shown up")
		if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
			s.Fatal("Failed to wait for virtual keyboard shown up: ", err)
		}

		if testCase.hasDecoder {
			s.Log("Wait for decoder running")
			if err := vkb.WaitForDecoderEnabled(ctx, cr, true); err != nil {
				s.Fatal("Failed to wait for decoder running: ", err)
			}
		}

		if err := vkb.TapKeys(ctx, tconn, testCase.typingKeys); err != nil {
			s.Fatal("Failed to input with virtual keyboard: ", err)
		}

		// Value change can be a bit delayed after input.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			inputValueElement, err := element.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeStaticText}, 2*time.Second)
			if err != nil {
				return err
			}
			if inputValueElement.Name != testCase.expectedTypingResult {
				return errors.Errorf("failed to input with virtual keyboard. Got: %s; Want: %s", inputValueElement.Name, testCase.expectedTypingResult)
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Error("Failed to input with virtual keyboard: ", err)
		}

		vkb.HideVirtualKeyboard(ctx, tconn)
	}

	const AcuteAccent = "\u0301"
	const Circumflex = "\u0302"

	TestDeadKeys(DeadKeysTestCase{
		// "French - French keyboard" input method is backed by a decoder. Dead keys
		// are implemented differently from those of a no-frills input method.
		inputMethodID: "xkb:fr::fra",
		hasDecoder:    true,

		// "French - French keyboard" input method uses a "compact layout" VK for
		// non-a11y mode where there's no dead keys. It uses a "full layout" VK for
		// a11y mode where there's dead keys. To test dead keys on the VK of this
		// input method, a11y mode must be enabled.
		useA11yVk: true,

		// TODO(b/162292283): Make vkb.TapKeys() less flaky when the VK changes
		// based on Shift and Caps states, then add Shift and Caps related typing
		// sequences to the test case.
		typingKeys:           []string{Circumflex, "a"},
		expectedTypingResult: "â",
	})

	TestDeadKeys(DeadKeysTestCase{
		// "Catalan keyboard" input method is no-frills. Dead keys are implemented
		// differently from those of a decoder-backed input method.
		inputMethodID: "xkb:es:cat:cat",
		hasDecoder:    false,

		// "Catalan keyboard" input method uses the same "full layout" VK (that has
		// dead keys) for both a11y and non-a11y modes. Just use non-a11y here.
		useA11yVk: false,

		// TODO(b/162292283): Make vkb.TapKeys() less flaky when the VK changes
		// based on Shift and Caps states, then add Shift and Caps related typing
		// sequences to the test case.
		typingKeys:           []string{AcuteAccent, "a"},
		expectedTypingResult: "á",
	})
}
