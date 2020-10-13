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
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     VirtualKeyboardTypingIME,
		Desc:     "Enables manual test using IME decoder, the virtual keyboard works in Chrome; this test only for manual test on G3 VM",
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

func VirtualKeyboardTypingIME(ctx context.Context, s *testing.State) {
	const expectedTypingResult = "google"
	typingKeys := strings.Split(expectedTypingResult, "")

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

	if !ime {
		s.Log("Wait for decoder running")
		if err := vkb.WaitForDecoderEnabled(ctx, cr, true); err != nil {
			s.Fatal("Failed to wait for virtual keyboard shown up: ", err)
		}
	} else {
		s.Log("No need to wait for decoder running")
	}

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
			return errors.Errorf("failed to input with virtual keyboard (got: %s; want: %s)", inputValueElement.Name, expectedTypingResult)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Error("Failed to input with virtual keyboard: ", err)
	}
}
