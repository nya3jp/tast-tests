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
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTypingBrowser,
		Desc:         "Checks that the virtual keyboard works in Chrome browser",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Pre:          pre.VKEnabled(),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name:              "stable",
			ExtraHardwareDeps: pre.InputsStableModels,
		}, {
			Name:              "unstable",
			ExtraHardwareDeps: pre.InputsUnstableModels,
		}}})
}

func VirtualKeyboardTypingBrowser(ctx context.Context, s *testing.State) {
	// typingKeys indicates a key series that tapped on virtual keyboard.
	const typingKeys = "go"
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

	inputWithVK := func(inputParams ui.FindParams) error {
		inputNode, err := ui.FindWithTimeout(ctx, tconn, inputParams, 10*time.Second)
		if err != nil {
			return errors.Wrapf(err, "failed to find input node with params %v", inputParams)
		}
		defer inputNode.Release(ctx)

		if err := inputNode.LeftClick(ctx); err != nil {
			return errors.Wrap(err, "failed to click the input node")
		}

		s.Log("Waiting for the virtual keyboard to show")
		if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to wait for the virtual keyboard to show")
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
			return errors.Wrap(err, "failed to input with virtual keyboard")
		}
		return nil
	}

	s.Run(ctx, "Omnibox", func(ctx context.Context, s *testing.State) {
		if err := inputWithVK(ui.FindParams{
			Role:       ui.RoleTypeTextField,
			Attributes: map[string]interface{}{"inputType": "url"},
		}); err != nil {
			s.Error("Failed to use virtual keybaord in omnibox: ", err)
		}
	})

	s.Run(ctx, "InputField", func(ctx context.Context, s *testing.State) {
		if err := inputWithVK(ui.FindParams{Name: identifier}); err != nil {
			s.Error("Failed to use virtual keybaord in input field: ", err)
		}
	})
}
