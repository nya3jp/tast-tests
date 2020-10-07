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
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardEnglishSettings,
		Desc:         "Checks that the input settings works in Chrome",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name:              "stable",
			ExtraHardwareDeps: pre.InputsStableModels,
		}, {
			Name:              "unstable",
			ExtraHardwareDeps: pre.InputsUnstableModels,
			ExtraAttr:         []string{"informational"},
		}},
	})
}

func VirtualKeyboardEnglishSettings(ctx context.Context, s *testing.State) {
	var typingKeys = []string{"space", "g", "o", "."}
	const expectedTypingResult = " Go. go. Go."

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
	defer element.Release(ctx)

	s.Log("Click searchbox to trigger virtual keyboard")
	if err := element.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click the input element: ", err)
	}

	s.Log("Wait for virtual keyboard shown up")
	if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for virtual keyboard shown up: ", err)
	}

	s.Log("Wait for decoder running")
	if err := vkb.WaitForDecoderEnabled(ctx, cr, true); err != nil {
		s.Fatal("Failed to wait for virtual keyboard shown up: ", err)
	}

	if err := vkb.TapKeys(ctx, tconn, typingKeys); err != nil {
		s.Fatal("Failed to input with virtual keyboard: ", err)
	}

	if err := tconn.Eval(ctx, `chrome.inputMethodPrivate.setSettings(
			"xkb:us::eng", 
			{"virtualKeyboardEnableCapitalization": false,
			"virtualKeyboardAutoCorrectionLevel": 1})`, nil); err != nil {
		s.Fatal("Failed to set settings: ", err)
	}

	// Hide and show VK twice to ensure the settings are updated.
	hideAndShowVirtualKeyboard(ctx, tconn, element, s)
	hideAndShowVirtualKeyboard(ctx, tconn, element, s)

	if err := vkb.TapKeys(ctx, tconn, typingKeys); err != nil {
		s.Fatal("Failed to input with virtual keyboard: ", err)
	}

	if err := tconn.Eval(ctx, `chrome.inputMethodPrivate.setSettings(
			"xkb:us::eng", 
			{"virtualKeyboardEnableCapitalization": true,
			"virtualKeyboardAutoCorrectionLevel": 1})`, nil); err != nil {
		s.Fatal("Failed to set settings: ", err)
	}

	// Hide and show VK twice to ensure the settings are updated.
	hideAndShowVirtualKeyboard(ctx, tconn, element, s)
	hideAndShowVirtualKeyboard(ctx, tconn, element, s)

	if err := vkb.TapKeys(ctx, tconn, typingKeys); err != nil {
		s.Fatal("Failed to input with virtual keyboard: ", err)
	}

	// Value change can be a bit delayed after input.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		inputValueElement, err := element.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeStaticText}, 2*time.Second)
		if err != nil {
			return err
		}
		if inputValueElement.Name != expectedTypingResult {
			return errors.Errorf("failed to input with virtual keyboard. Got: %s; Want: %s", inputValueElement.Name, expectedTypingResult)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Error("Failed to input with virtual keyboard: ", err)
	}
}

func hideAndShowVirtualKeyboard(ctx context.Context, tconn *chrome.TestConn, element *ui.Node, s *testing.State) {
	vkb.HideVirtualKeyboard(ctx, tconn)
	vkb.WaitUntilHidden(ctx, tconn)

	if err := element.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click the input element: ", err)
	}
	vkb.WaitUntilShown(ctx, tconn)
}
