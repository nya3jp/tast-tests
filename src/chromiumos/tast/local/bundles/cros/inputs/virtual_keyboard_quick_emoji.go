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
		Func:         VirtualKeyboardQuickEmoji,
		Desc:         "Checks that right click input field and select emoji will trigger virtual keyboard",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Params: []testing.Param{{
			Name:              "stable",
			ExtraHardwareDeps: pre.InputsStableModels,
		}, {
			Name:              "unstable",
			ExtraHardwareDeps: pre.InputsUnstableModels,
		}}})
}

func VirtualKeyboardQuickEmoji(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
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

	inputElement, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: identifier}, 5*time.Second)
	if err != nil {
		s.Fatalf("Failed to find input element %s: %v", identifier, err)
	}
	defer inputElement.Release(ctx)

	if err := inputElement.RightClick(ctx); err != nil {
		s.Fatal("Failed to right click the input element: ", err)
	}

	emojiMenuElement, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: "Emoji"}, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to find Emoji menu item: ", err)
	}
	defer emojiMenuElement.Release(ctx)

	if err := emojiMenuElement.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click the input element: ", err)
	}

	if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for virtual keyboard shown: ", err)
	}

	if isEmojiPanelShown, err := ui.Exists(ctx, tconn, ui.FindParams{Name: "emoji keyboard shown"}); err != nil {
		s.Fatal("Failed to check emoji panel: ", err)
	} else if !isEmojiPanelShown {
		s.Fatal("Emoji vk container is not quick shown")
	}

	// Hide virtual keyboard and click input field again should not trigger vk.
	if err := vkb.HideVirtualKeyboard(ctx, tconn); err != nil {
		s.Fatal("Failed to hide virtual keyboard: ", err)
	}

	if err := vkb.WaitUntilHidden(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for virtual keyboard hidden: ", err)
	}

	if err := inputElement.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click the input element: ", err)
	}

	// Check virtual keyboard is not shown in the following 10 seconds
	testing.Poll(ctx, func(ctx context.Context) error {
		if isVKShown, err := vkb.IsShown(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to check vk visibility")
		} else if isVKShown {
			s.Fatal("Virtual keyboard is still enabled after quick emoji input")
		}
		return errors.New("continuously check until timeout")
	}, &testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: 10 * time.Second})
}
