// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"time"

	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HotkeySearchPlusA,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test Search + A assistant hotkey to toggle launcher",
		Contacts:     []string{"yawano@google.com", "assistive-eng@google.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Pre:          chrome.LoggedIn(),
		Timeout:      3 * time.Minute,
	})
}

func HotkeySearchPlusA(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	if err := assistant.Enable(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}
	defer func() {
		if err := assistant.Cleanup(ctx, s.HasError, cr, tconn); err != nil {
			s.Fatal("Failed to disable assistant: ", err)
		}
	}()

	if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
		s.Fatal("Failed to confirm that launcher is closed before a test: ", err)
	}

	if err := assistant.ToggleUIWithHotkey(ctx, tconn, assistant.AccelSearchPlusA); err != nil {
		s.Fatal("Failed to toggle assistant UI with hotkey: ", err)
	}

	if err := ash.WaitForLauncherState(ctx, tconn, ash.Half); err != nil {
		s.Fatal("Failed to confirm that launcher got opened with assistant hotkey: ", err)
	}

	if err := assistant.ToggleUIWithHotkey(ctx, tconn, assistant.AccelSearchPlusA); err != nil {
		s.Fatal("Failed to toggle assistant UI with hotkey: ", err)
	}

	if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
		s.Fatal("Failed to confirm that launcher got closed with assistant hotkey: ", err)
	}
}
