// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/faillog"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardAccent,
		Desc:         "Checks that long pressing keys pop up accent window",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Pre:          chrome.VKEnabled(),
		Timeout:      3 * time.Minute,
	})
}

func VirtualKeyboardAccent(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s, tconn)

	testing.Sleep(ctx, 3*time.Second)

	// Virtual keyboard is mostly used in tablet mode.
	s.Log("Setting device to tablet mode")
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure tablet mode enabled: ", err)
	}
	defer cleanup(ctx)

	// Show a page with a text field that autofocuses. Turn off autocorrect as it
	// can interfere with the test.
	const html = `<input type="text" id="target" autocorrect="off" autofocus/>`
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
		`document.getElementById('target') === document.activeElement`); err != nil {
		s.Fatal("Failed to wait for text field to focus: ", err)
	}

	if err := vkb.ShowVirtualKeyboard(ctx, tconn); err != nil {
		s.Fatal("Failed to show the virtual keyboard: ", err)
	}

	s.Log("Waiting for the virtual keyboard to show")
	if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to show: ", err)
	}

	s.Log("Waiting for the virtual keyboard to render buttons")
	if err := vkb.WaitUntilButtonsRender(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to render: ", err)
	}

	kconn, err := vkb.UIConn(ctx, cr)
	if err != nil {
		s.Fatal("Failed to create connection to virtual keyboard UI: ", err)
	}
	defer kconn.Close()

	// The input method ID is from:
	// src/chrome/browser/resources/chromeos/input_method/google_xkb_manifest.json
	// Each input method should only have one test case.
	testCases := []struct {
		InputMethod   string
		Key           string
		AccentKey     string
		LanguageLabel string
	}{
		// Keep multi-test structure here for further languages scalablity.
		{"xkb:fr::fra", "e", "é", "FR"},
	}

	for _, testCase := range testCases {
		s.Log("Testing ", testCase.InputMethod)

		if err := vkb.SetCurrentInputMethod(ctx, tconn, testCase.InputMethod); err != nil {
			s.Error("Failed to set input method: ", err)
			continue
		}

		params := ui.FindParams{
			Name: testCase.LanguageLabel,
		}
		if err := ui.WaitUntilExists(ctx, tconn, params, 3*time.Second); err != nil {
			s.Errorf("Failed to switch to language %s: %v", testCase.InputMethod, err)
			continue
		}

		s.Log("Clear text field before test")
		if err := conn.Exec(ctx,
			`document.getElementById('target').value='';`); err != nil {
			s.Error("Failed to clear text field: ", err)
			continue
		}

		vk, err := vkb.VirtualKeyboard(ctx, tconn)
		if err != nil {
			s.Error("Failed to find virtual keyboad automation node: ", err)
			continue
		}
		defer vk.Release(ctx)

		keyParams := ui.FindParams{
			Role: ui.RoleTypeButton,
			Name: testCase.Key,
		}

		key, err := vk.Descendant(ctx, keyParams)
		if err != nil {
			s.Errorf("Failed to find key with %v: %v", keyParams, err)
			continue
		}
		defer key.Release(ctx)

		if err := mouse.Move(ctx, tconn, key.Location.CenterPoint(), 100*time.Millisecond); err != nil {
			s.Errorf("Failed to move mouse to key %s: %v", testCase.Key, err)
			continue
		}

		if err := mouse.Press(ctx, tconn, mouse.LeftButton); err != nil {
			s.Error("Failed to press key: ", err)
			continue
		}

		// Popup accent window sometimes flash on showing, so using polling instead of DescendantofTimeOut
		s.Log("Waiting for accent window pop up")
		var location coords.Point
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			accentContainer, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "goog-container goog-container-vertical accent-container"}, 1*time.Second)
			if err != nil {
				return errors.Wrap(err, "failed to find the container")
			}
			defer accentContainer.Release(ctx)

			accentKeyParams := ui.FindParams{Name: testCase.AccentKey}
			accentKey, err := accentContainer.Descendant(ctx, accentKeyParams)
			if err != nil {
				return errors.Wrapf(err, "fFailed to find accentkey with %v", accentKeyParams)
			}
			if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
				return errors.Wrap(err, "failed to wait for animation finished")
			}
			location = accentKey.Location.CenterPoint()
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			s.Error("Failed to wait for accent window: ", err)
			continue
		}

		if err := mouse.Move(ctx, tconn, location, 100*time.Millisecond); err != nil {
			s.Errorf("Failed to move mouse to key %s: %v", testCase.AccentKey, err)
			continue
		}

		if err := mouse.Release(ctx, tconn, mouse.LeftButton); err != nil {
			s.Error("Failed to release mouse click: ", err)
			continue
		}
	}
}
