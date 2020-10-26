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
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     VirtualKeyboardTypingBrowserIME,
		Desc:     "Checks that the virtual keyboard works in Chrome browser",
		Attr:     []string{"group:essential-inputs", "group:rapid-ime-decoder"},
		Contacts: []string{"essential-inputs-team@google.com"},
		// this test is not to be promoted as it is only intended for dev use
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		HardwareDeps: hwdep.D(hwdep.Model("betty")),
		Timeout:      5 * time.Minute,
		Vars: []string{
			"inputs.useIME", // if present will load IME decoder, default is NaCl
		}})
}

func VirtualKeyboardTypingBrowserIME(ctx context.Context, s *testing.State) {
	// typingKeys indicates a key series that tapped on virtual keyboard.
	const typingKeys = "go"

	extraArgs := []string{"--enable-virtual-keyboard", "--force-tablet-mode=touch_view"}

	_, ime := s.Var("inputs.useIME")

	if ime {
		extraArgs = append(extraArgs,
			"--enable-features=ImeInputLogicFst,EnableImeSandbox")
		s.Log("Appended IME params")
	}
	cr, err := chrome.New(ctx, chrome.ExtraArgs(extraArgs...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(ctx)

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
		s.Fatal("Failed to connect to test page: ", err)
	}
	defer conn.Close()

	inputWithVK := func(inputNode *ui.Node) error {
		if err := vkb.ClickUntilVKShown(ctx, tconn, inputNode); err != nil {
			return errors.Wrap(err, "failed to click the input node and wait for vk shown")
		}

		s.Log("Waiting for the virtual keyboard to render buttons")
		if err := vkb.WaitUntilButtonsRender(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to wait for the virtual keyboard to render")
		}

		if err := vkb.TapKeys(ctx, tconn, strings.Split(typingKeys, "")); err != nil {
			return errors.Wrap(err, "failed to input with virtual keyboard")
		}

		if err := vkb.HideVirtualKeyboard(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to hide virtual keyboard")
		}
		return nil
	}

	s.Run(ctx, "Omnibox", func(ctx context.Context, s *testing.State) {
		omniboxInputParams := ui.FindParams{
			Role:       ui.RoleTypeTextField,
			Attributes: map[string]interface{}{"inputType": "url"},
		}
		inputNode, err := ui.FindWithTimeout(ctx, tconn, omniboxInputParams, 10*time.Second)
		if err != nil {
			s.Fatalf("Failed to find Omnibox input with params %v: %v", omniboxInputParams, err)
		}
		defer inputNode.Release(ctx)
		if err := inputWithVK(inputNode); err != nil {
			s.Fatal("Failed to use virtual keyboard in omnibox: ", err)
		}

		// Value change can be a bit delayed after input.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := inputNode.Update(ctx); err != nil {
				return errors.Wrap(err, "failed to update node")
			}

			// When clicking Omnibox, on some devices existing text is highlighted and replaced by new input,
			// while on some other devices, it is not highlighted and inserted new input.
			// So use contains here to avoid flakiness.
			if !strings.Contains(inputNode.Value, typingKeys) {
				return errors.Errorf("failed to input with virtual keyboard. Got: %s; Want: %s", inputNode.Value, typingKeys)
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Error("Failed to input with virtual keyboard in Omnibox: ", err)
		}
	})

	s.Run(ctx, "InputField", func(ctx context.Context, s *testing.State) {
		inputNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: identifier}, 10*time.Second)
		if err != nil {
			s.Fatalf("Failed to find input node with name %s: %v", identifier, err)
		}
		defer inputNode.Release(ctx)
		if err := inputWithVK(inputNode); err != nil {
			s.Fatal("Failed to use virtual keyboard in input field: ", err)
		}

		// Value change can be a bit delayed after input.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := inputNode.Update(ctx); err != nil {
				return errors.Wrap(err, "failed to update node")
			}
			if inputNode.Value != typingKeys {
				return errors.Errorf("failed to input with virtual keyboard. Got: %s; Want: %s", inputNode.Value, typingKeys)
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Error("Failed to input with virtual keyboard in input field: ", err)
		}
	})
}
