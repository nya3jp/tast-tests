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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardEnglishSettings,
		Desc:         "Checks that the input settings works in Chrome",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:essential-inputs", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Pre:          pre.VKEnabled(),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name:              "stable",
			ExtraHardwareDeps: pre.InputsStableModels,
		}, {
			Name:              "unstable",
			ExtraHardwareDeps: pre.InputsUnstableModels,
		}},
	})
}

func VirtualKeyboardEnglishSettings(ctx context.Context, s *testing.State) {
	var typingKeys = []string{"space", "g", "o", "."}
	const expectedTypingResult = " Go. go."

	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(ctx)

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

	hideAndShowVirtualKeyboard := func() {
		if err := vkb.HideVirtualKeyboard(ctx, tconn); err != nil {
			s.Fatal("Failed to hide virtual keyboard: ", err)
		}

		if err := vkb.WaitUntilHidden(ctx, tconn); err != nil {
			s.Fatal("Failed to wait for vk hidden: ", err)
		}

		s.Log("Click input to trigger virtual keyboard")
		if err := vkb.ClickUntilVKShown(ctx, tconn, element); err != nil {
			s.Fatal("Failed to click the input and wait for vk shown: ", err)
		}
	}

	s.Log("Wait for decoder running")
	if err := vkb.WaitForDecoderEnabled(ctx, cr, true); err != nil {
		s.Fatal("Failed to wait for virtual keyboard shown up: ", err)
	}

	// TODO(jiwan): Change settings via Chrome OS settings page after we migrate settings to there.
	if err := tconn.Eval(ctx, `chrome.inputMethodPrivate.setSettings(
		"xkb:us::eng",
		{"virtualKeyboardEnableCapitalization": true,
		"virtualKeyboardAutoCorrectionLevel": 1})`, nil); err != nil {
		s.Fatal("Failed to set settings: ", err)
	}

	s.Log("Click input to trigger virtual keyboard")
	if err := vkb.ClickUntilVKShown(ctx, tconn, element); err != nil {
		s.Fatal("Failed to click the input and wait for vk shown: ", err)
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

	hideAndShowVirtualKeyboard()

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
			return errors.Errorf("failed to input with virtual keyboard; got: %s, want: %s", inputValueElement.Name, expectedTypingResult)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Error("Failed to input with virtual keyboard: ", err)
	}
}
