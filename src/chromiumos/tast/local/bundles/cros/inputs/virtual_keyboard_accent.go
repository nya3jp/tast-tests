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
		Timeout:      5 * time.Minute,
	})
}

func VirtualKeyboardAccent(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-virtual-keyboard"), chrome.ExtraArgs("--force-tablet-mode=touch_view"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer tconn.Close()

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
	const (
		inputMethodID = "xkb:fr::fra"
		keyName       = "e"
		accentKeyName = "é"
		languageLabel = "FR"
	)

	if err := vkb.SetCurrentInputMethod(ctx, tconn, inputMethodID); err != nil {
		s.Fatal("Failed to set input method: ", err)
	}

	params := ui.FindParams{
		Name: languageLabel,
	}
	if err := ui.WaitUntilExists(ctx, tconn, params, 3*time.Second); err != nil {
		s.Fatalf("Failed to switch to language %s: %v", inputMethodID, err)
	}

	s.Log("Click and hold key for accent window")
	vk, err := vkb.VirtualKeyboard(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find virtual keyboad automation node: ", err)
	}
	defer vk.Release(ctx)

	keyParams := ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: keyName,
	}

	key, err := vk.Descendant(ctx, keyParams)
	if err != nil {
		s.Fatalf("Failed to find key with %v: %v", keyParams, err)
	}
	defer key.Release(ctx)

	if err := mouse.Move(ctx, tconn, key.Location.CenterPoint(), 500*time.Millisecond); err != nil {
		s.Fatalf("Failed to move mouse to key %s: %v", keyName, err)
	}

	if err := mouse.Press(ctx, tconn, mouse.LeftButton); err != nil {
		s.Fatal("Failed to press key: ", err)
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

		// Wait for pop up window fully positioned
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			containerLocation := accentContainer.Location
			testing.Sleep(ctx, time.Second)
			accentContainer.Update(ctx)
			if accentContainer.Location != containerLocation {
				return errors.New("popup window is not positioned")
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return err
		}

		accentKeyParams := ui.FindParams{Name: accentKeyName}
		accentKey, err := accentContainer.Descendant(ctx, accentKeyParams)
		if err != nil {
			return errors.Wrapf(err, "failed to find accentkey with %v", accentKeyParams)
		}
		defer accentKey.Release(ctx)

		if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to wait for animation finished")
		}
		accentKey.Update(ctx)
		location = accentKey.Location.CenterPoint()
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second}); err != nil {
		s.Fatal("Failed to wait for accent window: ", err)
	}

	if err := mouse.Move(ctx, tconn, location, 500*time.Millisecond); err != nil {
		s.Fatalf("Failed to move mouse to key %s: %v", accentKeyName, err)
	}

	if err := mouse.Release(ctx, tconn, mouse.LeftButton); err != nil {
		s.Fatal("Failed to release mouse click: ", err)
	}

	s.Log("Verify value in input field")
	// Value change can be a bit delayed after input.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		inputValueElement, err := element.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeStaticText}, time.Second)
		if err != nil {
			return err
		}
		defer inputValueElement.Release(ctx)
		if inputValueElement.Name != accentKeyName {
			return errors.Errorf("failed to input with virtual keyboard. Got: %s; Want: %s", inputValueElement.Name, accentKeyName)
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Second}); err != nil {
		s.Error("Failed to input with virtual keyboard: ", err)
	}
}
